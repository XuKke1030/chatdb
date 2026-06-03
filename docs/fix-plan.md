# ChatDB 修复方案

> 基于代码审查报告，结合项目实际情况（Redis 未用于业务逻辑、UIAP 已对接、Admin 登录保持本地认证）制定

---

## 一、修复范围调整说明

### 1. Redis 可以移除

Redis 在项目中**仅用于 MCP 工具**（`mcp_tool_redis.go`），让 AI Agent 可执行任意 Redis 命令。核心业务（认证、聊天、SQL 执行、MQTT、聚合刷新）**完全不依赖 Redis**。

- `config.yaml` 无 Redis 配置段
- `.env.example` 无 Redis 环境变量
- 无 Docker Compose Redis 服务
- 业务代码中 `g.Redis()` 调用数为 0

**结论**：移除 Redis MCP 工具及其全部关联代码。一并消除 #2（Redis 命令无安全限制）P0 问题。

### 2. UIAP 已对接，Admin 保持本地认证

项目中已完整实现 UIAP 统一身份认证：

- `internal/logic/uiap/client.go` — OAuth2 授权码交换、用户信息、权限查询
- `internal/logic/uiap/persistence.go` — 用户同步、权限绑定
- `internal/logic/uiap/bootstrap.go` — 权限轮询（300s 间隔）
- `config.yaml` 已有 `uiap` 配置段（默认 `enabled: false`）

Admin 登录走本地 `admin_account` 表 +加盐 MD5，不会也不应走 UIAP。

**结论**：Admin 登录逻辑不需要对接 UIAP，但需要从 Controller 下移到 Logic 层。

---

## 二、分阶段修复计划

### 阶段 1 — 安全边界加固（1-2 天）

#### 1.1 移除 Redis MCP 工具 [消除 #2]

**删除文件**：
- `internal/logic/mcp/mcp_tool_redis.go`

**修改文件**：
- `internal/logic/mcp/handler.go` — 删除 `ExecRedisCommand` 注册
- `internal/service/mcp.go` — 从 `IMcpTool` 接口删除 `ExecRedisCommand` 方法
- `internal/model/config.go` — 删除 `RedisConfig` 结构体和 `ConfigData.Redis` 字段
- `internal/consts/consts.go` — 删除 Redis 配置加载（`CHATDB_REDIS_ADDRESS`、`CHATDB_REDIS_PASSWORD`、`CHATDB_REDIS_DB`）及默认值逻辑
- `utility/err.go` — 删除 Redis 连接错误的用户端映射分支
- `utility/err_test.go` — 删除 Redis 相关测试用例

#### 1.2 Shell 工具改为白名单 [修复 #1]

**文件**：`internal/logic/mcp/mcp_tool_shell.go`

将 `validateSafeCommand` 从黑名单改为白名单：

```go
var allowedCommands = map[string]bool{
    "ls":     true,
    "cat":    true,
    "head":   true,
    "tail":   true,
    "grep":   true,
    "wc":     true,
    "find":   true,
    "du":     true,
    "df":     true,
    "pwd":    true,
    "echo":   true,
    "date":   true,
    "uname":  true,
    "whoami": true,
}

func validateSafeCommand(cmd string) error {
    parts := strings.Fields(cmd)
    if len(parts) == 0 {
        return errors.New("empty command")
    }
    // 只允许单条白名单命令，禁止管道、重定向、分号
    if strings.ContainsAny(cmd, "|;&$\n`") {
        return errors.New("pipes, redirections and chaining are not allowed")
    }
    base := filepath.Base(parts[0])
    if !allowedCommands[base] {
        return fmt.Errorf("command %q is not in the allowlist", base)
    }
    return nil
}
```

删除旧的 `bannedOperators`、`bannedFragments`、`validateFirstToken` 等。

#### 1.3 SQL 安全改为连接层只读 [修复 #3]

**文件**：`internal/logic/mcp/mcp_tool_db.go`

方案：在建立 MCP 专用数据库连接时，立即执行 `SET SESSION TRANSACTION READ ONLY`，后续所有 SQL（无论怎么构造）都只能读。

```go
func (s *sMcpTool) getOrCreateConn(ctx context.Context, dbId int) (*gdb.DB, error) {
    // 从缓存获取或新建连接
    conn, err := gdb.New(configNode)
    if err != nil {
        return nil, err
    }
    // 连接层设只读，彻底杜绝写入
    _, _ = conn.Exec(ctx, "SET SESSION TRANSACTION READ ONLY")
    return conn, nil
}
```

保留 `isReadOnlySQL` 作为前端快速拦截（减少无意义只读请求打到数据库），但不再作为唯一防线。

#### 1.4 JWT 空密钥启动校验 [修复 #11]

**文件**：`internal/consts/consts.go`

在 `initConfig()` 末尾增加校验：

```go
for _, opt := range Config.Jwt {
    if opt.Secret == "" {
        panic(fmt.Sprintf("JWT secret for subject %q must not be empty, set CHATDB_JWT_SECRET or config.yaml", opt.Subject))
    }
}
```

同时为 Admin JWT 增加 env 覆盖路径：

```go
if adminSecret := genv.Get("CHATDB_ADMIN_JWT_SECRET"); adminSecret != "" {
    for i, opt := range Config.Jwt {
        if opt.Subject == "ai-chat-admin" {
            Config.Jwt[i].Secret = adminSecret
        }
    }
}
```

更新 `.env.example` 增加 `CHATDB_ADMIN_JWT_SECRET` 说明。

---

### 阶段 2 — 资源泄漏修复（1 天）

#### 2.1 Chat channel 统一关闭 [修复 #4]

**文件**：`internal/logic/ai/chat.go`

在 `Chat` 函数入口处用 `chanCloser` 统一保障：

```go
func (s *sAi) Chat(ctx context.Context, ... , respChan chan model.ChatOutDataItem) {
    closer := newChanCloser(respChan)
    defer closer.Close()

    // 所有错误退出路径只需 cancel() + return
    // closer.Close() 由 defer 保证执行
```

`chanCloser` 已在 `chan_out.go` 中定义，直接使用即可。

#### 2.2 Stream timer 改用 NewTimer + Reset [修复 #5]

**文件**：`internal/logic/ai/chan_out.go`

将 `streamWithTimeout` 中的 `time.After` 循环改为：

```go
timer := time.NewTimer(30 * time.Second)
defer timer.Stop()

for {
    select {
    case <-ctx.Done():
        // context cancelled
        return
    case <-timer.C:
        // timeout - send end event
        return
    case res := <-chCh:
        if !timer.Stop() {
            select {
            case <-timer.C:
            default:
            }
        }
        timer.Reset(15 * time.Second)
        // process chunk...
    }
}
```

---

### 阶段 3 — 数据一致性（2-3 天）

#### 3.1 聚合刷新改为事务 [修复 #7]

**文件**：
- `internal/logic/traffic/aggregate_refresh.go`
- `internal/logic/population/aggregate_refresh.go`

每个 refresh 函数包裹事务：

```go
func (s *sTraffic) RefreshMetricHourly(ctx context.Context, date string) error {
    return g.DB("master").Transaction(ctx, func(tx *gdb.TX) error {
        if _, err := tx.Exec(ctx, "DELETE FROM traffic_metric_hourly WHERE ..."); err != nil {
            return err
        }
        _, err := tx.Exec(ctx, "INSERT INTO traffic_metric_hourly SELECT ...")
        return err
    })
}
```

#### 3.2 upsert 改为单条 SQL [修复 #22]

**文件**：`internal/controller/ai_chat/ai_chat_v1_chats.go`

将 `upsertAskNumberQuestionStat` 从 SELECT+INSERT/UPDATE 改为：

```go
_, err := db.Exec(ctx, `
    INSERT INTO ask_number_question_stat (user_id, topic, question_normalized, hit_count, last_asked_at)
    VALUES (?, ?, ?, 1, ?)
    ON DUPLICATE KEY UPDATE
        hit_count = hit_count + 1,
        last_asked_at = VALUES(last_asked_at)
`, userId, topic, questionNormalized, now)
```

#### 3.3 DELETE+UPDATE 包裹事务 [修复 #23]

**文件**：`internal/controller/ai_chat/ai_chat_v1_chats.go`

- `ChatSessionReset`：用 `db.Transaction` 包裹 DELETE + UPDATE
- `appendAskNumberMessage`：用 `db.Transaction` 包裹 INSERT + UPDATE

#### 3.4 关键路径错误不再吞掉 [修复 #25]

优先修复以下关键路径（审计日志、DDL、权限）：

| 位置 | 修复方式 |
|------|---------|
| `admin_v1.go` 所有 `insertAdminLog` | 改为 `if err := insertAdminLog(...); err != nil { consts.Logger.Warning(ctx, "audit log failed: %v", err) }` |
| `admin_v1.go:696-698` DDL 迁移 | 返回 error，由调用方决定是否继续 |
| `user_v1_user.go:92-103` 登录后更新 | 记录 Warning 日志 |
| `ai_chat_v1_chats.go:254-275` SessionReset | DELETE 错误直接返回 |
| `ai_chat_v1_chats.go:593` suggestedQuestions | 传入 ctx 而非 context.Background() |

---

### 阶段 4 — 并发与性能（2-3 天）

#### 4.1 定时任务添加分布式锁 [修复 #8]

**文件**：
- `internal/logic/sync/scheduler.go`
- `internal/logic/traffic/aggregate_refresh.go`
- `internal/logic/population/aggregate_refresh.go`
- `internal/logic/precipitate/precipitate.go`

由于 Redis 已移除，使用数据库行锁实现简易分布式互斥：

```go
func tryAcquireLock(ctx context.Context, lockKey string, ttlMinutes int) (bool, error) {
    result, err := g.DB("master").Model("distributed_lock").
        Ctx(ctx).
        Where("lock_key = ? AND (locked_until IS NULL OR locked_until < NOW())", lockKey).
        Data(g.Map{
            "locked_by":    consts.Config.Server.Name,
            "locked_until": gtime.Now().Add(time.Duration(ttlMinutes) * time.Minute),
        }).
        Update()
    if err != nil {
        return false, err
    }
    return result.RowsAffected() > 0, nil
}

func releaseLock(ctx context.Context, lockKey string) {
    _, _ = g.DB("master").Model("distributed_lock").
        Ctx(ctx).
        Where("lock_key = ? AND locked_by = ?", lockKey, consts.Config.Server.Name).
        Data(g.Map{"locked_until": nil, "locked_by": ""}).
        Update()
}
```

建表：

```sql
CREATE TABLE IF NOT EXISTS distributed_lock (
    lock_key    VARCHAR(128) PRIMARY KEY,
    locked_by   VARCHAR(128),
    locked_until DATETIME,
    INDEX idx_expired (locked_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### 4.2 案件分析添加 LIMIT 和时间范围 [修复 #20]

**文件**：`internal/controller/ai_chat/ai_chat_v1_chats.go`

`analyzeGridMajorCase` 函数：

```go
query := s.model(ctx, "case_list").
    Where("report_time >= ?", gtime.Now().AddDate(0, 0, -30)).  // 最近30天
    OrderDesc("id").
    Limit(500)  // 最多500条
```

同时 `grid_case_record` 表查询也加同样约束。

#### 4.3 管理接口添加分页 [修复 #21]

**文件**：`internal/controller/admin/admin_v1.go`

| 函数 | 修复 |
|------|------|
| `listAdminUsers` | 改 `All()` 为 `Page(req.Page, req.PageSize)`，请求结构加 `Page int d:"1" v:"min:1"` / `PageSize int d:"20" v:"min:1|max:200"` |
| `AdminGridImports` | 同上 |
| `AdminLogs` | 同上，默认按时间倒序 |
| `AdminDataSources` | 同上 |
| 知识库查询（1051,1073,1106行） | 加 `Limit(200)` |
| `AdminCaseList` | `req.Page`/`req.PageSize` 加 `v:"min:1"` 校验 |

#### 4.4 MQTT 指数退避 [修复 #6]

**文件**：`internal/logic/traffic/mqtt.go`

```go
type backoff struct {
    attempts int
    mu       sync.Mutex
}

func (b *backoff) next() time.Duration {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.attempts++
    d := time.Duration(math.Min(float64(5*math.Pow(2, float64(b.attempts-1))), 60)) * time.Second
    return d
}

func (b *backoff) reset() {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.attempts = 0
}
```

在 `SetConnectionLostHandler` 中调用 `b.next()` 计算下次间隔，在 `SetOnConnectHandler` 中调用 `b.reset()`。

#### 4.5 MQTT 改为批量写入 [修复 #9]

**文件**：`internal/logic/traffic/ingest.go`

引入 channel + 定时批量刷新：

```go
var gateRecordCh = make(chan *model.GateRecord, 1000)

func init() {
    go batchFlush()
}

func batchFlush() {
    ticker := time.NewTicker(2 * time.Second)
    var batch []*model.GateRecord
    for {
        select {
        case r := <-gateRecordCh:
            batch = append(batch, r)
            if len(batch) >= 100 {
                flush(batch)
                batch = nil
            }
        case <-ticker.C:
            if len(batch) > 0 {
                flush(batch)
                batch = nil
            }
        }
    }
}
```

`HandleRawPayload` 改为解析后 `gateRecordCh <- record`，不再逐条写 DB。

---

### 阶段 5 — 安全加固（1-2 天）

#### 5.1 导出文件名清洗 [修复 #10]

**文件**：`internal/logic/mcp/mcp_tool_export.go`

```go
safeName := regexp.MustCompile(`[^\w\-.]`).ReplaceAllString(fileName, "_")
filePath := filepath.Join(exportDir, safeName+".xlsx")
// 额外验证：确保最终路径仍在 exportDir 下
absPath, _ := filepath.Abs(filePath)
absDir, _ := filepath.Abs(exportDir)
if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
    return nil, fmt.Errorf("invalid file path")
}
```

#### 5.2 LIKE 通配符转义 [修复 #24]

**文件**：`internal/controller/ai_chat/ai_chat_v1_chats.go`、`internal/controller/admin/admin_v1.go`

添加工具函数：

```go
func escapeLike(s string) string {
    s = strings.ReplaceAll(s, `\`, `\\`)
    s = strings.ReplaceAll(s, `%`, `\%`)
    s = strings.ReplaceAll(s, `_`, `\_`)
    return s
}
```

所有 `WhereLike("col", "%"+userInput+"%")` 改为 `WhereLike("col", "%"+escapeLike(userInput)+"%")`。

#### 5.3 全局可变状态改为受控访问 [修复 #12]

**文件**：`internal/consts/consts.go`

将 `McpClient` 和 `PrivateKey` 改为私有变量 + sync.Once getter：

```go
var (
    mcpClient     *mcpclient.Client
    mcpClientOnce sync.Once
    mcpClientMu   sync.RWMutex
)

func McpClientInstance() *mcpclient.Client {
    mcpClientMu.RLock()
    defer mcpClientMu.RUnlock()
    return mcpClient
}

func SetMcpClient(c *mcpclient.Client) {
    mcpClientMu.Lock()
    defer mcpClientMu.Unlock()
    mcpClient = c
}
```

同样处理 `PrivateKey`、`SystemConfig`、`Config`。

#### 5.4 context key 改为不导出类型 [修复 #13]

**文件**：`internal/model/jwt.go`

```go
// userGroupKey is the context key for storing authenticated user ID.
type userGroupKey struct{}
```

同时更新所有引用 `model.UserGroup{}` 的地方改为 `model.userGroupKey{}`。由于跨包引用，改用接口：

```go
// jwt.go 中提供公开的 Context 函数
func UserFromContext(ctx context.Context) string {
    v, _ := ctx.Value(userGroupKey{}).(string)
    return v
}

func ContextWithUser(ctx context.Context, userId string) context.Context {
    return context.WithValue(ctx, userGroupKey{}, userId)
}
```

---

### 阶段 6 — 代码质量（1-2 天）

#### 6.1 panic recover 修复 [修复 #16]

**文件**：`internal/logic/mcp/handler.go`

```go
defer func() {
    if r := recover(); r != nil {
        consts.Logger.Printf(ctx, "panic in MCP tool: %+v", r)
        out = mcp.NewToolResultText(fmt.Sprintf("tool execution error: %v", r))
        err = nil  // 已处理，不向框架返回 error
    }
}()
```

#### 6.2 统一表名去重机制 [修复 #17]

**文件**：`internal/logic/mcp/mcp_tool_db.go`

删除 `AddTable` 函数及 `chat.go` 中的 `tablesPtr` 传参，统一使用 `addTableToContext`（基于 session 的 context 存储）。

#### 6.3 删除死代码 [修复 #18]

**文件**：`internal/logic/mcp/mcp_tool_time.go`

删除第 32 行未使用的 `gtime.NewFromTimeStamp(...)` 调用。

#### 6.4 Holiday 同步错误记录 [修复 #19]

**文件**：`internal/logic/traffic/aggregate_refresh.go`

将 `_, _ = db.Exec(ctx, ...)` 改为：

```go
if _, err := db.Exec(ctx, sql, args...); err != nil {
    consts.Logger.Warningf(ctx, "holiday sync failed: %v", err)
    failCount++
}
```

函数末尾：

```go
if failCount > 0 {
    return fmt.Errorf("holiday sync: %d statements failed", failCount)
}
return nil
```

#### 6.5 Controller 输入校验补全 [修复 #27]

已确认 SpeechTranscribe 已有 10MB 限制，无需修复。

需要修复的：

| 接口 | 修复 |
|------|------|
| `AdminCaseList` | req 结构加 `v:"min:1"` |
| `AdminCaseStatistics` | month 格式校验 `v:"regex:^\d{4}-\d{2}$"` |
| `ChatSessionReset` | sessionId 加 `v:"required\|length:1,64"` |
| `SetDataBaseConfig` / `UpdateDataBaseConfig` | port 加 `v:"between:1,65535"` |
| `AdminAidgpConnectionTest` | req 字段加 `v:"required"` |
| `UserPopularQuestions` | limit 加 `v:"between:1,50"` |

---

### 阶段 7 — 架构改善（持续，按模块拆分）

#### 7.1 Controller 业务逻辑下移 [修复 #26]

按优先级分批：

**第一批（DDL 移出 Controller）**：
- `CreateAskNumberSessionTables` → `internal/logic/schema/schema.go`
- `CreateAdminTables` / `MigrateAdminTables` → `internal/logic/schema/schema.go`
- `CreateQaTables` / `SeedQaTables` → `internal/logic/schema/schema.go`

**第二批（Auth 逻辑下移）**：
- `AdminLogin` → `internal/logic/admin/auth.go`（密码验证、Token 生成、审计日志）
- `UiapCallback` 中的直接 DB 写入 → 调用 `service.Uiap().ApplyPermissions()`

**第三批（评分算法下移）**：
- `analyzeGridMajorCase` / `scoreMajorCase` → `internal/logic/grid/analysis.go`

**第四批（CRUD 下移）**：
- 数据库配置 CRUD → `internal/logic/database/config.go`
- 管理后台各 CRUD → 对应 Logic 模块

---

## 三、问题修复覆盖矩阵

| 编号 | 优先级 | 问题 | 阶段 | 修复方式 |
|------|--------|------|------|---------|
| #1 | P0 | Shell 黑名单可绕过 | 1 | 改白名单 |
| #2 | P0 | Redis 命令无限制 | 1 | 移除 Redis 工具 |
| #3 | P0 | isReadOnlySQL 可绕过 | 1 | 连接层只读 + 保留前端拦截 |
| #4 | P0 | Channel 关闭不完整 | 2 | defer closer.Close() |
| #5 | P0 | Timer goroutine 泄漏 | 2 | NewTimer + Reset |
| #20 | P0 | 案件分析全表扫描 | 4 | 加 LIMIT + 时间范围 |
| #6 | P1 | MQTT 无指数退避 | 4 | 指数退避实现 |
| #7 | P1 | DELETE+INSERT 非事务 | 3 | 包裹 Transaction |
| #8 | P1 | 定时任务无并发锁 | 4 | 数据库行锁 |
| #9 | P1 | MQTT 逐条写入 | 4 | Channel + 批量刷写 |
| #10 | P1 | 导出路径遍历 | 5 | 文件名清洗 + 路径校验 |
| #11 | P1 | JWT 空密钥 | 1 | 启动校验 + Admin env 覆盖 |
| #12 | P1 | 全局可变状态 | 5 | 私有化 + RWMutex getter |
| #13 | P1 | 空 UserGroup context key | 5 | 不导出类型 + 公开访问函数 |
| #21 | P1 | 管理接口无分页 | 4 | Page/PageSize + 校验 |
| #22 | P1 | upsert 竞态 | 3 | ON DUPLICATE KEY UPDATE |
| #23 | P1 | DELETE+UPDATE 非事务 | 3 | 包裹 Transaction |
| #24 | P1 | LIKE 注入 | 5 | escapeLike 工具函数 |
| #25 | P1 | 错误吞掉 | 3 | 关键路径改为 Warning/返回 |
| #26 | P1 | Controller 逻辑泄漏 | 7 | 分批下移到 Logic |
| #27 | P1 | 输入校验缺失 | 6 | v 标签 + 逻辑校验 |
| #14 | P2 | Model 标签缺失 | 6 | 逐文件补全 |
| #15 | P2 | interface{} 可具体化 | 7 | 重构时逐步替换 |
| #16 | P2 | panic recover 吞错误 | 6 | 设置 named return |
| #17 | P2 | 重复表名去重 | 6 | 统一为 context 方案 |
| #18 | P2 | 死代码 | 6 | 删除 |
| #19 | P2 | Holiday 同步忽略错误 | 6 | 记录 + 汇总 |

---

## 四、移除 Redis 的完整改动清单

| 操作 | 文件 |
|------|------|
| 删除 | `internal/logic/mcp/mcp_tool_redis.go` |
| 修改 | `internal/logic/mcp/handler.go` — 删除 ExecRedisCommand 注册 |
| 修改 | `internal/service/mcp.go` — 接口删除 ExecRedisCommand |
| 修改 | `internal/model/config.go` — 删除 RedisConfig 和 ConfigData.Redis |
| 修改 | `internal/consts/consts.go` — 删除 CHATDB_REDIS_* 环境变量加载 |
| 修改 | `utility/err.go` — 删除 Redis 错误映射 |
| 修改 | `utility/err_test.go` — 删除 Redis 测试用例 |
