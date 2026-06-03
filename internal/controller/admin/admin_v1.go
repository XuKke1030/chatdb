package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	v1 "ai-chat-sql/api/admin/v1"

	"github.com/extrame/xls"
	"github.com/gogf/gf/v2/crypto/gmd5"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/xuri/excelize/v2"
)

var (
	adminEnsureMu    sync.Mutex
	adminTablesReady bool
)

func (c *ControllerV1) AdminLogin(ctx context.Context, req *v1.AdminLoginReq) (res *v1.AdminLoginRes, err error) {
	if req.Username == "" || req.Password == "" {
		return nil, gerror.New("绠＄悊鍛樿处鍙峰拰瀵嗙爜涓嶈兘涓虹┖")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	record, err := g.DB("master").Model("admin_account").Ctx(ctx).Where("username = ? AND enabled = ?", req.Username, 1).One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, gerror.New("绠＄悊鍛樿处鍙锋垨瀵嗙爜閿欒")
	}
	verify := record["verify"].Int()
	password, err := genAdminPassword(req.Password, verify)
	if err != nil {
		return nil, err
	}
	if record["password"].String() != password {
		return nil, gerror.New("绠＄悊鍛樿处鍙锋垨瀵嗙爜閿欒")
	}
	_ = insertAdminLog(ctx, "admin", req.Username, "管理员登录", "管理员登录后台管理平台", "success")
	return &v1.AdminLoginRes{
		Token:    "admin-dev-token",
		Username: req.Username,
	}, nil
}

func (c *ControllerV1) AdminProfile(ctx context.Context, req *v1.AdminProfileReq) (res *v1.AdminProfileRes, err error) {
	return &v1.AdminProfileRes{Username: "admin", Role: "administrator"}, nil
}

func (c *ControllerV1) AdminUsers(ctx context.Context, req *v1.AdminUsersReq) (res *v1.AdminUsersRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	users, err := listAdminUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUsersRes{List: users}, nil
}

func (c *ControllerV1) AdminKnowledgeBases(ctx context.Context, req *v1.AdminKnowledgeBasesReq) (res *v1.AdminKnowledgeBasesRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	return &v1.AdminKnowledgeBasesRes{List: knowledgeBaseOptions(ctx)}, nil
}

func (c *ControllerV1) AdminUpdateUserPermissions(ctx context.Context, req *v1.AdminUpdateUserPermissionsReq) (res *v1.AdminUpdateUserPermissionsRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("鐢ㄦ埛ID涓嶈兘涓虹┖")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	ruleLevel := req.RuleLevel
	if ruleLevel == 0 && len(req.Permissions) > 0 {
		ruleLevel = ruleLevelFromPermissions(req.Permissions)
	}
	now := int(gtime.Timestamp())
	_, _ = g.DB("master").Model("user").Ctx(ctx).Where("user_id = ?", req.Id).Data(g.Map{
		"rule_level":  ruleLevel,
		"update_time": now,
	}).Update()
	if err = upsertUserProfile(ctx, req.Id, g.Map{"rule_level": ruleLevel, "update_time": now}); err != nil {
		return nil, err
	}
	if err = replaceUserKnowledgePermissions(ctx, req.Id, req.QaPermissions); err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "权限更新", fmt.Sprintf("更新用户ID %d 的问数/问答权限", req.Id), "success")
	user, err := getAdminUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUpdateUserPermissionsRes{User: user}, nil
}

func (c *ControllerV1) AdminUpdateUserStatus(ctx context.Context, req *v1.AdminUpdateUserStatusReq) (res *v1.AdminUpdateUserStatusRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("鐢ㄦ埛ID涓嶈兘涓虹┖")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	if err = upsertUserProfile(ctx, req.Id, g.Map{"enabled": boolToInt(req.Enabled), "update_time": int(gtime.Timestamp())}); err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "账号状态", fmt.Sprintf("更新用户ID %d 状态为 %v", req.Id, req.Enabled), "success")
	user, err := getAdminUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUpdateUserStatusRes{User: user}, nil
}

func (c *ControllerV1) AdminExampleQuestions(ctx context.Context, req *v1.AdminExampleQuestionsReq) (res *v1.AdminExampleQuestionsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	query := g.DB("master").Model("admin_example_question").Ctx(ctx)
	if req.Topic != "" {
		query = query.Where("topic", req.Topic)
	}
	records, err := query.OrderAsc("sort").OrderDesc("update_time").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminExampleQuestionsRes{List: scanExampleQuestions(records)}, nil
}

func (c *ControllerV1) AdminCreateExampleQuestion(ctx context.Context, req *v1.AdminCreateExampleQuestionReq) (res *v1.AdminCreateExampleQuestionRes, err error) {
	if req.Topic == "" || req.Question == "" {
		return nil, gerror.New("topic and question are required")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	now := int(gtime.Timestamp())
	id, err := g.DB("master").Model("admin_example_question").Ctx(ctx).Data(g.Map{
		"topic":       req.Topic,
		"question":    req.Question,
		"description": req.Description,
		"enabled":     boolToInt(req.Enabled),
		"sort":        req.Sort,
		"create_time": now,
		"update_time": now,
	}).InsertAndGetId()
	if err != nil {
		return nil, err
	}
	item, err := getExampleQuestion(ctx, int(id))
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "示例问题", fmt.Sprintf("新增示例问题：%s", req.Question), "success")
	return &v1.AdminCreateExampleQuestionRes{Item: item}, nil
}

func (c *ControllerV1) AdminUpdateExampleQuestion(ctx context.Context, req *v1.AdminUpdateExampleQuestionReq) (res *v1.AdminUpdateExampleQuestionRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("闂ID涓嶈兘涓虹┖")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	_, err = g.DB("master").Model("admin_example_question").Ctx(ctx).Where("id = ?", req.Id).Data(g.Map{
		"topic":       req.Topic,
		"question":    req.Question,
		"description": req.Description,
		"enabled":     boolToInt(req.Enabled),
		"sort":        req.Sort,
		"update_time": int(gtime.Timestamp()),
	}).Update()
	if err != nil {
		return nil, err
	}
	item, err := getExampleQuestion(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "示例问题", fmt.Sprintf("更新示例问题ID %d", req.Id), "success")
	return &v1.AdminUpdateExampleQuestionRes{Item: item}, nil
}

func (c *ControllerV1) AdminDeleteExampleQuestion(ctx context.Context, req *v1.AdminDeleteExampleQuestionReq) (res *v1.AdminDeleteExampleQuestionRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("闂ID涓嶈兘涓虹┖")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	_, err = g.DB("master").Model("admin_example_question").Ctx(ctx).Where("id = ?", req.Id).Delete()
	_ = insertAdminLog(ctx, "admin", "admin", "示例问题", fmt.Sprintf("删除示例问题ID %d", req.Id), "success")
	return &v1.AdminDeleteExampleQuestionRes{}, err
}

func (c *ControllerV1) AdminQuestionCandidates(ctx context.Context, req *v1.AdminQuestionCandidatesReq) (res *v1.AdminQuestionCandidatesRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	query := g.DB("master").Model("admin_question_candidate").Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	records, err := query.OrderDesc("last_seen_at").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminQuestionCandidatesRes{List: scanQuestionCandidates(records)}, nil
}

func (c *ControllerV1) AdminApproveQuestionCandidate(ctx context.Context, req *v1.AdminApproveQuestionCandidateReq) (res *v1.AdminApproveQuestionCandidateRes, err error) {
	return c.updateCandidateStatus(ctx, req.Id, "approved")
}

func (c *ControllerV1) AdminRejectQuestionCandidate(ctx context.Context, req *v1.AdminRejectQuestionCandidateReq) (res *v1.AdminRejectQuestionCandidateRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	if req.Id <= 0 {
		return nil, gerror.New("鍊欓€夐棶棰業D涓嶈兘涓虹┖")
	}
	item, err := updateQuestionCandidateStatus(ctx, req.Id, "rejected")
	if err != nil {
		return nil, err
	}
	return &v1.AdminRejectQuestionCandidateRes{Item: item}, nil
}

func (c *ControllerV1) AdminDataSources(ctx context.Context, req *v1.AdminDataSourcesReq) (res *v1.AdminDataSourcesRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	records, err := g.DB("master").Model("admin_data_source").Ctx(ctx).OrderAsc("id").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminDataSourcesRes{List: scanDataSources(records)}, nil
}

func (c *ControllerV1) AdminUpdateDataSource(ctx context.Context, req *v1.AdminUpdateDataSourceReq) (res *v1.AdminUpdateDataSourceRes, err error) {
	if req.Type == "" {
		return nil, gerror.New("data source type is required")
	}
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	status := req.Status
	if status == "" {
		if req.Enabled {
			status = "running"
		} else {
			status = "closed"
		}
	}
	now := int(gtime.Timestamp())
	_, err = g.DB("master").Model("admin_data_source").Ctx(ctx).Where("source_type = ?", req.Type).Data(g.Map{
		"enabled":     boolToInt(req.Enabled),
		"status":      status,
		"update_time": now,
	}).Update()
	if err != nil {
		return nil, err
	}
	item, err := getDataSource(ctx, req.Type)
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "数据接入", fmt.Sprintf("更新%s数据源状态为%s", req.Type, status), "success")
	return &v1.AdminUpdateDataSourceRes{Item: item}, nil
}

func (c *ControllerV1) AdminGridImportUpload(ctx context.Context, req *v1.AdminGridImportUploadReq) (res *v1.AdminGridImportUploadRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}

	fileName := strings.TrimSpace(req.FileName)
	month := strings.TrimSpace(req.Month)
	operator := defaultString(req.Operator, "admin")
	cases := []gridCaseRecord{}
	parseErrors := []gridParseError{}
	if request := ghttp.RequestFromCtx(ctx); request != nil {
		if month == "" {
			month = request.Get("month").String()
		}
		if operator == "admin" && request.Get("operator").String() != "" {
			operator = request.Get("operator").String()
		}
		if uploadFile := request.GetUploadFile("file"); uploadFile != nil {
			fileName = uploadFile.Filename
			file, openErr := uploadFile.Open()
			if openErr != nil {
				return nil, openErr
			}
			defer file.Close()
			content, readErr := io.ReadAll(file)
			if readErr != nil {
				return nil, readErr
			}
			cases, parseErrors, err = parseGridImportFile(fileName, content)
			if err != nil {
				return nil, err
			}
		}
	}
	if fileName == "" {
		return nil, gerror.New("请选择要上传的 .xlsx / .xls / .csv 文件")
	}

	totalRows := len(cases) + len(parseErrors)
	if totalRows == 0 && req.TotalRows > 0 {
		totalRows = req.TotalRows
	}
	failedRows := len(parseErrors)
	successRows := totalRows - failedRows
	if successRows < 0 {
		successRows = 0
	}
	status := "completed"
	if failedRows > 0 && successRows > 0 {
		status = "partial_success"
	} else if failedRows > 0 && successRows == 0 {
		status = "failed"
	}
	now := int(gtime.Timestamp())
	db := g.DB("master")
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	id, err := tx.Model("admin_grid_import").Ctx(ctx).Data(g.Map{
		"month":         month,
		"file_name":     fileName,
		"status":        status,
		"total_rows":    totalRows,
		"success_rows":  successRows,
		"failed_rows":   failedRows,
		"operator":      operator,
		"create_time":   now,
		"complete_time": now,
	}).InsertAndGetId()
	if err != nil {
		return nil, err
	}
	for _, item := range cases {
		if item.CaseNumber != "" {
			if _, err = tx.Model("case_list").Ctx(ctx).Where("case_number = ?", item.CaseNumber).Delete(); err != nil {
				return nil, err
			}
		}
		if _, err = tx.Model("case_list").Ctx(ctx).Data(item.toMap()).Insert(); err != nil {
			return nil, err
		}
	}
	for _, parseErr := range parseErrors {
		if _, err = tx.Model("admin_grid_import_error").Ctx(ctx).Data(g.Map{
			"import_id": id,
			"row_index": parseErr.RowIndex,
			"reason":    parseErr.Reason,
			"raw_data":  parseErr.RawData,
		}).Insert(); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	item, err := getGridImport(ctx, int(id))
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", req.Operator, "数据导入", fmt.Sprintf("上传网格数据：%s，成功%d条，失败%d条", fileName, successRows, failedRows), item.Status)
	return &v1.AdminGridImportUploadRes{Item: item}, nil
}

func (c *ControllerV1) AdminGridImports(ctx context.Context, req *v1.AdminGridImportsReq) (res *v1.AdminGridImportsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	records, err := g.DB("master").Model("admin_grid_import").Ctx(ctx).OrderDesc("create_time").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportsRes{List: scanGridImports(records)}, nil
}

func (c *ControllerV1) AdminGridImportDetail(ctx context.Context, req *v1.AdminGridImportDetailReq) (res *v1.AdminGridImportDetailRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	item, err := getGridImport(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportDetailRes{Item: item}, nil
}

func (c *ControllerV1) AdminGridImportErrors(ctx context.Context, req *v1.AdminGridImportErrorsReq) (res *v1.AdminGridImportErrorsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	records, err := g.DB("master").Model("admin_grid_import_error").Ctx(ctx).Where("import_id = ?", req.Id).OrderAsc("row_index").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportErrorsRes{List: scanGridImportErrors(records)}, nil
}

func (c *ControllerV1) AdminLogs(ctx context.Context, req *v1.AdminLogsReq) (res *v1.AdminLogsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	query := g.DB("master").Model("admin_operation_log").Ctx(ctx)
	if req.LogType == "system" || req.LogType == "admin" {
		query = query.Where("log_type = ?", req.LogType)
	}
	records, err := query.OrderDesc("create_time").OrderDesc("id").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminLogsRes{List: scanAdminLogs(records)}, nil
}

func (c *ControllerV1) updateCandidateStatus(ctx context.Context, id int, status string) (*v1.AdminApproveQuestionCandidateRes, error) {
	if err := ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, gerror.New("鍊欓€夐棶棰業D涓嶈兘涓虹┖")
	}
	item, err := updateQuestionCandidateStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}
	return &v1.AdminApproveQuestionCandidateRes{Item: item}, nil
}

func ensureAdminTables(ctx context.Context) error {
	adminEnsureMu.Lock()
	defer adminEnsureMu.Unlock()
	if adminTablesReady {
		return nil
	}
	db := g.DB("master")
	if err := createAdminTables(ctx, db); err != nil {
		return err
	}
	// 修复 MySQL 表结构：raw_data 字段改为 LONGTEXT
	dbType := ""
	if cfg := db.GetConfig(); cfg != nil {
		dbType = cfg.Type
	}
	migrateAdminTables(ctx, db, dbType)
	if dbType != "sqlite" {
		_, _ = db.Exec(ctx, "ALTER TABLE admin_grid_import_error MODIFY COLUMN raw_data LONGTEXT")
	}
	if err := seedAdminTables(ctx, db); err != nil {
		return err
	}
	adminTablesReady = true
	return nil
}

func createAdminTables(ctx context.Context, db gdb.DB) error {
	dbType := ""
	if cfg := db.GetConfig(); cfg != nil {
		dbType = cfg.Type
	}
	sqls := sqliteAdminTableSQL()
	if dbType != "sqlite" {
		sqls = mysqlAdminTableSQL()
	}
	for _, sql := range sqls {
		if _, err := db.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

func migrateAdminTables(ctx context.Context, db gdb.DB, dbType string) {
	if dbType == "sqlite" {
		_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN display_name TEXT")
		_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN last_login_at INTEGER NOT NULL DEFAULT 0")
		return
	}
	_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN display_name VARCHAR(64)")
	_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN last_login_at INT NOT NULL DEFAULT 0")
}

func sqliteAdminTableSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS admin_account (
id INTEGER PRIMARY KEY AUTOINCREMENT,
username TEXT NOT NULL UNIQUE,
password TEXT NOT NULL,
verify INTEGER NOT NULL,
role TEXT NOT NULL DEFAULT 'administrator',
enabled INTEGER NOT NULL DEFAULT 1,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_user_profile (
user_id INTEGER PRIMARY KEY,
department TEXT,
enabled INTEGER NOT NULL DEFAULT 1,
rule_level INTEGER NOT NULL DEFAULT 7,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_example_question (
id INTEGER PRIMARY KEY AUTOINCREMENT,
topic TEXT NOT NULL,
question TEXT NOT NULL,
description TEXT,
enabled INTEGER NOT NULL DEFAULT 1,
sort INTEGER NOT NULL DEFAULT 0,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_question_candidate (
id INTEGER PRIMARY KEY AUTOINCREMENT,
topic TEXT NOT NULL,
question TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'pending',
count INTEGER NOT NULL DEFAULT 1,
last_seen_at INTEGER NOT NULL,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_data_source (
id INTEGER PRIMARY KEY AUTOINCREMENT,
source_type TEXT NOT NULL UNIQUE,
name TEXT NOT NULL,
enabled INTEGER NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'closed',
latest_sync INTEGER NOT NULL DEFAULT 0,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import (
id INTEGER PRIMARY KEY AUTOINCREMENT,
month TEXT,
file_name TEXT NOT NULL,
status TEXT NOT NULL,
total_rows INTEGER NOT NULL DEFAULT 0,
success_rows INTEGER NOT NULL DEFAULT 0,
failed_rows INTEGER NOT NULL DEFAULT 0,
operator TEXT,
create_time INTEGER NOT NULL,
complete_time INTEGER NOT NULL DEFAULT 0
)`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import_error (
id INTEGER PRIMARY KEY AUTOINCREMENT,
import_id INTEGER NOT NULL,
row_index INTEGER NOT NULL,
reason TEXT NOT NULL,
raw_data TEXT
)`,
		`CREATE TABLE IF NOT EXISTS admin_knowledge_base (
code TEXT PRIMARY KEY,
name TEXT NOT NULL,
enabled INTEGER NOT NULL DEFAULT 1,
sort INTEGER NOT NULL DEFAULT 0,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_user_knowledge_permission (
id INTEGER PRIMARY KEY AUTOINCREMENT,
user_id INTEGER NOT NULL,
knowledge_code TEXT NOT NULL,
enabled INTEGER NOT NULL DEFAULT 0,
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS admin_operation_log (
id INTEGER PRIMARY KEY AUTOINCREMENT,
log_type TEXT NOT NULL,
username TEXT NOT NULL,
action_type TEXT NOT NULL,
content TEXT,
result TEXT NOT NULL DEFAULT 'success',
create_time INTEGER NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS case_list (
id INTEGER PRIMARY KEY AUTOINCREMENT,
responsibility_unit TEXT,
case_number TEXT,
case_source TEXT,
report_time TEXT,
pending_step TEXT,
case_type TEXT,
region TEXT,
case_location TEXT,
description TEXT,
create_time TEXT DEFAULT CURRENT_TIMESTAMP,
update_time TEXT DEFAULT CURRENT_TIMESTAMP
)`,
			`CREATE TABLE IF NOT EXISTS chat_session (
	session_id TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL DEFAULT 0,
	topic TEXT,
	source TEXT,
	alert_id INTEGER NOT NULL DEFAULT 0,
	suggested_questions TEXT,
	input_placeholder TEXT,
	create_time INTEGER NOT NULL,
	update_time INTEGER NOT NULL
	)`,
			`CREATE TABLE IF NOT EXISTS qa_sync_task (
	task_id INTEGER PRIMARY KEY AUTOINCREMENT,
	provider TEXT NOT NULL,
	sync_type TEXT NOT NULL DEFAULT 'full',
	status TEXT NOT NULL DEFAULT 'pending',
	message TEXT,
	success_count INTEGER NOT NULL DEFAULT 0,
	failure_count INTEGER NOT NULL DEFAULT 0,
	skipped_count INTEGER NOT NULL DEFAULT 0,
	started_at INTEGER NOT NULL DEFAULT 0,
	finished_at INTEGER NOT NULL DEFAULT 0,
	create_time INTEGER NOT NULL,
	update_time INTEGER NOT NULL
	)`,
			`CREATE TABLE IF NOT EXISTS qa_sync_log (
	log_id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER NOT NULL,
	provider TEXT NOT NULL,
	sync_type TEXT NOT NULL,
	external_id TEXT,
	local_id TEXT,
	action TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT,
	create_time INTEGER NOT NULL
	)`,
			`CREATE TABLE IF NOT EXISTS admin_topic_knowledge_binding (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	topic TEXT NOT NULL,
	knowledge_code TEXT NOT NULL,
	knowledge_name TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	create_time INTEGER NOT NULL,
	update_time INTEGER NOT NULL
	)`,
			`CREATE TABLE IF NOT EXISTS admin_document_relation (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	from_doc_id INTEGER NOT NULL,
	from_doc_title TEXT,
	to_doc_id INTEGER NOT NULL,
	to_doc_title TEXT,
	rel_type TEXT NOT NULL,
	description TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	create_time INTEGER NOT NULL,
	update_time INTEGER NOT NULL
	)`,
			`CREATE TABLE IF NOT EXISTS admin_metric (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	topic TEXT NOT NULL,
	metric_name TEXT NOT NULL,
	display_name TEXT NOT NULL,
	description TEXT,
	unit TEXT,
	dimensions TEXT,
	default_threshold REAL,
	threshold_direction TEXT,
	chart_type_hint TEXT,
	is_active INTEGER NOT NULL DEFAULT 1,
	create_time INTEGER NOT NULL,
	update_time INTEGER NOT NULL
	)`,
	}
}

func mysqlAdminTableSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS admin_account (
id INT PRIMARY KEY AUTO_INCREMENT,
username VARCHAR(64) NOT NULL UNIQUE,
password VARCHAR(64) NOT NULL,
verify INT NOT NULL,
role VARCHAR(32) NOT NULL DEFAULT 'administrator',
enabled TINYINT NOT NULL DEFAULT 1,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_user_profile (
user_id INT PRIMARY KEY,
department VARCHAR(128),
enabled TINYINT NOT NULL DEFAULT 1,
rule_level INT NOT NULL DEFAULT 7,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_example_question (
id INT PRIMARY KEY AUTO_INCREMENT,
topic VARCHAR(32) NOT NULL,
question VARCHAR(255) NOT NULL,
description TEXT,
enabled TINYINT NOT NULL DEFAULT 1,
sort INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_topic_sort (topic, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_question_candidate (
id INT PRIMARY KEY AUTO_INCREMENT,
topic VARCHAR(32) NOT NULL,
question VARCHAR(255) NOT NULL,
status VARCHAR(32) NOT NULL DEFAULT 'pending',
count INT NOT NULL DEFAULT 1,
last_seen_at INT NOT NULL,
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_status_seen (status, last_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_data_source (
id INT PRIMARY KEY AUTO_INCREMENT,
source_type VARCHAR(32) NOT NULL UNIQUE,
name VARCHAR(64) NOT NULL,
enabled TINYINT NOT NULL DEFAULT 0,
status VARCHAR(32) NOT NULL DEFAULT 'closed',
latest_sync INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import (
id INT PRIMARY KEY AUTO_INCREMENT,
month VARCHAR(16),
file_name VARCHAR(255) NOT NULL,
status VARCHAR(32) NOT NULL,
total_rows INT NOT NULL DEFAULT 0,
success_rows INT NOT NULL DEFAULT 0,
failed_rows INT NOT NULL DEFAULT 0,
operator VARCHAR(64),
create_time INT NOT NULL,
complete_time INT NOT NULL DEFAULT 0,
INDEX idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import_error (
id INT PRIMARY KEY AUTO_INCREMENT,
import_id INT NOT NULL,
row_index INT NOT NULL,
reason TEXT NOT NULL,
raw_data LONGTEXT,
INDEX idx_import_id (import_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_knowledge_base (
code VARCHAR(64) PRIMARY KEY,
name VARCHAR(128) NOT NULL,
enabled TINYINT NOT NULL DEFAULT 1,
sort INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_user_knowledge_permission (
id INT PRIMARY KEY AUTO_INCREMENT,
user_id INT NOT NULL,
knowledge_code VARCHAR(64) NOT NULL,
enabled TINYINT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_user_knowledge (user_id, knowledge_code),
INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_operation_log (
id INT PRIMARY KEY AUTO_INCREMENT,
log_type VARCHAR(16) NOT NULL,
username VARCHAR(64) NOT NULL,
action_type VARCHAR(64) NOT NULL,
content TEXT,
result VARCHAR(32) NOT NULL DEFAULT 'success',
create_time INT NOT NULL,
INDEX idx_log_type_time (log_type, create_time),
INDEX idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS case_list (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
responsibility_unit VARCHAR(100),
case_number VARCHAR(100),
case_source VARCHAR(100),
report_time DATETIME,
pending_step VARCHAR(50),
case_type VARCHAR(200),
region VARCHAR(200),
case_location VARCHAR(500),
description TEXT,
create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
update_time DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
INDEX idx_case_number (case_number),
INDEX idx_report_time (report_time),
INDEX idx_case_type (case_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS chat_session (
	session_id VARCHAR(64) PRIMARY KEY,
	user_id INT NOT NULL DEFAULT 0,
	topic VARCHAR(32),
	source VARCHAR(32),
	alert_id INT NOT NULL DEFAULT 0,
	suggested_questions TEXT,
	input_placeholder VARCHAR(255),
	create_time INT NOT NULL,
	update_time INT NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS qa_sync_task (
	task_id INT PRIMARY KEY AUTO_INCREMENT,
	provider VARCHAR(64) NOT NULL,
	sync_type VARCHAR(16) NOT NULL DEFAULT 'full',
	status VARCHAR(32) NOT NULL DEFAULT 'pending',
	message TEXT,
	success_count INT NOT NULL DEFAULT 0,
	failure_count INT NOT NULL DEFAULT 0,
	skipped_count INT NOT NULL DEFAULT 0,
	started_at INT NOT NULL DEFAULT 0,
	finished_at INT NOT NULL DEFAULT 0,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	INDEX idx_provider_status (provider, status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS qa_sync_log (
	log_id INT PRIMARY KEY AUTO_INCREMENT,
	task_id INT NOT NULL,
	provider VARCHAR(64) NOT NULL,
	sync_type VARCHAR(16) NOT NULL,
	external_id VARCHAR(255),
	local_id VARCHAR(255),
	action VARCHAR(32) NOT NULL,
	status VARCHAR(32) NOT NULL,
	message TEXT,
	create_time INT NOT NULL,
	INDEX idx_task_id (task_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS admin_topic_knowledge_binding (
	id INT PRIMARY KEY AUTO_INCREMENT,
	topic VARCHAR(32) NOT NULL,
	knowledge_code VARCHAR(64) NOT NULL,
	knowledge_name VARCHAR(128),
	enabled TINYINT NOT NULL DEFAULT 1,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	UNIQUE KEY uk_topic_code (topic, knowledge_code)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS admin_document_relation (
	id INT PRIMARY KEY AUTO_INCREMENT,
	from_doc_id INT NOT NULL,
	from_doc_title VARCHAR(255),
	to_doc_id INT NOT NULL,
	to_doc_title VARCHAR(255),
	rel_type VARCHAR(32) NOT NULL,
	description TEXT,
	enabled TINYINT NOT NULL DEFAULT 1,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	INDEX idx_from_doc (from_doc_id),
	INDEX idx_to_doc (to_doc_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
			`CREATE TABLE IF NOT EXISTS admin_metric (
	id INT PRIMARY KEY AUTO_INCREMENT,
	topic VARCHAR(32) NOT NULL,
	metric_name VARCHAR(64) NOT NULL,
	display_name VARCHAR(128) NOT NULL,
	description TEXT,
	unit VARCHAR(32),
	dimensions TEXT,
	default_threshold DOUBLE,
	threshold_direction VARCHAR(16),
	chart_type_hint VARCHAR(32),
	is_active TINYINT NOT NULL DEFAULT 1,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	UNIQUE KEY uk_topic_metric (topic, metric_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
}

func seedAdminTables(ctx context.Context, db gdb.DB) error {
	now := int(gtime.Timestamp())
	if count, err := db.Model("admin_account").Ctx(ctx).Count(); err == nil && count == 0 {
		verify := 8137
		password, err := genAdminPassword("admin123", verify)
		if err != nil {
			return err
		}
		if _, err = db.Model("admin_account").Ctx(ctx).Data(g.Map{
			"username":    "admin",
			"password":    password,
			"verify":      verify,
			"role":        "administrator",
			"enabled":     1,
			"create_time": now,
			"update_time": now,
		}).Insert(); err != nil {
			return err
		}
	}
	if count, err := db.Model("admin_example_question").Ctx(ctx).Count(); err == nil && count == 0 {
		seeds := []g.Map{
			{"topic": "grid", "question": "鏈湀楂樻柊鍖烘浠剁粨妗堢巼鏄灏戯紵", "description": "鏌ヨ缃戞牸妗堜欢鍔炵悊鎴愭晥", "enabled": 1, "sort": 10, "create_time": now, "update_time": now},
			{"topic": "population", "question": "杩囧幓涓€鍛ㄤ汉娴佽繘鍑鸿秼鍔垮浣曪紵", "description": "灞曠ず姣忔棩杩涘嚭浜烘暟瀵规瘮", "enabled": 1, "sort": 20, "create_time": now, "update_time": now},
			{"topic": "traffic", "question": "浠婃棩娓境杞﹁締鍗犳瘮鏄灏戯紵", "description": "缁熻閲嶇偣鍗″彛璺ㄥ杞﹁締鎯呭喌", "enabled": 1, "sort": 30, "create_time": now, "update_time": now},
		}
		for _, seed := range seeds {
			if _, err := db.Model("admin_example_question").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
	}
	if count, err := db.Model("admin_data_source").Ctx(ctx).Count(); err == nil && count == 0 {
		seeds := []g.Map{
			{"source_type": "population", "name": "浜烘祦鏁版嵁鎺ュ叆", "enabled": 0, "status": "closed", "latest_sync": 0, "create_time": now, "update_time": now},
			{"source_type": "traffic", "name": "杞︽祦鏁版嵁鎺ュ叆", "enabled": 0, "status": "closed", "latest_sync": 0, "create_time": now, "update_time": now},
			{"source_type": "grid", "name": "缃戞牸鏈堝害瀵煎叆", "enabled": 1, "status": "ready", "latest_sync": 0, "create_time": now, "update_time": now},
		}
		for _, seed := range seeds {
			if _, err := db.Model("admin_data_source").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
	}
	seeds := []g.Map{
		{"code": "policy_files", "name": "政策文件库", "enabled": 1, "sort": 10, "create_time": now, "update_time": now},
		{"code": "laws", "name": "法规知识库", "enabled": 1, "sort": 20, "create_time": now, "update_time": now},
		{"code": "service_guides", "name": "办事指南库", "enabled": 1, "sort": 30, "create_time": now, "update_time": now},
		{"code": "case_library", "name": "历史案例库", "enabled": 1, "sort": 40, "create_time": now, "update_time": now},
	}
	for _, seed := range seeds {
		count, err := db.Model("admin_knowledge_base").Ctx(ctx).Where("code = ?", seed["code"]).Count()
		if err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.Model("admin_knowledge_base").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
		}
		if count, err := db.Model("admin_topic_knowledge_binding").Ctx(ctx).Count(); err == nil && count == 0 {
			bindings := []g.Map{
				{"topic": "grid", "knowledge_code": "policy_files", "knowledge_name": "\u653f\u7b56\u6587\u4ef6\u5e93", "enabled": 1, "create_time": now, "update_time": now},
				{"topic": "grid", "knowledge_code": "case_library", "knowledge_name": "\u5386\u53f2\u6848\u4f8b\u5e93", "enabled": 1, "create_time": now, "update_time": now},
				{"topic": "population", "knowledge_code": "laws", "knowledge_name": "\u6cd5\u89c4\u77e5\u8bc6\u5e93", "enabled": 1, "create_time": now, "update_time": now},
			}
			for _, b := range bindings {
				if _, err := db.Model("admin_topic_knowledge_binding").Ctx(ctx).Data(b).Insert(); err != nil {
					return err
				}
			}
		}
		if count, err := db.Model("admin_metric").Ctx(ctx).Count(); err == nil && count == 0 {
			metrics := []g.Map{
				{"topic": "grid", "metric_name": "grid_close_rate", "display_name": "\u7f51\u683c\u7ed3\u6848\u7387", "description": "\u672c\u6708\u7f51\u683c\u6848\u4ef6\u7ed3\u6848\u5360\u6bd4", "unit": "%", "dimensions": "[]", "default_threshold": 80.0, "threshold_direction": "above", "chart_type_hint": "gauge", "is_active": 1, "create_time": now, "update_time": now},
				{"topic": "population", "metric_name": "population_inflow", "display_name": "\u4eba\u53e3\u6d41\u5165", "description": "\u4e0a\u5468\u4eba\u53e3\u51c0\u6d41\u5165\u6570", "unit": "\u4eba", "dimensions": "[]", "default_threshold": nil, "threshold_direction": "", "chart_type_hint": "line", "is_active": 1, "create_time": now, "update_time": now},
				{"topic": "traffic", "metric_name": "vehicle_ratio", "display_name": "\u8f66\u8f86\u5360\u6bd4", "description": "\u4eca\u65e5\u53e3\u5cb8\u8f66\u8f86\u5360\u6bd4", "unit": "%", "dimensions": "[]", "default_threshold": nil, "threshold_direction": "", "chart_type_hint": "pie", "is_active": 1, "create_time": now, "update_time": now},
			}
			for _, m := range metrics {
				if _, err := db.Model("admin_metric").Ctx(ctx).Data(m).Insert(); err != nil {
					return err
				}
			}
		}
	return nil
}

func listAdminUsers(ctx context.Context) ([]v1.AdminUserItem, error) {
	records, err := g.DB("master").Model("user").Ctx(ctx).Fields("user_id, username, rule_level, last_login_tme, update_time").OrderAsc("user_id").All()
	if err != nil {
		return []v1.AdminUserItem{}, nil
	}
	list := make([]v1.AdminUserItem, 0, len(records))
	for _, record := range records {
		id := record["user_id"].Int()
		profile := getProfileMap(ctx, id)
		ruleLevel := record["rule_level"].Int()
		if profile["rule_level"].Val() != nil {
			ruleLevel = profile["rule_level"].Int()
		}
		enabled := true
		if profile["enabled"].Val() != nil {
			enabled = profile["enabled"].Int() != 0
		}
		displayName := profile["display_name"].String()
		if displayName == "" {
			displayName = record["username"].String()
		}
		lastLoginAt := maxInt(record["last_login_tme"].Int(), profile["last_login_at"].Int())
		list = append(list, v1.AdminUserItem{
			UserId:        id,
			Username:      record["username"].String(),
			DisplayName:   displayName,
			Department:    profile["department"].String(),
			Enabled:       enabled,
			RuleLevel:     ruleLevel,
			Permissions:   permissionsFromRuleLevel(ruleLevel),
			QaPermissions: knowledgePermissionsForUser(ctx, id),
			LastLoginAt:   lastLoginAt,
			UpdateTime:    maxInt(record["update_time"].Int(), profile["update_time"].Int()),
		})
	}
	return list, nil
}

func getAdminUser(ctx context.Context, id int) (v1.AdminUserItem, error) {
	users, err := listAdminUsers(ctx)
	if err != nil {
		return v1.AdminUserItem{}, err
	}
	for _, user := range users {
		if user.UserId == id {
			return user, nil
		}
	}
	return v1.AdminUserItem{UserId: id, Enabled: true, RuleLevel: 7, Permissions: permissionsFromRuleLevel(7)}, nil
}

func upsertUserProfile(ctx context.Context, userId int, data g.Map) error {
	now := int(gtime.Timestamp())
	count, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).Data(data).Update()
		return err
	}
	insertData := g.Map{"user_id": userId, "department": "", "enabled": 1, "rule_level": 7, "create_time": now, "update_time": now}
	for key, value := range data {
		insertData[key] = value
	}
	_, err = g.DB("master").Model("admin_user_profile").Ctx(ctx).Data(insertData).Insert()
	return err
}

func getProfileMap(ctx context.Context, userId int) gdb.Record {
	record, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).One()
	if err != nil || record == nil {
		return gdb.Record{}
	}
	return record
}

func ensureDefaultKnowledgePermissions(ctx context.Context, userId int, ruleLevel int, enabled bool) error {
	if userId <= 0 {
		return nil
	}
	count, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).Count()
	if err != nil || count > 0 {
		return err
	}
	bases, err := g.DB("master").Model("admin_knowledge_base").Ctx(ctx).Fields("code").Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return err
	}
	now := int(gtime.Timestamp())
	for _, base := range bases {
		code := base["code"].String()
		allowed := enabled && (code == "policy_files" || (ruleLevel&7 == 7 && code == "laws"))
		if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Data(g.Map{
			"user_id":        userId,
			"knowledge_code": code,
			"enabled":        boolToInt(allowed),
			"create_time":    now,
			"update_time":    now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func knowledgePermissionsForUser(ctx context.Context, userId int) []v1.KnowledgePermission {
	bases, err := g.DB("master").Model("admin_knowledge_base").Ctx(ctx).Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return []v1.KnowledgePermission{}
	}
	perms, _ := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).All()
	enabledByCode := make(map[string]bool, len(perms))
	for _, perm := range perms {
		enabledByCode[perm["knowledge_code"].String()] = perm["enabled"].Int() != 0
	}
	list := make([]v1.KnowledgePermission, 0, len(bases))
	for _, base := range bases {
		code := base["code"].String()
		list = append(list, v1.KnowledgePermission{
			Code:    code,
			Name:    base["name"].String(),
			Enabled: enabledByCode[code],
		})
	}
	return list
}

func knowledgeBaseOptions(ctx context.Context) []v1.KnowledgePermission {
	bases, err := g.DB("master").Model("admin_knowledge_base").Ctx(ctx).Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return []v1.KnowledgePermission{}
	}
	list := make([]v1.KnowledgePermission, 0, len(bases))
	for _, base := range bases {
		list = append(list, v1.KnowledgePermission{
			Code:    base["code"].String(),
			Name:    base["name"].String(),
			Enabled: true,
		})
	}
	return list
}

func replaceUserKnowledgePermissions(ctx context.Context, userId int, permissions []v1.KnowledgePermission) error {
	now := int(gtime.Timestamp())
	if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).Delete(); err != nil {
		return err
	}
	for _, item := range permissions {
		if item.Code == "" {
			continue
		}
		if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Data(g.Map{
			"user_id":        userId,
			"knowledge_code": item.Code,
			"enabled":        boolToInt(item.Enabled),
			"create_time":    now,
			"update_time":    now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func getExampleQuestion(ctx context.Context, id int) (v1.ExampleQuestionItem, error) {
	record, err := g.DB("master").Model("admin_example_question").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return v1.ExampleQuestionItem{}, err
	}
	if record == nil {
		return v1.ExampleQuestionItem{}, gerror.New("question does not exist")
	}
	return exampleQuestionFromRecord(record), nil
}

func getDataSource(ctx context.Context, sourceType string) (v1.DataSourceItem, error) {
	record, err := g.DB("master").Model("admin_data_source").Ctx(ctx).Where("source_type = ?", sourceType).One()
	if err != nil {
		return v1.DataSourceItem{}, err
	}
	if record == nil {
		return v1.DataSourceItem{}, gerror.New("鏁版嵁婧愪笉瀛樺湪")
	}
	return dataSourceFromRecord(record), nil
}

func getGridImport(ctx context.Context, id int) (v1.GridImportItem, error) {
	record, err := g.DB("master").Model("admin_grid_import").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return v1.GridImportItem{}, err
	}
	if record == nil {
		return v1.GridImportItem{}, gerror.New("import record does not exist")
	}
	return gridImportFromRecord(record), nil
}

func updateQuestionCandidateStatus(ctx context.Context, id int, status string) (v1.QuestionCandidateItem, error) {
	_, err := g.DB("master").Model("admin_question_candidate").Ctx(ctx).Where("id = ?", id).Data(g.Map{
		"status":      status,
		"update_time": int(gtime.Timestamp()),
	}).Update()
	if err != nil {
		return v1.QuestionCandidateItem{}, err
	}
	record, err := g.DB("master").Model("admin_question_candidate").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return v1.QuestionCandidateItem{}, err
	}
	if record == nil {
		return v1.QuestionCandidateItem{}, gerror.New("鍊欓€夐棶棰樹笉瀛樺湪")
	}
	return questionCandidateFromRecord(record), nil
}

func scanExampleQuestions(records gdb.Result) []v1.ExampleQuestionItem {
	list := make([]v1.ExampleQuestionItem, 0, len(records))
	for _, record := range records {
		list = append(list, exampleQuestionFromRecord(record))
	}
	return list
}

func scanQuestionCandidates(records gdb.Result) []v1.QuestionCandidateItem {
	list := make([]v1.QuestionCandidateItem, 0, len(records))
	for _, record := range records {
		list = append(list, questionCandidateFromRecord(record))
	}
	return list
}

func scanDataSources(records gdb.Result) []v1.DataSourceItem {
	list := make([]v1.DataSourceItem, 0, len(records))
	for _, record := range records {
		list = append(list, dataSourceFromRecord(record))
	}
	return list
}

func scanGridImports(records gdb.Result) []v1.GridImportItem {
	list := make([]v1.GridImportItem, 0, len(records))
	for _, record := range records {
		list = append(list, gridImportFromRecord(record))
	}
	return list
}

func scanGridImportErrors(records gdb.Result) []v1.GridImportErrorItem {
	list := make([]v1.GridImportErrorItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.GridImportErrorItem{
			Id:       record["id"].Int(),
			ImportId: record["import_id"].Int(),
			RowIndex: record["row_index"].Int(),
			Reason:   record["reason"].String(),
			RawData:  record["raw_data"].String(),
		})
	}
	return list
}

func scanAdminLogs(records gdb.Result) []v1.AdminLogItem {
	list := make([]v1.AdminLogItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.AdminLogItem{
			Id:         record["id"].Int(),
			LogType:    record["log_type"].String(),
			Username:   record["username"].String(),
			ActionType: record["action_type"].String(),
			Content:    record["content"].String(),
			Result:     record["result"].String(),
			CreateTime: record["create_time"].Int(),
		})
	}
	return list
}

func insertAdminLog(ctx context.Context, logType string, username string, actionType string, content string, result string) error {
	if logType == "" {
		logType = "admin"
	}
	if username == "" {
		username = "admin"
	}
	if result == "" {
		result = "success"
	}
	_, err := g.DB("master").Model("admin_operation_log").Ctx(ctx).Data(g.Map{
		"log_type":    logType,
		"username":    username,
		"action_type": actionType,
		"content":     content,
		"result":      result,
		"create_time": int(gtime.Timestamp()),
	}).Insert()
	return err
}

func exampleQuestionFromRecord(record gdb.Record) v1.ExampleQuestionItem {
	return v1.ExampleQuestionItem{
		Id:          record["id"].Int(),
		Topic:       record["topic"].String(),
		Question:    record["question"].String(),
		Description: record["description"].String(),
		Enabled:     record["enabled"].Int() != 0,
		Sort:        record["sort"].Int(),
		UpdateTime:  record["update_time"].Int(),
	}
}

func questionCandidateFromRecord(record gdb.Record) v1.QuestionCandidateItem {
	return v1.QuestionCandidateItem{
		Id:         record["id"].Int(),
		Topic:      record["topic"].String(),
		Question:   record["question"].String(),
		Status:     record["status"].String(),
		Count:      record["count"].Int(),
		LastSeenAt: record["last_seen_at"].Int(),
		UpdateTime: record["update_time"].Int(),
	}
}

func dataSourceFromRecord(record gdb.Record) v1.DataSourceItem {
	return v1.DataSourceItem{
		Type:       record["source_type"].String(),
		Name:       record["name"].String(),
		Enabled:    record["enabled"].Int() != 0,
		Status:     record["status"].String(),
		LatestSync: record["latest_sync"].Int(),
		UpdateTime: record["update_time"].Int(),
	}
}

func gridImportFromRecord(record gdb.Record) v1.GridImportItem {
	return v1.GridImportItem{
		Id:           record["id"].Int(),
		Month:        record["month"].String(),
		FileName:     record["file_name"].String(),
		Status:       record["status"].String(),
		TotalRows:    record["total_rows"].Int(),
		SuccessRows:  record["success_rows"].Int(),
		FailedRows:   record["failed_rows"].Int(),
		Operator:     record["operator"].String(),
		CreateTime:   record["create_time"].Int(),
		CompleteTime: record["complete_time"].Int(),
	}
}

type gridParseError struct {
	RowIndex int
	Reason   string
	RawData  string
}

type gridCaseRecord struct {
	ResponsibilityUnit string
	CaseNumber         string
	CaseSource         string
	ReportTime         string
	PendingStep        string
	CaseType           string
	Region             string
	CaseLocation       string
	Description        string
}

func (r gridCaseRecord) toMap() g.Map {
	reportTime := interface{}(nil)
	if r.ReportTime != "" {
		reportTime = r.ReportTime
	}
	return g.Map{
		"responsibility_unit": r.ResponsibilityUnit,
		"case_number":         r.CaseNumber,
		"case_source":         r.CaseSource,
		"report_time":         reportTime,
		"pending_step":        r.PendingStep,
		"case_type":           r.CaseType,
		"region":              r.Region,
		"case_location":       r.CaseLocation,
		"description":         r.Description,
	}
}

func parseGridImportFile(fileName string, content []byte) ([]gridCaseRecord, []gridParseError, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	var allRows [][]string
	var err error
	switch ext {
	case ".xlsx":
		allRows, err = readXLSXRows(content)
	case ".xls":
		allRows, err = readXLSRows(content)
	case ".csv":
		allRows, err = readCSVRows(content)
	default:
		return nil, nil, gerror.New("only .xlsx, .xls and .csv files are supported")
	}
	if err != nil {
		return nil, nil, err
	}
	return parseGridCaseRows(allRows)
}

func readXLSXRows(content []byte) ([][]string, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, gerror.Newf("failed to parse Excel file: %v", err)
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return nil, gerror.New("Excel file has no sheet")
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		return nil, gerror.Newf("failed to read rows: %v", err)
	}
	return rows, nil
}

func readXLSRows(content []byte) ([][]string, error) {
	// 中文 Excel 通常使用 GBK 编码
	workbook, err := xls.OpenReader(bytes.NewReader(content), "gbk")
	if err != nil {
		// 如果 GBK 失败，尝试 UTF-8
		workbook, err = xls.OpenReader(bytes.NewReader(content), "utf-8")
		if err != nil {
			return nil, gerror.Newf("failed to parse .xls file: %v", err)
		}
	}
	sheets := workbook.NumSheets()
	if sheets == 0 {
		return nil, gerror.New(".xls file has no sheet")
	}
	sheet := workbook.GetSheet(0)
	if sheet == nil {
		return nil, gerror.New("failed to get first sheet")
	}
	rows := make([][]string, 0)
	for i := 0; i < int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			continue
		}
		cells := make([]string, 0)
		for j := 0; j < row.LastCol(); j++ {
			cell := row.Col(j)
			cells = append(cells, cell)
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

func readCSVRows(content []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, gerror.New("failed to parse CSV file")
	}
	return rows, nil
}

func parseGridCaseRows(rows [][]string) ([]gridCaseRecord, []gridParseError, error) {
	if len(rows) <= 1 {
		return nil, []gridParseError{{RowIndex: 1, Reason: "file has no data rows", RawData: ""}}, nil
	}

	headerMap := buildGridHeaderMap(rows[0])
	missingHeaders := requiredGridHeadersMissing(headerMap)
	if len(missingHeaders) > 0 {
		return nil, nil, gerror.Newf("template columns missing: %s", strings.Join(missingHeaders, ", "))
	}

	cases := make([]gridCaseRecord, 0)
	errors := make([]gridParseError, 0)
	for index, row := range rows {
		if index == 0 || rowIsBlank(row) {
			continue
		}
		item, reason := buildGridCaseRecord(headerMap, row)
		if reason != "" {
			errors = append(errors, gridParseError{
				RowIndex: index + 1,
				Reason:   reason,
				RawData:  rowToJSONWithHeaders(rows[0], row),
			})
			continue
		}
		cases = append(cases, item)
	}
	return cases, errors, nil
}

func buildGridHeaderMap(header []string) map[string]int {
	result := make(map[string]int)
	for index, value := range header {
		key := normalizeGridHeader(value)
		if key != "" {
			result[key] = index
		}
	}
	return result
}

func requiredGridHeadersMissing(headerMap map[string]int) []string {
	required := []string{"责任单位", "案件编号", "案件来源", "上报时间", "待办环节", "案件类别", "所属区域", "案件位置", "问题描述"}
	missing := make([]string, 0)
	for _, header := range required {
		if _, ok := headerMap[normalizeGridHeader(header)]; !ok {
			missing = append(missing, header)
		}
	}
	return missing
}

func buildGridCaseRecord(headerMap map[string]int, row []string) (gridCaseRecord, string) {
	item := gridCaseRecord{
		ResponsibilityUnit: getGridCell(headerMap, row, "责任单位"),
		CaseNumber:         getGridCell(headerMap, row, "案件编号"),
		CaseSource:         getGridCell(headerMap, row, "案件来源"),
		PendingStep:        getGridCell(headerMap, row, "待办环节"),
		CaseType:           getGridCell(headerMap, row, "案件类别"),
		Region:             getGridCell(headerMap, row, "所属区域"),
		CaseLocation:       getGridCell(headerMap, row, "案件位置"),
		Description:        getGridCell(headerMap, row, "问题描述"),
	}
	if item.CaseNumber == "" {
		return item, "case_number is required"
	}
	reportTime := getGridCell(headerMap, row, "上报时间")
	parsedTime, err := normalizeCaseReportTime(reportTime)
	if reportTime != "" && err != nil {
		return item, "invalid report_time"
	}
	item.ReportTime = parsedTime
	return item, ""
}

func getGridCell(headerMap map[string]int, row []string, header string) string {
	index, ok := headerMap[normalizeGridHeader(header)]
	if !ok || index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func normalizeGridHeader(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\t", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func normalizeCaseReportTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if serial, err := strconv.ParseFloat(value, 64); err == nil && serial > 20000 && serial < 100000 {
		if t, err := excelize.ExcelDateToTime(serial, false); err == nil {
			return t.Format("2006-01-02 15:04:05"), nil
		}
	}
	layouts := []string{
		"2006-01-02 15:04:05", "2006/01/02 15:04:05", "2006-1-2 15:04:05", "2006/1/2 15:04:05",
		"2006-01-02 15:04", "2006/01/02 15:04", "2006-1-2 15:04", "2006/1/2 15:04",
		"2006-01-02", "2006/01/02", "2006-1-2", "2006/1/2",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t.Format("2006-01-02 15:04:05"), nil
		}
	}
	return "", fmt.Errorf("unsupported report_time: %s", value)
}

func rowIsBlank(row []string) bool {
	return nonEmptyCellCount(row) == 0
}

func nonEmptyCellCount(row []string) int {
	count := 0
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			count++
		}
	}
	return count
}

func rowToJSONWithHeaders(headers []string, row []string) string {
	data := make(map[string]string)
	for index, header := range headers {
		key := strings.TrimSpace(header)
		if key == "" {
			key = fmt.Sprintf("column_%d", index+1)
		}
		if index < len(row) {
			data[key] = strings.TrimSpace(row[index])
		} else {
			data[key] = ""
		}
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return strings.Join(row, ",")
	}
	return string(bytes)
}
func permissionsFromRuleLevel(ruleLevel int) []v1.TopicPermission {
	return []v1.TopicPermission{
		{Topic: "grid", Enabled: ruleLevel&1 != 0},
		{Topic: "population", Enabled: ruleLevel&2 != 0},
		{Topic: "traffic", Enabled: ruleLevel&4 != 0},
	}
}

func ruleLevelFromPermissions(permissions []v1.TopicPermission) int {
	ruleLevel := 0
	for _, permission := range permissions {
		if !permission.Enabled {
			continue
		}
		switch permission.Topic {
		case "grid":
			ruleLevel |= 1
		case "population":
			ruleLevel |= 2
		case "traffic":
			ruleLevel |= 4
		}
	}
	return ruleLevel
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func genAdminPassword(password string, code int) (string, error) {
	return gmd5.Encrypt(fmt.Sprintf("chatdb-admin:%s:%d", password, code))
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

// AdminCaseList 查询案件列表
func (c *ControllerV1) AdminCaseList(ctx context.Context, req *v1.AdminCaseListReq) (res *v1.AdminCaseListRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}

	db := g.DB("master").Model("case_list")

	// 构建查询条件
	if req.CaseNumber != "" {
		db = db.WhereLike("case_number", "%"+req.CaseNumber+"%")
	}
	if req.CaseType != "" {
		db = db.WhereLike("case_type", "%"+req.CaseType+"%")
	}
	if req.Region != "" {
		db = db.WhereLike("region", "%"+req.Region+"%")
	}
	if req.StartDate != "" {
		db = db.WhereGTE("report_time", req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		db = db.WhereLTE("report_time", req.EndDate+" 23:59:59")
	}

	// 获取总数
	total, err := db.Count()
	if err != nil {
		return nil, err
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	records, err := db.OrderDesc("id").Limit(offset, req.PageSize).All()
	if err != nil {
		return nil, err
	}

	list := make([]v1.CaseListItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.CaseListItem{
			Id:                 r["id"].Int(),
			ResponsibilityUnit: r["responsibility_unit"].String(),
			CaseNumber:         r["case_number"].String(),
			CaseSource:         r["case_source"].String(),
			ReportTime:         r["report_time"].String(),
			PendingStep:        r["pending_step"].String(),
			CaseType:           r["case_type"].String(),
			Region:             r["region"].String(),
			CaseLocation:       r["case_location"].String(),
			Description:        r["description"].String(),
			CreateTime:         r["create_time"].String(),
			UpdateTime:         r["update_time"].String(),
		})
	}

	return &v1.AdminCaseListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// AdminCaseDelete 删除单个案件
func (c *ControllerV1) AdminCaseDelete(ctx context.Context, req *v1.AdminCaseDeleteReq) (res *v1.AdminCaseDeleteRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}

	_, err = g.DB("master").Model("case_list").Ctx(ctx).Where("id = ?", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return &v1.AdminCaseDeleteRes{}, nil
}

// AdminCaseDeleteAll 删除所有案件
func (c *ControllerV1) AdminCaseDeleteAll(ctx context.Context, req *v1.AdminCaseDeleteAllReq) (res *v1.AdminCaseDeleteAllRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}

	result, err := g.DB("master").Model("case_list").Ctx(ctx).Delete()
	if err != nil {
		return nil, err
	}

	deleted, _ := result.RowsAffected()
	return &v1.AdminCaseDeleteAllRes{Deleted: int(deleted)}, nil
}

// AdminCaseStatistics 获取案件统计数据
func (c *ControllerV1) AdminCaseStatistics(ctx context.Context, req *v1.AdminCaseStatisticsReq) (res *v1.AdminCaseStatisticsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}

	db := g.DB("master")

	// 获取总数
	total, _ := db.Model("case_list").Ctx(ctx).Count()

	// 按案件类型统计
	byTypeRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("case_type as name, COUNT(*) as count").
		Where("case_type != ''").
		Group("case_type").
		OrderDesc("count").
		Limit(10).
		All()
	byType := make([]v1.CaseTypeStat, 0)
	for _, r := range byTypeRecords {
		byType = append(byType, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	// 按区域统计
	byRegionRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("region as name, COUNT(*) as count").
		Where("region != ''").
		Group("region").
		OrderDesc("count").
		Limit(10).
		All()
	byRegion := make([]v1.CaseTypeStat, 0)
	for _, r := range byRegionRecords {
		byRegion = append(byRegion, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	// 按来源统计
	bySourceRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("case_source as name, COUNT(*) as count").
		Where("case_source != ''").
		Group("case_source").
		OrderDesc("count").
		Limit(10).
		All()
	bySource := make([]v1.CaseTypeStat, 0)
	for _, r := range bySourceRecords {
		bySource = append(bySource, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	// 按待办环节统计
	byPendingRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("pending_step as name, COUNT(*) as count").
		Where("pending_step != ''").
		Group("pending_step").
		OrderDesc("count").
		Limit(10).
		All()
	byPending := make([]v1.CaseTypeStat, 0)
	for _, r := range byPendingRecords {
		byPending = append(byPending, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	return &v1.AdminCaseStatisticsRes{
		Statistics: v1.CaseStatistics{
			Total:     total,
			ByType:    byType,
			ByRegion:  byRegion,
			BySource:  bySource,
			ByPending: byPending,
		},
	}, nil
}

func (c *ControllerV1) AdminTopicKnowledgeBindings(ctx context.Context, req *v1.AdminTopicKnowledgeBindingsReq) (res *v1.AdminTopicKnowledgeBindingsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	records, err := g.DB("master").Model("admin_topic_knowledge_binding").Ctx(ctx).OrderAsc("id").All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.TopicKnowledgeBindingItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.TopicKnowledgeBindingItem{
			Id:            r["id"].Int(),
			Topic:         r["topic"].String(),
			KnowledgeCode: r["knowledge_code"].String(),
			KnowledgeName: r["knowledge_name"].String(),
			Enabled:       r["enabled"].Int() != 0,
			CreateTime:    r["create_time"].Int(),
			UpdateTime:    r["update_time"].Int(),
		})
	}
	return &v1.AdminTopicKnowledgeBindingsRes{List: list}, nil
}

func (c *ControllerV1) AdminCreateTopicKnowledgeBinding(ctx context.Context, req *v1.AdminCreateTopicKnowledgeBindingReq) (res *v1.AdminCreateTopicKnowledgeBindingRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	kbRecord, err := g.DB("master").Model("admin_knowledge_base").Ctx(ctx).Where("code = ?", req.KnowledgeCode).One()
	if err != nil {
		return nil, err
	}
	knowledgeName := req.KnowledgeCode
	if kbRecord != nil {
		knowledgeName = kbRecord["name"].String()
	}
	now := int(gtime.Timestamp())
	result, err := g.DB("master").Model("admin_topic_knowledge_binding").Ctx(ctx).Data(g.Map{
		"topic":          req.Topic,
		"knowledge_code": req.KnowledgeCode,
		"knowledge_name": knowledgeName,
		"enabled":        1,
		"create_time":    now,
		"update_time":    now,
	}).Insert()
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	record, _ := g.DB("master").Model("admin_topic_knowledge_binding").Ctx(ctx).Where("id = ?", id).One()
	item := v1.TopicKnowledgeBindingItem{}
	if record != nil {
		item = v1.TopicKnowledgeBindingItem{
			Id:            record["id"].Int(),
			Topic:         record["topic"].String(),
			KnowledgeCode: record["knowledge_code"].String(),
			KnowledgeName: record["knowledge_name"].String(),
			Enabled:       record["enabled"].Int() != 0,
			CreateTime:    record["create_time"].Int(),
			UpdateTime:    record["update_time"].Int(),
		}
	}
	return &v1.AdminCreateTopicKnowledgeBindingRes{Item: item}, nil
}

func (c *ControllerV1) AdminToggleTopicKnowledgeBinding(ctx context.Context, req *v1.AdminToggleTopicKnowledgeBindingReq) (res *v1.AdminToggleTopicKnowledgeBindingRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	_, err = g.DB("master").Model("admin_topic_knowledge_binding").Ctx(ctx).Where("id = ?", req.Id).Data(g.Map{
		"enabled":     boolToInt(req.Enabled),
		"update_time": int(gtime.Timestamp()),
	}).Update()
	return &v1.AdminToggleTopicKnowledgeBindingRes{}, err
}

func (c *ControllerV1) AdminDeleteTopicKnowledgeBinding(ctx context.Context, req *v1.AdminDeleteTopicKnowledgeBindingReq) (res *v1.AdminDeleteTopicKnowledgeBindingRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	_, err = g.DB("master").Model("admin_topic_knowledge_binding").Ctx(ctx).Where("id = ?", req.Id).Delete()
	return &v1.AdminDeleteTopicKnowledgeBindingRes{}, err
}

func (c *ControllerV1) AdminDocumentRelations(ctx context.Context, req *v1.AdminDocumentRelationsReq) (res *v1.AdminDocumentRelationsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	records, err := g.DB("master").Model("admin_document_relation").Ctx(ctx).OrderAsc("id").All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.DocumentRelationItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.DocumentRelationItem{
			Id:           r["id"].Int(),
			FromDocId:    r["from_doc_id"].Int(),
			FromDocTitle: r["from_doc_title"].String(),
			ToDocId:      r["to_doc_id"].Int(),
			ToDocTitle:   r["to_doc_title"].String(),
			RelType:      r["rel_type"].String(),
			Description:  r["description"].String(),
			Enabled:      r["enabled"].Int() != 0,
		})
	}
	return &v1.AdminDocumentRelationsRes{List: list}, nil
}

func (c *ControllerV1) AdminCreateDocumentRelation(ctx context.Context, req *v1.AdminCreateDocumentRelationReq) (res *v1.AdminCreateDocumentRelationRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	now := int(gtime.Timestamp())
	_, err = g.DB("master").Model("admin_document_relation").Ctx(ctx).Data(g.Map{
		"from_doc_id":    req.FromDocId,
		"from_doc_title": "",
		"to_doc_id":      req.ToDocId,
		"to_doc_title":   "",
		"rel_type":       req.RelType,
		"description":    req.Description,
		"enabled":        1,
		"create_time":    now,
		"update_time":    now,
	}).Insert()
	return &v1.AdminCreateDocumentRelationRes{}, err
}

func (c *ControllerV1) AdminDeleteDocumentRelation(ctx context.Context, req *v1.AdminDeleteDocumentRelationReq) (res *v1.AdminDeleteDocumentRelationRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	_, err = g.DB("master").Model("admin_document_relation").Ctx(ctx).Where("id = ?", req.Id).Delete()
	return &v1.AdminDeleteDocumentRelationRes{}, err
}

func (c *ControllerV1) AdminAutoDocumentRelation(ctx context.Context, req *v1.AdminAutoDocumentRelationReq) (res *v1.AdminAutoDocumentRelationRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	return &v1.AdminAutoDocumentRelationRes{
		Discovered: []v1.DocumentRelationItem{},
		Created:    0,
	}, nil
}

func (c *ControllerV1) AdminDocumentCompliance(ctx context.Context, req *v1.AdminDocumentComplianceReq) (res *v1.AdminDocumentComplianceRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	return &v1.AdminDocumentComplianceRes{
		Issues: []v1.ComplianceIssueItem{},
	}, nil
}

func (c *ControllerV1) AdminMetrics(ctx context.Context, req *v1.AdminMetricsReq) (res *v1.AdminMetricsRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	records, err := g.DB("master").Model("admin_metric").Ctx(ctx).OrderAsc("id").All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.MetricItem, 0, len(records))
	for _, r := range records {
		var dims []string
		if r["dimensions"].String() != "" {
			_ = json.Unmarshal([]byte(r["dimensions"].String()), &dims)
		}
		var threshold *float64
		if r["default_threshold"].Val() != nil {
			v := r["default_threshold"].Float64()
			threshold = &v
		}
		list = append(list, v1.MetricItem{
			Id:                 r["id"].Int(),
			Topic:              r["topic"].String(),
			MetricName:         r["metric_name"].String(),
			DisplayName:        r["display_name"].String(),
			Description:        r["description"].String(),
			Unit:               r["unit"].String(),
			Dimensions:         dims,
			DefaultThreshold:   threshold,
			ThresholdDirection: r["threshold_direction"].String(),
			ChartTypeHint:      r["chart_type_hint"].String(),
			IsActive:           r["is_active"].Int() != 0,
		})
	}
	return &v1.AdminMetricsRes{List: list}, nil
}

func (c *ControllerV1) AdminCreateMetric(ctx context.Context, req *v1.AdminCreateMetricReq) (res *v1.AdminCreateMetricRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	now := int(gtime.Timestamp())
	dimsJSON, _ := json.Marshal([]string{})
	thresholdStr := "NULL"
	if req.DefaultThreshold != nil {
		thresholdStr = fmt.Sprintf("%f", *req.DefaultThreshold)
	}
	_, err = g.DB("master").Model("admin_metric").Ctx(ctx).Data(g.Map{
		"topic":               req.Topic,
		"metric_name":         req.MetricName,
		"display_name":        req.DisplayName,
		"description":         "",
		"unit":                req.Unit,
		"dimensions":          string(dimsJSON),
		"default_threshold":   thresholdStr,
		"threshold_direction": req.ThresholdDirection,
		"chart_type_hint":     req.ChartTypeHint,
		"is_active":           1,
		"create_time":         now,
		"update_time":         now,
	}).Insert()
	return &v1.AdminCreateMetricRes{}, err
}

func (c *ControllerV1) AdminUpdateMetric(ctx context.Context, req *v1.AdminUpdateMetricReq) (res *v1.AdminUpdateMetricRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	data := g.Map{"update_time": int(gtime.Timestamp())}
	if req.Topic != "" {
		data["topic"] = req.Topic
	}
	if req.MetricName != "" {
		data["metric_name"] = req.MetricName
	}
	if req.DisplayName != "" {
		data["display_name"] = req.DisplayName
	}
	if req.Description != "" {
		data["description"] = req.Description
	}
	if req.Unit != "" {
		data["unit"] = req.Unit
	}
	if req.Dimensions != nil {
		dimsJSON, _ := json.Marshal(req.Dimensions)
		data["dimensions"] = string(dimsJSON)
	}
	if req.DefaultThreshold != nil {
		data["default_threshold"] = fmt.Sprintf("%f", *req.DefaultThreshold)
	}
	if req.ThresholdDirection != "" {
		data["threshold_direction"] = req.ThresholdDirection
	}
	if req.ChartTypeHint != "" {
		data["chart_type_hint"] = req.ChartTypeHint
	}
	_, err = g.DB("master").Model("admin_metric").Ctx(ctx).Where("id = ?", req.Id).Data(data).Update()
	return &v1.AdminUpdateMetricRes{}, err
}

func (c *ControllerV1) AdminDeleteMetric(ctx context.Context, req *v1.AdminDeleteMetricReq) (res *v1.AdminDeleteMetricRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	_, err = g.DB("master").Model("admin_metric").Ctx(ctx).Where("id = ?", req.Id).Delete()
	return &v1.AdminDeleteMetricRes{}, err
}

func (c *ControllerV1) AdminToggleMetric(ctx context.Context, req *v1.AdminToggleMetricReq) (res *v1.AdminToggleMetricRes, err error) {
	if err = ensureAdminTables(ctx); err != nil {
		return nil, err
	}
	var isActive int
	record, _ := g.DB("master").Model("admin_metric").Ctx(ctx).Where("id = ?", req.Id).One()
	if record != nil {
		isActive = record["is_active"].Int()
	}
	newVal := 1
	if isActive != 0 {
		newVal = 0
	}
	_, err = g.DB("master").Model("admin_metric").Ctx(ctx).Where("id = ?", req.Id).Data(g.Map{
		"is_active":   newVal,
		"update_time": int(gtime.Timestamp()),
	}).Update()
	return &v1.AdminToggleMetricRes{}, err
}
