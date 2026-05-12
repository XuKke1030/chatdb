# 项目无用文件与死代码检查报告

检查日期：2026-05-12  
检查范围：`D:\code\ChatDB-master`  
验证命令：`go list ./...`、`go list -json ai-chat-sql`、`go test ./...`、源码引用扫描

## 1. 可以直接清理的本地产物

这些文件不参与源码构建，属于本地运行或构建产物。`.gitignore` 已经配置忽略 `*.exe`、`*.log`、`.idea/`、`.claude/`、`*.key`，如果仍在工作区显示，通常是历史遗留或本地文件。

| 路径/模式 | 类型 | 处理建议 |
|---|---|---|
| `*.log` | 后端运行日志 | 可删除；保留最近一次排错需要的日志即可 |
| `chatdb.exe` | 编译产物 | 可删除；由 `go build` 重新生成 |
| `chatdb_new.exe` | 编译产物 | 可删除；疑似临时版本 |
| `chatdb_test.exe` | 编译产物 | 可删除；疑似测试构建 |
| `bin/chatdb-backend.exe` | 编译产物 | 可删除或改为发布目录产物，不建议入库 |
| `bin/chatdb-current.exe` | 编译产物 | 可删除或改为发布目录产物，不建议入库 |
| `bin/chatdb-dev.exe` | 编译产物 | 可删除或改为本地构建产物 |
| `.idea/` | JetBrains IDE 配置 | 不建议入库；本地可保留，仓库应忽略 |
| `.claude/` | 本地助手配置 | 不建议入库；本地可保留，仓库应忽略 |
| `private.key` | 本地私钥/运行密钥 | 不建议入库；生产必须用环境变量或密钥系统 |

## 2. 不进入主程序运行时的 Go 包

通过 `go list -json ai-chat-sql` 对主程序依赖图检查，以下本地包不在主程序运行时依赖中：

| 包 | 说明 | 处理建议 |
|---|---|---|
| `ai-chat-sql/internal/model/do` | GoFrame 生成的 DO 模型，目前主程序未引用 | 谨慎处理；如果不再使用 GoFrame DAO 生成模板，可考虑删除 |
| `ai-chat-sql/test` | 测试包 | 保留 |
| `ai-chat-sql/tools` | 手工工具脚本包 | 保留或迁移到 `cmd/tools`；不属于服务运行时 |
| `ai-chat-sql/tools/perfcheck` | 性能检查工具 | 保留；不属于服务运行时 |

## 3. 疑似死代码或未对外暴露的代码

### 3.1 后台日志接口未挂到 API interface

发现：

- `internal/controller/admin/admin_v1.go` 实现了 `AdminLogs`
- `api/admin/v1/admin.go` 定义了 `AdminLogsReq`、`AdminLogsRes`、`AdminLogItem`
- 但 `api/admin/admin.go` 的 `IAdminV1` interface 没有声明 `AdminLogs`

影响：

- `internal/cmd/cmd.go` 中通过 `Bind(admin.NewV1())` 绑定的是 `admin.IAdminV1`。
- `AdminLogs` 没在 interface 里，实际不会作为接口暴露。
- 这是“实现存在但路由不可达”的高可信死路由。

处理建议：

- 如果后台需要操作日志接口：在 `api/admin/admin.go` 的 `IAdminV1` 增加：

```go
AdminLogs(ctx context.Context, req *v1.AdminLogsReq) (res *v1.AdminLogsRes, err error)
```

- 如果后台不需要该接口：删除 `AdminLogs`、`scanAdminLogs`、`AdminLogsReq/Res/Item`。

### 3.2 问答旧同步占位函数未被调用

发现：

- `internal/controller/qa/qa_v1.go` 中存在 `createQaSyncPlaceholder`
- `qaSyncPlaceholderMessage` 只被 `createQaSyncPlaceholder` 调用
- 当前同步入口调用的是 `executeQaSync`

影响：

- 这组函数属于旧的“占位同步”实现，当前路径已经切换到真实任务表/日志表流程。

处理建议：

- 可删除 `createQaSyncPlaceholder` 和 `qaSyncPlaceholderMessage`。
- 删除后跑 `go test ./...`。

### 3.3 空壳 controller 文件

发现以下文件只有 `package` 声明或自动生成注释，没有实际逻辑：

| 文件 | 说明 |
|---|---|
| `internal/controller/ai_chat/ai_chat.go` | 空壳 |
| `internal/controller/qa/qa.go` | 空壳 |
| `internal/controller/traffic/traffic.go` | 空壳 |
| `internal/controller/user/user.go` | 空壳 |

处理建议：

- 可以保留：GoFrame 模板生成常见结构，后续生成代码可能依赖这个布局。
- 如果追求仓库整洁，也可以删除；删除前确认 GoFrame 生成脚本不会重新创建或依赖这些文件。

### 3.4 build ignore 工具文件

发现：

- `check_user.go`
- `init_db.go`

这两个文件带 `//go:build ignore`，不会进入正常构建。

处理建议：

- 如果只是一次性调试脚本，可以迁移到 `tools/legacy/` 或删除。
- 如果仍要保留，建议修复中文乱码并写明运行方式，例如 `go run check_user.go`。

### 3.5 `model.PromptGetListOutputItem.GetContent`

发现：

- `internal/model/prompt.go` 的 `GetContent` 当前只在 `internal/logic/ai/chat.go` 使用。

结论：

- 不是死代码。虽然引用少，但属于主路径 Prompt 格式化方法，应保留。

### 3.6 `internal/model/config.go` 的 `IsDebug`

发现：

- `ConfigData.IsDebug` 没有被项目代码调用。

处理建议：

- 低风险可删除。
- 也可以保留作为配置辅助方法；不影响运行。

## 4. 不建议删除的“看似没直接引用”的代码

以下代码通过接口、反射、注册或 GoFrame Bind 间接使用，不能只按文本引用判断：

| 代码 | 原因 |
|---|---|
| `internal/controller/*/*_v1.go` 中大多数 controller 方法 | GoFrame 通过 interface + `Bind` 暴露路由 |
| `internal/logic/*` 的 `init()` 注册 | `main.go` 空导入 `internal/logic`，触发服务注册 |
| `internal/logic/ai/mcp_cache.go` 的 `InvokableRun` | 实现 Eino tool interface，属于接口调用 |
| `internal/service/*` 的 Register/Get 方法 | 服务定位器模式，间接引用 |
| `api/*/v1/*.go` request/response 类型 | GoFrame 路由和 OpenAPI 元数据依赖 |
| `prompt/*.md` | 问数 Prompt 运行时加载 |
| `manifest/` | 部署资源，运行时不加载但部署需要 |
| `docs/` | 项目交付文档，运行时不加载但不属于死代码 |

## 5. 当前验证结果

已执行：

```bash
go test ./...
```

结果：通过。

说明：

- 首次 `go list`/`go test` 在沙箱内访问 Go 构建缓存会失败；提权后通过。
- 目前没有因为“未使用变量/未使用 import”导致编译失败的硬性死代码。
- 本报告中的死代码主要是“路由不可达/主程序不依赖/旧占位逻辑”层面的判断。

## 6. 建议清理顺序

1. 先清理本地产物：`*.log`、`*.exe`、`bin/*.exe`。
2. 确认 `private.key` 不入库，生产改环境变量。
3. 修复或删除 `check_user.go`、`init_db.go`。
4. 决定 `AdminLogs` 是补 interface 暴露，还是删除实现。
5. 删除 `createQaSyncPlaceholder`、`qaSyncPlaceholderMessage`。
6. 评估是否删除空壳 controller 文件。
7. 最后再考虑 `internal/model/do` 是否随着 DAO 生成链路保留。

