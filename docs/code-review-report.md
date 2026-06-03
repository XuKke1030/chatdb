# ChatDB 项目深度 Code Review 报告

> 审查日期：2026-06-03
> 审查范围：全项目（Model / DAO / Logic / Controller / Security）
> 项目定位：AI 驱动的智能数据分析助手，面向移动端业务用户
> 技术栈：Go 1.25 / GoFrame v2 / CloudWeGo Eino / MCP / MySQL / MQTT / JWT

---

## 目录

- [一、审查概览](#一审查概览)
- [二、P0 — 高危问题（必须修复）](#二p0--高危问题必须修复)
- [三、P1 — 中危问题（应该修复）](#三p1--中危问题应该修复)
- [四、P2 — 低危/代码质量问题](#四p2--低危代码质量问题)
- [五、Controller 层架构问题](#五controller-层架构问题)
- [六、问题统计总表](#六问题统计总表)
- [七、建议修复顺序与排期](#七建议修复顺序与排期)
- [附录：审查方法论](#附录审查方法论)

---

## 一、审查概览

### 分层审查结果

| 层级 | 文件数 | 问题数 | 关键发现 |
|------|--------|--------|----------|
| Model | 15+ | 14 | 大范围缺少校验标签、导出结构体无注释、interface{} 可具体化 |
| DAO | 6 | 2 | DAO 本身安全（ORM 参数化），消费者层存在无 LIMIT 查询 |
| Logic | 18+ | 12 | 并发安全、channel 泄漏、timer 泄漏、非事务写入 |
| Controller | 10+ | 15 | 全表扫描 OOM、竞态条件、LIKE 注入、错误吞掉、业务逻辑泄漏 |
| Security | 全局 | 6 | Shell/Redis/SQL 黑名单可绕过、JWT 空密钥、路径遍历 |

### 整体评价

- **优点**：GoFrame ORM 参数化查询普遍到位，SQL 注入在 DAO 层已有效防御；MCP 工具设计思路正确（只读 SQL 检查、Shell 黑名单）；SSE 流式输出状态机设计清晰；`SanitizeOutput` 和 `SafeUserErr` 对用户隐藏了内部细节；`traffic_v1.go` 控制器是唯一正确委托的范例。
- **核心风险**：安全防护依赖黑名单/正则，在 AI 可自主调用工具的场景下存在系统性绕过风险；多个关键路径存在资源泄漏和数据一致性隐患；Controller 层存在严重的业务逻辑泄漏。

---

## 二、P0 — 高危问题（必须修复）

### #1 Shell 工具黑名单可绕过

**文件**: `internal/logic/mcp/mcp_tool_shell.go:109-157`

**问题**: `validateSafeCommand` 基于黑名单过滤，在 AI 自主调用工具的场景下存在多种绕过路径：

1. 管道 `|` 被放行，AI 可构造 `echo hacked | bash` 实现任意执行
2. Base64 工具 + Shell 工具组合：AI 可先调用 Base64 编码恶意命令，再用 `echo <encoded> | base64 -d | sh` 解码执行，绕过黑名单检测
3. Windows 下 `cmd /c` 引号内的 `&` 不会被字符串匹配检测到
4. Unicode/编码等价字符可绕过 `strings.Contains` 检测

**建议**: Shell 工具改为严格白名单模式（只允许预定义的安全命令如 `ls`/`cat`/`grep`/`wc`），或直接移除此 MCP 工具。黑名单在 AI 场景下不可接受。

---

### #2 Redis 命令无安全限制

**文件**: `internal/logic/mcp/mcp_tool_redis.go:17-66`

**问题**: `ExecRedisCommand` 将 AI 传入的 `command` 和 `args` 直接传给 `conn.Do(ctx, command, args...)`，没有任何命令过滤：

- AI 可执行 `FLUSHALL` 清空全部数据
- AI 可执行 `CONFIG SET` 修改 Redis 配置
- AI 可执行 `EVAL` 执行任意 Lua 脚本（Lua 沙箱逃逸可访问主机文件系统）
- AI 可执行 `KEYS *` 全量扫描导致阻塞

**建议**: 添加命令白名单，仅允许读命令：`GET`/`MGET`/`HGET`/`HGETALL`/`HKEYS`/`HVALS`/`LRANGE`/`ZSCORE`/`ZRANGE`/`ZCARD`/`SCARD`/`SMEMBERS`/`TYPE`/`TTL`/`EXISTS`/`LLEN`/`INFO`。禁止一切写命令和 `EVAL`/`FLUSHALL`/`CONFIG`/`KEYS`/`DEBUG`。

---

### #3 isReadOnlySQL 正则可绕过

**文件**: `internal/logic/mcp/mcp_tool_db.go:314-365`

**问题**: `isReadOnlySQL` 检查依赖关键词前后有空格的字符串匹配：

```
paddedSQL := " " + sql + " "
for _, keyword := range dangerousKeywords {
    if strings.Contains(paddedSQL, keyword) { // keyword 前后有空格
```

绕过方式：

1. 内联注释：`SEL/**/ECT` / `INS/**/ERT` — 关键词被注释分割后不含空格
2. 反引号包裹：`` `information_schema`.tables `` 不被 `INFORMATION_SCHEMA` 规则匹配
3. 零宽字符/unicode 等效字符可绕过字符串检测
4. 多语句虽被 `;` 检查拦截，但 MySQL 的 `HANDLER ... READ` 不含分号且不被拦截

**建议**: 在数据库连接层面设置只读模式，而非依赖字符串正则：

```go
// MySQL: 连接后立即设置只读事务
_, _ = db.Exec(ctx, "SET SESSION TRANSACTION READ ONLY")
// 或在 GoFrame 配置中设置 initSql
```

---

### #4 Chat channel 关闭路径不完整

**文件**: `internal/logic/ai/chat.go:31-211`

**问题**: `Chat` 函数中 `respChan` 的关闭路径不完整。多处错误退出只调用了 `cancel()` 但没有关闭 channel：

- 第 57-60 行：模型获取失败 → `cancel()` 但未 `close(respChan)`
- 第 90-93 行：MCP 工具获取失败 → 同上
- 第 112-114 行：Agent 创建失败 → 同上

SSE handler 侧如果 `respChan` 未被关闭，会导致消费端 goroutine 永久阻塞。

**建议**: 统一使用 `chanCloser` 模式（`chan_out.go` 已实现），在函数入口处 `defer closer.Close()` 确保所有退出路径都关闭 channel：

```go
closer := newChanCloser(respChan)
defer closer.Close()

// 所有错误退出路径只需：
cancel()
return  // closer.Close() 由 defer 保证
```

---

### #5 Stream 输出的 timer goroutine 泄漏

**文件**: `internal/logic/ai/chan_out.go:112-137`

**问题**:

```go
timeoutCh := time.After(30 * time.Second)
// ...
case res := <-chCh:
    timeoutCh = time.After(15 * time.Second) // 重置超时
```

`time.After` 每次调用都创建新 timer 并启动内部 goroutine，旧 timer 不会被取消，会持续运行直到过期后自行退出。在高频 chunk 场景下（如 AI 逐字流式输出），会产生大量泄漏的 timer goroutine。

**建议**: 改用 `time.NewTimer` + `Stop()`/`Reset()` 模式：

```go
timer := time.NewTimer(30 * time.Second)
defer timer.Stop()

for {
    select {
    case <-ctx.Done():
        // ...
    case <-timer.C:
        // 超时处理
    case res := <-chCh:
        if !timer.Stop() {
            <-timer.C
        }
        timer.Reset(15 * time.Second)
        // 处理 chunk
    }
}
```

---

### #20 案件分析全表扫描可致 OOM

**文件**: `internal/controller/ai_chat/ai_chat_v1_chats.go:653`

**问题**: `analyzeGridMajorCase` 函数中：

```go
query.OrderDesc("id").All()
```

在 `case_list` 或 `grid_case_record` 表上无 LIMIT、无时间范围过滤，将全表加载到 Go 内存中进行评分排序。当案件表达到数万行时会导致 OOM 或严重延迟。

**建议**:

1. 添加时间范围条件（如最近 30 天）
2. 添加 LIMIT（如 TOP 500）
3. 如需全量统计，改为 SQL 聚合查询而非全表加载到内存

---

## 三、P1 — 中危问题（应该修复）

### #6 MQTT 断线无指数退避

**文件**: `internal/logic/traffic/mqtt.go:53-134`

**问题**: Paho MQTT 客户端设了 `SetAutoReconnect(true)` 和固定 `SetConnectRetryInterval(5s)`，但无指数退避。如果 broker 长期不可用，会产生大量高频重连请求。

**建议**: 在 `SetConnectionLostHandler` 中记录断线次数，通过 `SetConnectRetryInterval` 动态调整间隔（如 5s → 10s → 20s → 最大 60s），连接成功后重置。

---

### #7 聚合刷新 DELETE+INSERT 非事务操作

**文件**: `internal/logic/traffic/aggregate_refresh.go`，`internal/logic/population/aggregate_refresh.go`

**问题**: 所有聚合刷新均采用先 DELETE 再 INSERT 模式，两者之间无事务保护：

```go
db.Exec(ctx, "DELETE FROM traffic_metric_hourly WHERE ...")
db.Exec(ctx, "INSERT INTO traffic_metric_hourly SELECT ...")
```

如果 INSERT 失败或服务中途重启，该时间段数据会被删除而新数据未写入，导致数据丢失。

**建议**: 用事务包裹：

```go
err := db.Transaction(ctx, func(tx *gdb.TX) error {
    if _, err := tx.Exec(ctx, "DELETE FROM ..."); err != nil {
        return err
    }
    _, err := tx.Exec(ctx, "INSERT INTO ... SELECT ...")
    return err
})
```

或使用 `INSERT ... ON DUPLICATE KEY UPDATE`（MySQL upsert）替代 DELETE+INSERT。

---

### #8 定时任务无并发保护

**文件**: `internal/logic/sync/scheduler.go`，`internal/logic/traffic/aggregate_refresh.go`，`internal/logic/population/aggregate_refresh.go`，`internal/logic/precipitate/precipitate.go`

**问题**: 多实例部署时，`RefreshAggregates`、`StartScheduler`、`StartCandidateScanner` 会同时在多个实例上运行，导致：

- 聚合数据重复计算（浪费资源）
- Scheduler 同步任务重复执行
- 候选扫描重复写入

**建议**: 使用分布式锁（Redis `SET NX EX`）或数据库行锁保护：

```go
locked, _ := g.Redis().SetNX(ctx, "lock:aggregate_refresh", "1", 10*time.Minute)
if !locked {
    return nil // 其他实例正在执行
}
defer g.Redis().Del(ctx, "lock:aggregate_refresh")
```

---

### #9 MQTT 消息逐条写入无批量

**文件**: `internal/logic/traffic/ingest.go:45-136`

**问题**: 每条 MQTT 消息都单独执行一次 `SaveGateRecord` + `WriteIngestLog` + `UpdateIngestStatus`。在高吞吐场景下（卡口每秒数百条），会产生大量独立 INSERT 和 UPDATE。

**建议**: 改为批量写入模式，使用 channel 收集消息，定时或攒够一定数量后批量 `INSERT ... VALUES (...), (...), ...`。

---

### #10 导出文件名路径遍历

**文件**: `internal/logic/mcp/mcp_tool_export.go:122-123`

**问题**:

```go
filePath := filepath.Join(exportDir, fileName+".xlsx")
```

如果 AI 传入 `fileName = "../../etc/something`，`filepath.Join` 不会阻止路径穿越（它只做字符串拼接，不检查 `..`）。

**建议**: 导出文件名应清洗或生成随机名，不能信任 AI 传入的 fileName：

```go
safeName := regexp.MustCompile(`[^\w\-.]`).ReplaceAllString(fileName, "_")
filePath := filepath.Join(exportDir, safeName+".xlsx")
// 或直接使用随机名
safeName := "export_" + grand.S(8, false)
```

---

### #11 JWT 可能使用空密钥签名

**文件**: `internal/logic/jwt/jwt.go:49`，`internal/consts/consts.go:27`

**问题**:

1. `PrivateKey string = ""` — 如果配置未正确加载，JWT 签名密钥为空字符串
2. 环境 `CHATDB_JWT_SECRET` 覆盖时，只覆盖 user subject（`consts.go:245-257`），admin subject 的密钥可能仍为空
3. 空 secret + HMAC-SHA256 仍可正常签名/验证，意味着任何人只要知道密钥为空就能伪造 token

**建议**:

- 启动时校验所有 subject 的 secret 非空，空则 panic 或拒绝启动
- user 和 admin JWT 必须使用不同密钥

---

### #12 全局可变状态无保护

**文件**: `internal/consts/consts.go:14-23`

**问题**:

```go
var (
    Ctx          = gctx.New()
    Logger       = g.Log()
    SystemConfig *gjson.Json
    Config       *model.ConfigData
)
var (
    McpClient *mcpclient.Client
)
var (
    PrivateKey string = ""
)
```

这些全局变量在 `init()` 中被赋值，但都可以被其他包修改（是 `var` 不是 `const`）。特别是 `McpClient` 和 `PrivateKey`，如果被意外置 nil 或覆盖，会导致运行时 panic。

**建议**: 使用 `sync.Once` + getter 方法暴露只读接口，或改为私有变量：

```go
var (
    mcpClient     *mcpclient.Client
    mcpClientOnce sync.Once
)

func McpClientInstance() *mcpclient.Client {
    mcpClientOnce.Do(func() { /* 初始化 */ })
    return mcpClient
}
```

---

### #13 空 UserGroup 结构体作 context key

**文件**: `internal/model/jwt.go:31-32`

**问题**:

```go
type UserGroup struct{}
```

空结构体作为 `context.WithValue` 的 key 在 Go 中合法，但缺少注释说明其用途。此外 `UserGroup` 是导出类型，任何包都可以用 `UserGroup{}` 读取/覆盖该 context 值。

**建议**: 改为不导出类型并添加注释：

```go
// userGroupKey is the context key for storing authenticated user ID.
type userGroupKey struct{}
```

---

### #21 多处管理接口无分页、无 LIMIT

| 文件:行 | 表 | 说明 |
|---------|-----|------|
| `controller/admin/admin_v1.go:945` | `user` | 全量加载所有用户，无分页 |
| `controller/admin/admin_v1.go:451` | `admin_grid_import` | 全量加载导入记录 |
| `controller/admin/admin_v1.go:668` | `admin_operation_log` | 无默认 LIMIT |
| `controller/admin/admin_v1.go:280` | `admin_data_source` | 全量加载 |
| `controller/ai_chat/ai_chat_v1_config.go:161` | `database_conf` | `Count()` 无 WHERE，暴露系统配置总数 |
| `controller/admin/admin_v1.go:1051,1073,1106` | `qa_knowledge_base` | 仅 `enabled=1` 过滤，无 LIMIT |
| `controller/admin/admin_v1.go:2024` | `admin_case_list` | page/pageSize 未校验，零值/负值导致负 SQL offset |

**建议**: 所有列表接口添加分页参数（page/pageSize），设置合理的默认值和上限（如 pageSize 默认 20，最大 200）。对 page/pageSize 做 `v:"min:1"` 校验。

---

### #22 upsert 竞态条件

**文件**: `internal/controller/ai_chat/ai_chat_v1_chats.go:539-572`

**问题**: `upsertAskNumberQuestionStat` 先 SELECT 检查记录是否存在，再决定 INSERT 或 UPDATE。两个并发请求可能同时看到无记录，都执行 INSERT，触发唯一键冲突返回错误。

**建议**: 改为单条 SQL：

```sql
INSERT INTO ask_number_question_stat (question_normalized, topic, hit_count, last_asked_at)
VALUES (?, ?, 1, ?)
ON DUPLICATE KEY UPDATE hit_count = hit_count + 1, last_asked_at = VALUES(last_asked_at)
```

---

### #23 DELETE + UPDATE 非事务操作

**文件**: `internal/controller/ai_chat/ai_chat_v1_chats.go:264-274`（ChatSessionReset）
**文件**: `internal/controller/ai_chat/ai_chat_v1_chats.go:436-451`（appendAskNumberMessage）

**问题**: 两个操作分别执行但不在同一事务中。如果第一个成功第二个失败，数据处于不一致状态。

**建议**: 用 `db.Transaction(ctx, func(tx *gdb.TX) error { ... })` 包裹。

---

### #24 LIKE 通配符注入

**文件**: `internal/controller/ai_chat/ai_chat_v1_chats.go:651`
**文件**: `internal/controller/admin/admin_v1.go:2030,2033,2036`

**问题**: 用户输入直接拼入 `%` + userInput + `%`，未转义 `%` 和 `_`。攻击者可输入 `%` 匹配所有行，或 `_` 匹配任意单字符，扩大查询范围。

**建议**: 转义用户输入中的 LIKE 通配符：

```go
func escapeLike(s string) string {
    s = strings.ReplaceAll(s, `\`, `\\`)
    s = strings.ReplaceAll(s, `%`, `\%`)
    s = strings.ReplaceAll(s, `_`, `\_`)
    return s
}

query.WhereLike("region", "%"+escapeLike(userInput)+"%")
```

---

### #25 错误被静默吞掉（扩展清单）

| 文件:行 | 操作 | 影响 |
|---------|------|------|
| `admin_v1.go:1077` | 知识库权限查询 `perms, _ :=` | 用户权限静默为空，无法感知异常 |
| `admin_v1.go:696-698` | DDL 迁移 `_, _ = db.Exec` | 磁盘满等严重错误被隐藏 |
| `admin_v1.go:51,130,149,191,223,312,446` | 审计日志插入 `_, _ = insertAdminLog` | 审计记录静默丢失，合规风险 |
| `user_v1_user.go:30-37` | 登录后更新 `_, _ =` | last_login_tme 更新失败无感知 |
| `user_v1_user.go:34-37,96-103` | 操作日志 `_, _ = insertAdminLog` | 同审计丢失 |
| `ai_chat_v1_chats.go:254-275` | ChatSessionReset DELETE 错误 | 数据残留，会话状态不一致 |
| `ai_chat_v1_chats.go:593` | suggestedQuestionsForUser 用 `context.Background()` | 请求取消不会传播，DB 错误静默空列表 |
| `admin_grid_import.go:125` | Rollback 审计日志 `_, _ =` | 回滚审计丢失 |
| `qa_v1.go:651` | SessionReset DELETE 错误 | 同 ChatSessionReset |
| `qa_v1.go:390+` | PopularQuestions DB 错误 | 静默空列表 |
| `qa_v1.go:1846` | batchKnowledgePermissions 循环内 DB 错误 | 部分权限静默丢失 |

**建议**: 关键路径（权限、审计、会话）的错误必须传播或至少记录日志。非关键路径（如审计日志）至少 `log.Warning`。

---

## 四、P2 — 低危/代码质量问题

### #14 Model 层大范围缺少标签

| 类别 | 涉及文件 | 数量 |
|------|---------|------|
| 输入结构体缺少 `v` 校验标签 | population.go, traffic.go, jwt.go, mcp.go | 15+ 字段 |
| 输入结构体缺少 `json` 标签 | population.go, traffic.go 的 Query 结构体 | 6 个结构体 |
| 结构体缺少 `dc` 描述标签 | 几乎所有 model 文件 | 80+ 字段 |
| 导出结构体缺少注释 | 几乎所有 model 文件 | 50+ 结构体 |
| `v:"required"` 缺少 `#消息` | metric.go:23-25 | 3 个字段 |

**影响**: 不影响运行时正确性，但降低代码可维护性和 API 文档自动生成质量。

---

### #15 interface{} 可替换为具体类型

| 文件:行 | 字段 | 当前类型 | 建议类型 |
|---------|------|---------|---------|
| model/ai.go:10 | `Parent` | `interface{}` | `*string` |
| model/ai.go:25 | `Group` | `interface{}` | `*string` |
| model/chat.go:35 | `Data` | `any` | 定义具体事件类型或 `map[string]any` |

---

### #16 MCP handler 的 panic recover 吞掉错误

**文件**: `internal/logic/mcp/handler.go:226-230`

```go
defer func() {
    if err := recover(); err != nil {
        consts.Logger.Printf(ctx, "panic error %+v", err)
    }
}()
```

recover 后外层 named return `err` 仍为 nil，调用方收不到任何错误信息，只得到空响应。

**建议**: recover 后设置 `err` 或返回错误文本：

```go
defer func() {
    if r := recover(); r != nil {
        consts.Logger.Printf(ctx, "panic error %+v", r)
        out = mcp.NewToolResultText("internal error: processing failed")
        err = nil
    }
}()
```

---

### #17 重复的表名去重逻辑

**文件**: `internal/logic/mcp/mcp_tool_db.go:373-383` 和 `mcp_tool_db.go:407-420`

`AddTable` 和 `addTableToContext` 都实现了相同的去重逻辑。两套表名累加机制并存（`tablesPtr` + `sessionTables`），增加维护负担。

**建议**: 统一为一套表名收集机制，删除 `chat.go` 中的 `tablesPtr` 逻辑，仅使用 context-based 方案。

---

### #18 TimestampToDateTime 创建了两次对象

**文件**: `internal/logic/mcp/mcp_tool_time.go:32-33`

```go
gtime.NewFromTimeStamp(gconv.Int64(timestamp)).Format("Y-m-d H:i:s")  // 无用
out = mcp.NewToolResultText(gjson.MustEncodeString(g.Map{
    "datetime": gtime.NewFromTimeStamp(gconv.Int64(timestamp)).Format("Y-m-d H:i:s"),
}))
```

第一行的结果未使用，第二行重复了相同操作。

**建议**: 删除第 32 行。

---

### #19 SyncHolidaysFromCode 忽略所有错误

**文件**: `internal/logic/traffic/aggregate_refresh.go:255-267`

```go
_, _ = db.Exec(ctx, `INSERT INTO traffic_holiday ...`)
```

所有 INSERT 都用 `_, _ =` 忽略错误。如果写入失败，节假日数据会静默丢失。

**建议**: 至少记录错误日志，或收集失败数并在完成后汇总报告。

---

## 五、Controller 层架构问题

### #26 严重业务逻辑泄漏至 Controller 层

**这是除安全问题外最大的架构隐患。** 在 10+ 个 Controller 文件中，仅 `traffic_v1.go` 正确地将全部逻辑委托给 service 层。其余 Controller 均包含大量直接 DB 访问、业务计算和 DDL 操作。

**典型泄漏清单**:

| Controller 文件 | 泄漏逻辑 | 严重度 |
|----------------|---------|--------|
| `admin_v1.go:32-56` | AdminLogin: 完整登录流程（密码验证、Token 生成、审计日志） | 高 |
| `admin_v1.go:115-195` | CreateAdminUser: 完整用户创建（密码哈希、DB 插入、权限位掩码构造） | 高 |
| `admin_v1.go:232-315` | UpdateAdminUser: 完整更新逻辑（权限位掩码构造、DB 更新） | 高 |
| `admin_v1.go:317-455` | ToggleUserEnabled: 启用禁用 + 审计 | 中 |
| `admin_v1.go:641-940` | AdminAidgpSync: 完整同步编排（外部 API 调用、分页、批量插入） | 高 |
| `admin_v1.go:1050-1500` | 合规检查、自动发现、文档管理：全部 DB 密集型逻辑 | 高 |
| `admin_v1.go:2024-2110` | AdminCaseList: 分页构造 + DB 查询 | 中 |
| `admin_v1.go:2111-2200` | AdminCaseStatistics: 多个聚合 SQL 查询 | 高 |
| `admin_v1.go:2200+` | DDL 函数: CreateAdminTables, MigrateAdminTables — 控制器内建表 | 极高 |
| `admin_grid_import.go:59-140` | AdminGridImportRollback: 完整回滚（DB 删除、状态更新、审计插入） — 必须在事务中 | 高 |
| `admin_metric.go:20-83` | CRUD 混合直接 DB 和 service.Metric() 调用（不一致委托） | 中 |
| `ai_chat_v1_chats.go:254-275` | ChatSessionReset: DB 删除操作 | 中 |
| `ai_chat_v1_chats.go:340-500` | GridMajorCaseAnalysis: 完整案件评分算法 + 格式化 | 极高 |
| `ai_chat_v1_chats.go:500-593` | 会话管理: DB 操作 + 会话状态管理 | 高 |
| `ai_chat_v1_chats.go:830+` | DDL: CreateAskNumberSessionTables — 控制器内建表 | 极高 |
| `ai_chat_v1_config.go:21-144` | 数据库配置 CRUD: 完整 DB 插入/更新/删除 | 高 |
| `user_v1_user.go:49-105` | UiapCallback: 完整 OAuth 流程（code 交换、用户信息获取、权限应用、JWT 生成） | 极高 |
| `user_v1_user.go:123-188` | UserPermissions: 权限快照装配 | 高 |
| `user_v1_user.go:411-462` | 位掩码到主题映射: 业务规则泄漏 | 高 |
| `qa_v1.go:128-226` | Chat (QA): 完整 SSE 流式聊天 + prompt 构建 | 极高 |
| `qa_v1.go:702-992` | streamQaChat: ~290 行 SSE 流式、prompt 构建、token 管理 | 极高 |
| `qa_v1.go:993-1114` | raw SQL 构建 + NLP/文本处理 + 评分算法 | 极高 |
| `qa_v1.go:2037-2350` | DDL: CreateQaTables, SeedQaTables — 控制器内建表 | 极高 |

**正面范例**: `internal/controller/traffic/traffic_v1.go`

- 全部 5 个 handler 方法只做参数转换 → `service.Traffic().Method()` → 返回结果
- 零 DB 访问、零业务逻辑、零直接响应操作

**建议**: 逐步将 Controller 中的 DB 操作和业务逻辑下移到 Logic/Service 层，Controller 只保留参数绑定、校验和响应格式化。优先处理标记为"极高"的项目，特别是 DDL 和 OAuth 回调。

---

### #27 Controller 输入校验缺失汇总

| Controller 方法 | 缺失校验 | 风险 |
|----------------|---------|------|
| AdminCaseList | page/pageSize 未校验 min:1 | 负 SQL offset |
| AdminCaseStatistics | month 格式未校验 | 无效查询 |
| AdminGridImportDetail | req.Id 未校验 >0 | 查询异常 |
| AdminAidgpConnectionTest | req 字段未校验 | 空指针或无效连接 |
| SetDataBaseConfig / UpdateDataBaseConfig | port 未校验 1-65535 | 无效端口 |
| ChatSessionReset | sessionId 未校验非空 | 误删其他会话 |
| SpeechTranscribe | audio 大小未限制 | 大文件上传耗尽内存 |
| UserPopularQuestions | limit 无上限 | 超量查询 |

---

## 六、问题统计总表

| 编号 | 优先级 | 类别 | 问题 | 文件 |
|------|--------|------|------|------|
| #1 | P0 | 安全 | Shell 工具黑名单可绕过 | mcp_tool_shell.go |
| #2 | P0 | 安全 | Redis 命令无安全限制 | mcp_tool_redis.go |
| #3 | P0 | 安全 | isReadOnlySQL 正则可绕过 | mcp_tool_db.go |
| #4 | P0 | 资源泄漏 | Chat channel 关闭路径不完整 | chat.go |
| #5 | P0 | 资源泄漏 | Stream timer goroutine 泄漏 | chan_out.go |
| #20 | P0 | 性能/安全 | 案件分析全表扫描 OOM | ai_chat_v1_chats.go |
| #6 | P1 | 可靠性 | MQTT 无指数退避 | mqtt.go |
| #7 | P1 | 数据一致性 | 聚合 DELETE+INSERT 非事务 | aggregate_refresh.go |
| #8 | P1 | 并发 | 定时任务无并发锁 | scheduler.go, precipitate.go |
| #9 | P1 | 性能 | MQTT 消息逐条写入 | ingest.go |
| #10 | P1 | 安全 | 导出文件名路径遍历 | mcp_tool_export.go |
| #11 | P1 | 安全 | JWT 可能空密钥签名 | jwt.go, consts.go |
| #12 | P1 | 安全 | 全局可变状态无保护 | consts.go |
| #13 | P1 | 安全 | 空 UserGroup 作 context key | jwt.go |
| #21 | P1 | 性能 | 多处管理接口无分页 | admin_v1.go 多处 |
| #22 | P1 | 数据一致性 | upsert 竞态条件 | ai_chat_v1_chats.go:539 |
| #23 | P1 | 数据一致性 | DELETE+UPDATE 非事务 | ai_chat_v1_chats.go:264,436 |
| #24 | P1 | 安全 | LIKE 通配符注入 | admin_v1.go, ai_chat_v1_chats.go |
| #25 | P1 | 可靠性 | 错误静默吞掉 | 多文件，20+ 处 |
| #26 | P1 | 架构 | Controller 业务逻辑严重泄漏 | 几乎所有 Controller |
| #27 | P1 | 校验 | Controller 输入校验缺失 | 多文件，8+ 处 |
| #14 | P2 | 代码质量 | Model 标签缺失 | model/* 多文件 |
| #15 | P2 | 代码质量 | interface{} 可具体化 | ai.go, chat.go |
| #16 | P2 | 可靠性 | panic recover 吞掉错误 | handler.go |
| #17 | P2 | 代码质量 | 重复表名去重逻辑 | mcp_tool_db.go |
| #18 | P2 | 代码质量 | 死代码（重复创建对象） | mcp_tool_time.go |
| #19 | P2 | 可靠性 | Holiday 同步忽略错误 | aggregate_refresh.go |

---

## 七、建议修复顺序与排期

### 阶段 1 — 安全边界加固（1-2 天）

```
├── #1  Shell 工具改为白名单或移除
├── #2  Redis 工具添加命令白名单
├── #3  SQL 安全改为连接层只读
└── #11 JWT 空密钥启动校验
```

### 阶段 2 — 资源泄漏修复（1 天）

```
├── #4  Chat channel 统一 defer close
└── #5  Stream timer 改用 NewTimer + Reset
```

### 阶段 3 — 数据一致性（2-3 天）

```
├── #7  聚合刷新改为事务或 upsert
├── #22 upsert 改为单条 SQL
├── #23 DELETE+UPDATE 包裹事务
└── #25 关键查询错误不再吞掉
```

### 阶段 4 — 并发与性能（2-3 天）

```
├── #8  定时任务添加分布式锁
├── #9  MQTT 改为批量写入
├── #20 案件分析添加 LIMIT 和时间范围
├── #21 管理接口添加分页
└── #27 Controller 输入校验补全
```

### 阶段 5 — 安全加固（1-2 天）

```
├── #6  MQTT 指数退避
├── #10 导出文件名清洗
├── #12 全局变量改为受控访问
├── #13 context key 改为不导出类型
└── #24 LIKE 通配符转义
```

### 阶段 6 — 架构改善（持续，按模块拆分）

```
├── #26 将 Controller 中 DB 操作和业务逻辑下移到 Logic/Service 层
│   ├── 优先级 1: DDL 操作移出 Controller
│   ├── 优先级 2: OAuth 回调移到 Logic
│   ├── 优先级 3: 案件评分算法移到 Logic
│   └── 优先级 4: 其他 DB 密集型逻辑逐步下移
├── #14-19 标签补全、死代码清理、重复逻辑合并
```

---

## 附录：审查方法论

### 人工 Review 流程

1. **分层推进**：Model → DAO → Logic → Controller → Security 逐层审查
2. **优先级排序**：P0 安全 → P1 一致性/并发 → P2 质量/架构
3. **交叉验证**：对关键路径画 goroutine 生命周期图，对 MCP SQL 工具做边界测试

### 建议 CI 自动化检查

| 工具 | 用途 | 阈值 |
|------|------|------|
| `golangci-lint` | 静态分析 | 零 error |
| `gosec` | 安全扫描 | 零 warning |
| `go test -race -cover` | 竞态 + 覆盖率 | logic 层 ≥ 50% |
| `gocyclo` | 圈复杂度 | ≤ 15 |
| `go vet` | 编译器级检查 | 零 warning |
