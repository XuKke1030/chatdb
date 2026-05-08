# ChatDB 移除 SQLite 支持优化方案

> 审查日期：2026-05-08  
> 前提：项目不再需要 SQLite 支持，仅保留 MySQL（及已有的 PostgreSQL）

---

## 一、需删除的文件

### 1.1 SQLite 驱动依赖

| 文件/依赖 | 说明 |
|---|---|
| `go.mod` 中 `github.com/gogf/gf/contrib/drivers/sqlite/v2` | SQLite 驱动直接依赖，移除后执行 `go mod tidy` |
| `go.mod` 中 `github.com/glebarez/go-sqlite` | SQLite 驱动的间接依赖（transitive），`go mod tidy` 会自动清理 |
| `go.mod` 中 `modernc.org/sqlite` 及相关 `modernc.org/*` | 纯 Go SQLite 实现的间接依赖，`go mod tidy` 会自动清理 |

### 1.2 SQLite 相关 DAO/Entity

| 文件 | 说明 |
|---|---|
| `internal/dao/sqlite_sequence.go` | SQLite 系统表 `sqlite_sequence` 的 DAO，**不应为系统表生成业务 DAO** |
| `internal/dao/internal/sqlite_sequence.go` | 同上，内部实现 |
| `internal/model/do/sqlite_sequence.go` | 同上，DO 结构体 |
| `internal/model/entity/sqlite_sequence.go` | 同上，Entity 结构体 |

> 这些文件本身就是上轮审查中标记为"零引用"的死代码，移除 SQLite 后更应删除。

### 1.3 SQLite 数据库文件

| 文件 | 说明 |
|---|---|
| `data.db` | 项目根目录下的 SQLite 数据库文件，不应纳入版本控制。删除后需在 `.gitignore` 中添加 `*.db` |

---

## 二、需修改的 Go 文件（共 7 个文件，6 处 `case "sqlite"` 分支 + 1 处驱动导入）

### 2.1 驱动导入 — `internal/packed/packed.go`

```go
// 删除此行：
_ "github.com/gogf/gf/contrib/drivers/sqlite/v2"
```

仅保留 MySQL 驱动导入。

### 2.2 建表 SQL 双版本合并 — 6 处 `case "sqlite"` 分支

以下 6 个位置均维护了 SQLite 和 MySQL 两套 DDL，移除 SQLite 后应：
1. **删除** `case "sqlite":` 分支及其对应的 SQL 常量
2. **删除** `dbType` 检测逻辑（`db.GetConfig().Type`）
3. **直接使用** MySQL DDL（或将 MySQL DDL 提取为唯一版本）

#### ① `internal/logic/traffic/traffic.go`

**当前逻辑**：
```go
func trafficTableSQL(dbType string) []string {
    if dbType == "sqlite" {
        return []string{sqliteRecordSQL, sqliteDeviceSQL, sqliteLogSQL, sqliteStatusSQL}
    }
    return []string{mysqlRecordSQL, mysqlDeviceSQL, mysqlLogSQL, mysqlStatusSQL}
}
```

**优化后**：删除 `trafficTableSQL()` 函数、`dbType()` 函数，直接在 `InitTables()` 中使用 MySQL SQL 常量。删除 4 个 `sqlite*SQL` 常量（约 **80 行**）。

#### ② `internal/logic/alert/alert.go`

**当前逻辑**：
```go
switch dbType {
case "sqlite":
    alertTableSQL = `CREATE TABLE IF NOT EXISTS alert_event (id INTEGER PRIMARY KEY AUTOINCREMENT, ...)`
    userStateTableSQL = `CREATE TABLE IF NOT EXISTS alert_user_state (id INTEGER PRIMARY KEY AUTOINCREMENT, ...)`
default:
    alertTableSQL = `CREATE TABLE IF NOT EXISTS alert_event (id INT PRIMARY KEY AUTO_INCREMENT, ...)`
    userStateTableSQL = `CREATE TABLE IF NOT EXISTS alert_user_state (id INT PRIMARY KEY AUTO_INCREMENT, ...)`
}
```

**优化后**：删除 switch 和 `dbType` 检测，直接使用 MySQL DDL。删除 SQLite 分支（约 **25 行**）。

#### ③ `internal/controller/ai_chat/ai_chat_v1_chats.go`

**当前逻辑**：
```go
switch dbType {
case "sqlite":
    sessionSQL = `CREATE TABLE IF NOT EXISTS ask_number_session (session_id TEXT PRIMARY KEY, ...)`
    messageSQL = `CREATE TABLE IF NOT EXISTS ask_number_message (id INTEGER PRIMARY KEY AUTOINCREMENT, ...)`
default:
    sessionSQL = `CREATE TABLE IF NOT EXISTS ask_number_session (session_id VARCHAR(64) PRIMARY KEY, ...)`
    messageSQL = `CREATE TABLE IF NOT EXISTS ask_number_message (id BIGINT PRIMARY KEY AUTO_INCREMENT, ...)`
}
```

**优化后**：删除 switch 和 `dbType` 检测，直接使用 MySQL DDL。删除 SQLite 分支（约 **20 行**）。

#### ④ `internal/controller/qa/qa_v1.go`

**当前逻辑**：
```go
switch dbType {
case "sqlite":
    sqlList = sqliteQaTableSQL()
default:
    sqlList = mysqlQaTableSQL()
}
```

**优化后**：删除 `sqliteQaTableSQL()` 函数（约 **80 行**），删除 switch 和 `dbType` 检测，直接调用 `mysqlQaTableSQL()`。

#### ⑤ `internal/controller/admin/admin_v1.go`

**当前逻辑**：
```go
func createAdminTables(ctx context.Context, db gdb.DB) error {
    dbType := ""
    if cfg := db.GetConfig(); cfg != nil {
        dbType = cfg.Type
    }
    sqls := sqliteAdminTableSQL()
    if dbType != "sqlite" {
        sqls = mysqlAdminTableSQL()
    }
    ...
}
```

以及 `migrateAdminTables()` 中：
```go
func migrateAdminTables(ctx context.Context, db gdb.DB, dbType string) {
    if dbType == "sqlite" {
        _, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN display_name TEXT")
        _, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN last_login_at INTEGER NOT NULL DEFAULT 0")
        return
    }
    _, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN display_name VARCHAR(64)")
    _, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN last_login_at INT NOT NULL DEFAULT 0")
}
```

以及 `ensureAdminTables()` 中：
```go
if dbType != "sqlite" {
    _, _ = db.Exec(ctx, "ALTER TABLE admin_grid_import_error MODIFY COLUMN raw_data LONGTEXT")
}
```

**优化后**：
1. 删除 `sqliteAdminTableSQL()` 函数（约 **60 行**）
2. 删除 `createAdminTables()` 中的 `dbType` 检测，直接调用 `mysqlAdminTableSQL()`
3. 删除 `migrateAdminTables()` 中的 SQLite 分支
4. 删除 `ensureAdminTables()` 中的 `dbType != "sqlite"` 判断，直接执行 `MODIFY COLUMN`

### 2.3 MCP 工具中的 SQLite 支持 — `internal/logic/mcp/mcp_tool_db.go`

**当前逻辑**（2 处）：

```go
// getVersionQuery
case "sqlite":
    return "SELECT sqlite_version() as version"

// getSizeQuery
case "sqlite":
    return "SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size()"
```

以及 `isReadOnlySQL()` 中的 PRAGMA 检查：
```go
// 检查是否为PRAGMA语句（SQLite的PRAGMA语句通常是只读的）
if strings.HasPrefix(sql, "PRAGMA") {
    return true
}
```

**优化后**：
1. 删除 `getVersionQuery()` 中的 `case "sqlite"` 分支
2. 删除 `getSizeQuery()` 中的 `case "sqlite"` 分支
3. 删除 `isReadOnlySQL()` 中的 PRAGMA 检查及注释

### 2.4 数据库连接生成 — `internal/logic/config/gen.go`

**当前逻辑**：
```go
case "sqlite":
    link = fmt.Sprintf("sqlite::@file(%s)", config.DbName)
```

**优化后**：删除此 case 分支。如未来需要支持用户配置外部 SQLite 数据源（作为查询目标），可保留但建议改为显式配置而非默认支持。

### 2.5 模型注释 — `internal/model/db_config.go`

**当前**：
```go
DbType string `json:"dbType" dc:"数据库类型，如: mysql, postgres, sqlite"`
```

**优化后**：
```go
DbType string `json:"dbType" dc:"数据库类型，如: mysql, postgres"`
```

### 2.6 API 验证层 — `internal/controller/ai_chat/ai_chat_v1_config.go`

**当前**（2 处）：
```go
supportedTypes := map[string]bool{
    "mysql":    true,
    "postgres": true,
    "sqlite":   true,
}
```

**优化后**：从 supportedTypes 中移除 `"sqlite": true`。

---

## 三、SQL 初始化脚本

### 3.1 `docs/admin-platform-init.sql`（SQLite 版）

此文件包含 SQLite 语法的建表和插入语句（`INTEGER PRIMARY KEY AUTOINCREMENT`、`CURRENT_TIMESTAMP` 等）。如果项目仅使用 MySQL，此文件已过时。

**建议**：`docs/admin-platform-init.mysql.sql` 已存在 MySQL 版本，删除 SQLite 版的 `admin-platform-init.sql`，或在其顶部标注"仅用于 SQLite，已废弃"。

---

## 四、环境配置

### 4.1 `.env.example`

```
CHATDB_TRAFFIC_MQTT_PASSWORD=
```

无直接 SQLite 引用，无需修改。

### 4.2 `config.yaml`

当前 `database.master.link` 已配置为 MySQL，无需修改。

---

## 五、操作步骤（建议执行顺序）

| 步骤 | 操作 | 影响范围 |
|---|---|---|
| 1 | 删除 `internal/packed/packed.go` 中的 SQLite 驱动 import | 编译 |
| 2 | 删除 6 处 `case "sqlite"` 分支及相关 SQL 常量/函数 | 7 个文件 |
| 3 | 删除 `internal/logic/mcp/mcp_tool_db.go` 中 SQLite 查询和 PRAGMA 逻辑 | 1 个文件 |
| 4 | 删除 `internal/logic/config/gen.go` 中 SQLite 连接生成 | 1 个文件 |
| 5 | 修改 `ai_chat_v1_config.go` 中 supportedTypes | 1 个文件 |
| 6 | 修改 `db_config.go` 模型注释 | 1 个文件 |
| 7 | 删除 `internal/dao/sqlite_sequence.go`、`internal/dao/internal/sqlite_sequence.go`、`internal/model/do/sqlite_sequence.go`、`internal/model/entity/sqlite_sequence.go` | 4 个文件 |
| 8 | 删除 `data.db`，在 `.gitignore` 添加 `*.db` | 1 个文件 + .gitignore |
| 9 | 执行 `go mod tidy` 清理依赖 | go.mod / go.sum |
| 10 | 编译验证 + 运行测试 | 全项目 |

---

## 六、预估可删除行数

| 来源 | 行数 |
|---|---|
| 4 个 `sqlite*SQL` 常量块（traffic） | ~80 |
| `sqliteAdminTableSQL()` 函数 | ~60 |
| `sqliteQaTableSQL()` 函数 | ~80 |
| 6 处 `case "sqlite"` 分支 + `dbType` 检测逻辑 | ~50 |
| MCP 工具中 SQLite 查询 + PRAGMA | ~10 |
| `config/gen.go` 中 SQLite 连接生成 | ~3 |
| DAO/DO/Entity 文件（sqlite_sequence） | ~60 |
| `data.db` | 二进制文件 |
| **合计** | **~343+ 行 + 1 二进制文件** |

---

## 七、附加收益

移除 SQLite 后，还可以顺带完成以下简化：

1. **统一建表入口**：当前每个模块自行检测 `dbType` 再选 SQL，移除后可直接硬编码 MySQL DDL，为后续"建表逻辑收拢到统一初始化模块"扫清障碍
2. **减少二进制体积**：`modernc.org/sqlite` 是纯 Go 实现的 SQLite，体积较大（~20MB+），移除后编译产物显著缩小
3. **消除 `sqlite_sequence` DAO 的生成风险**：GoFrame 的 `gf gen dao` 会扫描 SQLite 系统表，移除 SQLite 数据库后不再有此问题
