package mcp

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/service"
	"ai-chat-sql/utility"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/mark3labs/mcp-go/mcp"
)

// ExecSql 执行SQL
func (s *sMcpTool) ExecSql(ctx context.Context, request mcp.CallToolRequest) (out *mcp.CallToolResult, err error) {
	databaseId := request.GetInt("databaseId", 0)
	if databaseId == 0 {
		err = errors.New("databaseId is required")
		return
	}
	sql := request.GetString("sql", "")
	if sql == "" {
		err = errors.New("sql is required")
		return
	}
	db, err := service.Config().GetDataBase(ctx, databaseId)
	if err != nil {
		return
	}

	// 强制只读检查：任何 SQL 必须通过 isReadOnlySQL 校验
	if !isReadOnlySQL(sql) {
		errMsg := "安全策略：仅允许 SELECT / WITH / SHOW / DESCRIBE / EXPLAIN 查询，禁止执行写操作"
		consts.Logger.Warning(ctx, errMsg)
		out = mcp.NewToolResultText(errMsg)
		err = nil
		return
	}

	// 30s 查询超时（防止无界查询拖垮服务，LLM 超时后会自动修正 SQL）
	queryCtx, queryCancel := context.WithTimeout(ctx, 30*time.Second)
	defer queryCancel()

	queryStart := time.Now()
	sqlOut, queryErr := db.Query(queryCtx, sql)
	queryMs := time.Since(queryStart).Milliseconds()

	if queryCtx.Err() == context.DeadlineExceeded {
		errMsg := "查询执行超时（30秒），请尝试简化查询或添加更严格的时间范围条件"
		consts.Logger.Warning(ctx, errMsg)
		out = mcp.NewToolResultText(errMsg)
		err = nil
		return
	}

	if queryErr != nil {
		consts.Logger.Errorf(ctx, "数据库执行失败: %v", queryErr)
		consts.Logger.Infof(ctx, "sql_audit sql=%s durationMs=%d rowCount=0 error=%v", sql, queryMs, queryErr)
		out = mcp.NewToolResultText(utility.SafeUserErr(queryErr))
		err = nil
		return
	}

	rows := sqlOut.List()
	rowCount := len(rows)

	respStr, convErr := utility.ConvertAnyToMarkdownTable(rows)
	if convErr != nil {
		// 空结果是正常业务场景，不作为错误抛出
		respStr = "暂无数据"
	}

	// 审计日志
	consts.Logger.Infof(ctx, "sql_audit sql=%s durationMs=%d rowCount=%d", sql, queryMs, rowCount)

	// 记录查询表名到context（供SSE推送数据来源）
	for _, t := range extractTableNames(sql) {
		addTableToContext(ctx, t)
	}

	// 在返回结果中包含执行的 SQL 语句信息
	fullResult := fmt.Sprintf("**执行的 SQL：**\n\n```sql\n%s\n```\n\n**执行结果：**\n\n%s", sql, respStr)
	out = mcp.NewToolResultText(fullResult)
	return
}

// GetDatabaseInfo 获取数据库信息
func (s *sMcpTool) GetDatabaseInfo(ctx context.Context, request mcp.CallToolRequest) (out *mcp.CallToolResult, err error) {
	// 兜底防止 g.DB 在未配置 default 数据源时直接 panic，避免整个服务挂掉
	defer func() {
		if r := recover(); r != nil {
			consts.Logger.Errorf(ctx, "GetDatabaseInfo panic: %+v", r)
			// 友好的错误提示返回给前端 / AI
			out = mcp.NewToolResultText("数据库配置错误：未找到默认数据库配置，请检查服务端 config 中的数据库设置")
			err = nil
		}
	}()

	// Prefer databaseId because ChatDB stores business databases in
	// database_conf. Keep dbname for legacy GoFrame group-name calls.
	databaseId := request.GetInt("databaseId", 0)
	dbname := request.GetString("dbname", "")
	if databaseId == 0 && strings.TrimSpace(dbname) != "" {
		if id, convErr := strconv.Atoi(strings.TrimSpace(dbname)); convErr == nil {
			databaseId = id
			dbname = ""
		}
	}

	var db gdb.DB
	if databaseId > 0 {
		db, err = service.Config().GetDataBase(ctx, databaseId)
	} else {
		db, err = getConfiguredGroupDB(dbname)
	}
	if err != nil {
		consts.Logger.Errorf(ctx, "GetDatabaseInfo 配置错误: %v", err)
		out = mcp.NewToolResultText(utility.SafeUserErr(err))
		err = nil
		return
	}
	if db == nil {
		out = mcp.NewToolResultText("数据库连接不存在，请检查数据库配置")
		err = nil
		return
	}

	dbConfig := db.GetConfig()
	if dbConfig == nil {
		out = mcp.NewToolResultText("无法获取数据库配置")
		err = nil
		return
	}

	// 构建数据库信息
	dbInfo := g.Map{
		"databaseId":   databaseId,
		"databaseType": dbConfig.Type,
		"host":         extractHostFromLink(dbConfig.Link),
		"port":         extractPortFromLink(dbConfig.Link),
		"databaseName": extractDatabaseNameFromLink(dbConfig.Link),
		"username":     extractUsernameFromLink(dbConfig.Link),
		"prefix":       dbConfig.Prefix,
		"debug":        dbConfig.Debug,
	}
	databaseName := extractDatabaseNameFromLink(dbConfig.Link)

	// 测试连接并获取数据库版本信息
	versionQuery := getVersionQuery(dbConfig.Type)
	if versionQuery != "" {
		sqlOut, queryErr := db.Query(ctx, versionQuery)
		if queryErr == nil && sqlOut != nil && len(sqlOut.List()) > 0 {
			versionInfo := sqlOut.List()[0]
			dbInfo["version"] = versionInfo
		}
	}

	// 获取数据库大小（如果支持）
	sizeSQL, sizeArgs := getSizeQuery(dbConfig.Type, databaseName)
	if sizeSQL != "" {
		sqlOut, queryErr := db.Query(ctx, sizeSQL, sizeArgs...)
		if queryErr == nil && sqlOut != nil && len(sqlOut.List()) > 0 {
			sizeInfo := sqlOut.List()[0]
			dbInfo["databaseSize"] = sizeInfo
		}
	}

	out = mcp.NewToolResultText(gjson.MustEncodeString(dbInfo))
	return
}

func getConfiguredGroupDB(dbname string) (db gdb.DB, err error) {
	defer func() {
		if r := recover(); r != nil {
			db = nil
			err = fmt.Errorf("%v", r)
		}
	}()
	return g.DB(dbname), nil
}

// 从连接字符串中提取主机地址
func extractHostFromLink(link string) string {
	// 简单的解析，实际项目中可能需要更复杂的解析
	// 格式通常是: user:pass@tcp(host:port)/dbname?params
	if link == "" {
		return ""
	}

	// 查找 tcp( 和 ) 之间的内容
	start := strings.Index(link, "tcp(")
	if start == -1 {
		return ""
	}
	start += 4 // 跳过 "tcp("

	end := strings.Index(link[start:], ")")
	if end == -1 {
		return ""
	}

	hostPort := link[start : start+end]
	// 分离主机和端口
	parts := strings.Split(hostPort, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return hostPort
}

// 从连接字符串中提取端口
func extractPortFromLink(link string) string {
	if link == "" {
		return ""
	}

	start := strings.Index(link, "tcp(")
	if start == -1 {
		return ""
	}
	start += 4

	end := strings.Index(link[start:], ")")
	if end == -1 {
		return ""
	}

	hostPort := link[start : start+end]
	parts := strings.Split(hostPort, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// 从连接字符串中提取数据库名称
func extractDatabaseNameFromLink(link string) string {
	if link == "" {
		return ""
	}

	// 查找最后一个 / 之后的内容
	lastSlash := strings.LastIndex(link, "/")
	if lastSlash == -1 {
		return ""
	}

	dbPart := link[lastSlash+1:]
	// 移除查询参数
	questionMark := strings.Index(dbPart, "?")
	if questionMark != -1 {
		dbPart = dbPart[:questionMark]
	}

	return dbPart
}

// 从连接字符串中提取用户名
func extractUsernameFromLink(link string) string {
	if link == "" {
		return ""
	}

	// 查找 @ 之前的内容
	atIndex := strings.Index(link, "@")
	if atIndex == -1 {
		return ""
	}

	userPass := link[:atIndex]
	// 分离用户名和密码
	colonIndex := strings.Index(userPass, ":")
	if colonIndex != -1 {
		return userPass[:colonIndex]
	}
	return userPass
}

// 根据数据库类型获取版本查询语句
func getVersionQuery(dbType string) string {
	switch dbType {
	case "mysql":
		return "SELECT VERSION() as version"
	case "postgresql", "postgres":
		return "SELECT version() as version"
	case "mssql", "sqlserver":
		return "SELECT @@VERSION as version"
	default:
		return ""
	}
}

// getSizeQuery 返回数据库大小查询的 SQL 和参数，使用参数化查询防止注入
func getSizeQuery(dbType, dbname string) (sql string, args []interface{}) {
	switch dbType {
	case "mysql":
		return "SELECT ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) AS `Size (MB)` FROM information_schema.tables WHERE table_schema = ?", []interface{}{dbname}
	case "postgresql", "postgres":
		return "SELECT pg_size_pretty(pg_database_size($1)) as size", []interface{}{dbname}
	default:
		return "", nil
	}
}

// isReadOnlySQL 检查SQL语句是否为只读操作
func isReadOnlySQL(sql string) bool {
	// 去除前后空格并转换为大写
	sql = strings.TrimSpace(strings.ToUpper(sql))
	sql = strings.TrimSuffix(sql, ";")
	sql = strings.Join(strings.Fields(sql), " ")
	if sql == "" {
		return false
	}
	if strings.Contains(sql, ";") {
		return false
	}
	dangerousKeywords := []string{
		" INSERT ", " UPDATE ", " DELETE ", " DROP ", " ALTER ", " CREATE ",
		" TRUNCATE ", " REPLACE ", " GRANT ", " REVOKE ", " EXEC ", " CALL ",
		" MERGE ", " LOCK ", " UNLOCK ", " RENAME ", " OUTFILE", " DUMPFILE",
		" INFORMATION_SCHEMA", " LOAD_FILE", " BENCHMARK", " SLEEP",
	}
	paddedSQL := " " + sql + " "
	for _, keyword := range dangerousKeywords {
		if strings.Contains(paddedSQL, keyword) {
			return false
		}
	}

	// 单独检查 INTO OUTFILE / INTO DUMPFILE（可能跨单词匹配）
	if strings.Contains(sql, "INTO OUTFILE") || strings.Contains(sql, "INTO DUMPFILE") {
		return false
	}

	// 检查是否以SELECT开头（包括WITH语句，因为WITH通常用于查询）
	if strings.HasPrefix(sql, "SELECT") || strings.HasPrefix(sql, "WITH") {
		return true
	}

	// 检查是否为SHOW语句（MySQL的SHOW语句是只读的）
	if strings.HasPrefix(sql, "SHOW") {
		return true
	}

	// 检查是否为DESCRIBE或DESC语句
	if strings.HasPrefix(sql, "DESCRIBE") || strings.HasPrefix(sql, "DESC") {
		return true
	}

	// 检查是否为EXPLAIN语句
	if strings.HasPrefix(sql, "EXPLAIN") {
		return true
	}

	// 其他情况都认为是非只读操作
	return false
}

// ExtractTableNames 从MCP工具返回文本中提取表名（公开给AI侧使用）
func ExtractTableNames(text string) []string {
	return extractTableNames(text)
}

// AddTable 向累加器添加表名（去重）
func AddTable(tables *[]string, table string) {
	if tables == nil {
		return
	}
	for _, t := range *tables {
		if t == table {
			return
		}
	}
	*tables = append(*tables, table)
}

type ctxKeyTables struct{}

// sessionTables 用 sessionID 做 key，避免 eino 内部 context 派生丢值
var sessionTables sync.Map // map[string]*[]string

func WithTablesAccumulator(ctx context.Context) (context.Context, *[]string) {
	sid := sessionIdFromCtx(ctx)
	tables := make([]string, 0)
	ptr := &tables
	sessionTables.Store(sid, ptr)
	return context.WithValue(ctx, ctxKeyTables{}, sid), ptr
}

func TablesFromContext(ctx context.Context) *[]string {
	if v, ok := ctx.Value(ctxKeyTables{}).(string); ok {
		if ptr, ok := sessionTables.Load(v); ok {
			return ptr.(*[]string)
		}
	}
	return nil
}

func addTableToContext(ctx context.Context, table string) {
	consts.Logger.Infof(ctx, "perf addTable table=%s sidFromCtx=%v", table, ctx.Value("sessionId"))
	if v := TablesFromContext(ctx); v != nil {
		for _, t := range *v {
			if t == table {
				return
			}
		}
		*v = append(*v, table)
		consts.Logger.Infof(ctx, "perf addTable added table=%s total=%d", table, len(*v))
	} else {
		consts.Logger.Infof(ctx, "perf addTable FAILED no accumulator for table=%s", table)
	}
}

func sessionIdFromCtx(ctx context.Context) string {
	if v := ctx.Value("sessionId"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "unknown"
}

// extractTableNames 从SQL语句中提取表名
func extractTableNames(sql string) []string {
	upper := strings.ToUpper(sql)
	tables := make(map[string]bool)

	// FROM table
	fromRe := regexp.MustCompile(`(?i)\bFROM\s+` + "`" + `?(\w+)` + "`" + `?`)
	for _, m := range fromRe.FindAllStringSubmatch(sql, -1) {
		name := m[1]
		if !isSQLKeyword(strings.ToUpper(name)) {
			tables[name] = true
		}
	}

	// JOIN table
	joinRe := regexp.MustCompile(`(?i)\bJOIN\s+` + "`" + `?(\w+)` + "`" + `?`)
	for _, m := range joinRe.FindAllStringSubmatch(sql, -1) {
		name := m[1]
		if !isSQLKeyword(strings.ToUpper(name)) {
			tables[name] = true
		}
	}

	// 如果正则没匹配到，尝试简单解析
	if len(tables) == 0 && strings.Contains(upper, "FROM") {
		parts := strings.Fields(upper)
		for i, p := range parts {
			if p == "FROM" && i+1 < len(parts) {
				raw := strings.Trim(strings.Fields(sql)[i+1], "`;,")
				if !isSQLKeyword(strings.ToUpper(raw)) {
					tables[raw] = true
				}
			}
		}
	}

	result := make([]string, 0, len(tables))
	for t := range tables {
		result = append(result, t)
	}
	return result
}

func isSQLKeyword(s string) bool {
	keywords := map[string]bool{
		"SELECT": true, "WHERE": true, "AND": true, "OR": true,
		"GROUP": true, "ORDER": true, "HAVING": true, "LIMIT": true,
		"JOIN": true, "ON": true, "SET": true, "INTO": true,
		"VALUES": true, "AS": true, "NOT": true, "NULL": true,
		"LEFT": true, "RIGHT": true, "INNER": true, "OUTER": true,
		"CROSS": true, "UNION": true, "INSERT": true, "UPDATE": true,
		"DELETE": true, "CREATE": true, "ALTER": true, "DROP": true,
		"INDEX": true, "TABLE": true, "FROM": true, "BETWEEN": true,
		"IN": true, "LIKE": true, "EXISTS": true, "CASE": true,
		"WHEN": true, "THEN": true, "ELSE": true, "END": true,
		"DUAL": true,
	}
	return keywords[s]
}
