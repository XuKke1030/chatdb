# 问答问数（移动端）功能完成情况检查报告

检查日期：2026-05-12（初次）、2026-05-12（迭代一&二更新）
检查范围：`D:\code\ChatDB-master` 后端项目、桌面功能清单《问答问数（移动端）功能清单.xlsx》
验证方式：代码静态检查、接口定义核对、关键链路追踪、`go test ./...`

## 1. 总体结论

当前项目后端已经具备"问数/问答/后台管理"的主干接口与部分数据能力，但还没有达到完整上线状态。

已基本可用的能力集中在：

- 用户登录、JWT 鉴权、主题权限、知识库权限。
- 问数 SSE 会话、上下文保存、新问题重置。
- 问数网格重大案件 Top1 分析。
- 车流 MQTT 本地接入、明细落库、聚合统计、车牌归属识别、接入状态查询。
- 问答知识库列表、本地关键词 RAG、SSE 回答、引用详情、原文查看、热门问题、会话重置。
- 后台用户权限、常用问题、问题候选、数据源状态、网格数据上传、AIDGP 同步任务日志。
- 快路径问数缓存，支持 Redis 优先、内存兜底。
- **【新增】模糊问题澄清机制**：后端流式状态机检测 `chatdb-clarify` 格式，输出结构化 `clarification` SSE 事件；Prompt 增加 3 个澄清示例。
- **【新增】问数 SSE 错误事件**：流读取异常时发送 `error` 事件，前端可感知。
- **【新增】参数提取引擎**：从用户问题中提取时间范围、卡口名称、区域名称、对比基准，影响缓存 key 和 SQL 查询过滤。
- **【新增】快路径意图扩展**：从 14 个扩展到 18 个，新增驻留时长 Top、按小时人流趋势、案件类型分布。
- **【新增】车流查询参数化**：`TrafficAggregateQuery` 支持 GateName/PlateRegion 过滤，"拱北口岸车流"等带参数问题不再走全局查询。

主要未完成或不具备上线质量的能力：

- AIDGP 真实对接没有实现，`internal/logic/aidgp/client.go` 当前仍返回 Mock 数据；网格、人流、车流并未实时去 AIDGP 查询。
- 人流没有正式数据模型、同步字段、落库表和真实查询接口，只存在动态探测常见表名的快路径兜底。
- 网格数据主要依赖本地 `case_list` 和后台上传，缺少 AIDGP 增量同步、指标库、月度覆盖规则的完整生产闭环。
- 问答文档解析、附件解析、版本有效性、管理/执行文件图谱、AIDGP 知识库同步均未真实实现。
- 联网搜索只是配置占位，不会真正发起外部搜索。
- 移动端前端不在本仓库内，输入 100 字限制、按钮停止、图表复制、图表切换、澄清选项交互、洞察展开/收起等前端验收项只能在前端仓库另行核验。
- 代码中存在大量中文注释、错误信息、接口描述显示为乱码的现象，编译不受影响，但上线前必须治理。
- `config/config.yaml` 中存在明文密钥样例，必须迁移到环境变量或密钥管理系统。

## 2. 功能清单逐项完成度

状态定义：

- 已完成：后端已有接口/逻辑，且能通过编译测试。
- 部分完成：有主干能力，但缺少真实数据、外部对接、前端闭环或上线级细节。
- 未完成：仅有规划、占位或没有发现实现。
- 不在本仓库：主要为移动端/管理端前端体验，需要到前端仓库验收。

| 序号 | 模块 | 功能点 | 状态 | 代码/证据 | 主要差距 |
|---:|---|---|---|---|---|
| 1 | 问数/主题选择 | 主题入口展示、无权限隐藏 | 部分完成 | `api/user/v1/user.go`、`internal/controller/user/user_v1_user.go` | 后端能返回权限主题；移动端入口展示需前端验证 |
| 2 | 问数/主题选择 | 实时异常告警 | 部分完成 | `api/ai_chat/v1/alert.go`、`internal/logic/alert/alert.go`、用户 bootstrap 返回 `alertSummary` | 告警规则与推送链路不完整；人流/车流持续异常依赖真实数据 |
| 3 | 问数/主题选择 | 主题进入逻辑 | 不在本仓库 | 后端支持 `topic` 参数和权限校验 | 前端跳转携带主题需前端仓库验证 |
| 4 | 问数/网格 | 重大案件分析 | 已完成 | `GridMajorCaseAnalysis`、`streamGridMajorCaseAnswer` | 评分依据为本地规则，需业务确认权重；依赖网格数据导入质量 |
| 5 | 问数/人流 | 人流进出趋势、节假日对比 | 部分完成 | `fastPopWeekTrendQuery`、`fastPopHolidayCompareQuery`、**【新】`fastPopHourlyTrendQuery`** | 无正式人流表结构；节假日对比当前是简化环比；按小时趋势已实现但依赖真实数据表 |
| 6 | 问数/人流 | 流动人口识别 | 未完成 | 未发现正式模型或接口 | 需 AIDGP 提供户籍地、运营商驻留地、统计口径或脱敏聚合接口 |
| 7 | 问数/车流 | 车流数据处理与预聚合 | 部分完成→**改善** | `traffic_gate_record`、`Aggregate`、快路径统计、**【新】参数化查询（GateName/PlateRegion过滤）** | 预聚合表仍未实现；**【新】驻留时长Top快路径已实现**；节假日参数在接口中预留但聚合逻辑未使用 |
| 8 | 问数/车流 | 车牌识别 | 已完成 | `internal/logic/traffic/plate.go`、`/traffic/plate/recognize` | 省份/城市中文乱码风险；识别规则需补充新能源、特殊号牌、纯港澳牌边界用例 |
| 9 | 问数/输入 | 文字输入、回车发送、停止回答 | 不在本仓库 | 后端 `/chats` SSE | 输入限制、发送/停止按钮属于前端 |
| 10 | 问数/输入 | 语音提问 | 部分完成 | `/speech/transcribe` | 需确认 ASR 配置和真实音频验收；静默超时属于前端 |
| 11 | 问数/结果 | 模糊问题澄清 | **未完成→部分完成** | **【新】`model.ClarificationData`、`chan_out.go` 流式状态机、`clarification` SSE 事件、Prompt 3个示例** | 后端已能检测并输出结构化澄清事件；前端澄清选项交互需前端仓库实现 |
| 12 | 问数/结果 | 三级结构回答、智能图表 | 部分完成→**改善** | `prompt/main.md`、快路径 `formatFastPathAnswer`、`chatdb-chart`、**【新】快路径意图 14→18，新增驻留时长(bar)、按小时趋势(line)、案件类型分布(pie)** | 大模型路径依赖 Prompt，不保证稳定；**【新】快路径图表类型覆盖line/bar/pie/radar/map**；图表切换为前端能力 |
| 13 | 问数/结果 | 洞察分析展开/收起 | 部分完成 | 快路径有 `InsightCollapsedDefault` 元数据 | 大模型路径无统一结构化字段；前端需实现 |
| 14 | 问数/结果 | 回答与图表复制 | 不在本仓库 | 无后端要求 | 前端实现与验收 |
| 15 | 问数/对话 | 上下文追问 | 已完成 | `ask_number_session`、`ask_number_message`、`mergeChatHistory` | 可补会话列表接口 |
| 16 | 问数/对话 | 新问题开启 | 已完成 | `/chats/sessions/{sessionId}/reset` | 前端按钮需验证 |
| 17 | 问数/支撑 | AI 聚合与指标库 | 部分完成→**改善** | 快路径聚合、MCP SQL 查询、**【新】参数提取引擎（时间/卡口/区域/对比基准）** | 没有指标库模型、指标沉淀、批计算任务；**【新】参数提取使同意图不同参数问题产生独立缓存和差异化查询** |
| 18 | 问数/支撑 | 用户主题权限 | 已完成 | rule_level bit：grid=1、population=2、traffic=4 | 权限实时生效依赖前端刷新/轮询 bootstrap |
| 19 | 问数/支撑 | 人流/车流实时接入 | 部分完成 | 车流 MQTT 完成；人流未完成 | 人流每日同步、AIDGP 取数未完成 |
| 20 | 问数/支撑 | 网格月度导入 | 部分完成 | `/admin/grid-data/upload` 支持 xlsx/xls/csv | 模板校验、覆盖策略、错误提示中文乱码、前端上传需验收 |
| 21 | 问答/初始化 | 欢迎引导 | 不在本仓库 | 后端 bootstrap 提供上下文 | 前端展示 |
| 22 | 问答/初始化 | 热门问答 3 条 | 已完成 | `/qa/popular-questions`、`/popular-questions` | 可继续完善"最优问法"自动沉淀 |
| 23 | 问答/文档库 | 文档类型选择 | 部分完成 | `/qa/knowledge-bases`、用户知识库列表 | AIDGP 知识库配置未真实同步；前端弹窗需验收 |
| 24 | 问答/文档库 | 文档权限控制 | 已完成 | `accessibleKnowledgeCodes`、`canAccessDocument` | 需对 AIDGP 权限同步后做端到端测试 |
| 25 | 问答/解析检索 | 阅件/办件文档覆盖 | 未完成 | 仅本地 demo 知识库 | 需 AIDGP 文档类型字段和同步 |
| 26 | 问答/解析检索 | 附件内容识别 | 未完成 | 未发现附件解析管道 | 需 AIDGP 或本地解析结果同步 |
| 27 | 问答/解析检索 | 管理/执行文件联动 | 未完成 | 未发现图谱模型 | 需关系模型、推荐和合规检查逻辑 |
| 28 | 问答/解析检索 | 文件版本智能识别 | 未完成 | 未发现版本过滤模型 | 需 effective_status、version、superseded_by 等字段 |
| 29 | 问答/输入 | 多行文字输入 | 不在本仓库 | 后端支持 message | 前端 |
| 30 | 问答/输入 | 发送/停止控制 | 不在本仓库 | 后端 SSE | 前端中断控制需验证 |
| 31 | 问答/模型 | 深度思考 | 部分完成 | `DeepThinking` 参数、stream 中有思考事件设计 | 需确认模型返回和前端展示 |
| 32 | 问答/模型 | 联网搜索 | 部分完成 | `/qa/web-search`、`WebSearch` 参数 | 当前只返回占位，不会真实搜索 |
| 33 | 问答/结果 | AI 思考过程展示 | 部分完成 | `DeepThinking` 参数 | 前端和模型输出稳定性待验收 |
| 34 | 问答/结果 | 结构化回答 | 已完成 | `streamQaChat` 基于召回和 LLM SSE 输出 | 依赖模型质量，需 Prompt 回归测试 |
| 35 | 问答/结果 | 引用跳转 | 部分完成 | `/qa/citations/{id}`、`/qa/documents/{id}/view` | 前端滚动定位、附件定位需验收 |
| 36 | 问答/结果 | 底部参考资料 | 部分完成 | retrieval/citation 事件 | 前端折叠面板需验收 |
| 37 | 问答/结果 | 回答复制 | 不在本仓库 | 无后端要求 | 前端 |
| 38 | 问答/对话 | 上下文对话 | 已完成 | `qa_session`、`qa_message`、`mergeQaHistory` | 可补会话列表 |
| 39 | 问答/对话 | 对话重置 | 已完成 | `/qa/sessions/{sessionId}/reset` | 前端按钮需验证 |
| 40 | 问答/支撑 | 用户权限控制 | 部分完成 | 后端本地权限完整 | 未对接统一身份认证平台 |
| 41 | 问答/支撑 | 主题知识库同 AIDGP | 未完成 | `aidgp.NewClient` 仍 Mock | 需真实 AIDGP 知识库同步 |
| 42 | 后台 | 管理员入口 | 部分完成 | `/admin/login`、`/admin/profile` | 管理端前端不在本仓库；admin token 为开发 token |
| 43 | 后台 | 用户权限管理 | 已完成 | `/admin/users`、`/admin/users/{id}/permissions` | 前端需验收 |
| 44 | 后台 | 常用问题沉淀 | 部分完成 | 示例问题 CRUD、候选问题表 | 自动提炼标准问法能力不完整 |
| 45 | 后台 | 人流/车流接入管理 | 部分完成 | `/admin/data-sources`、车流 MQTT 配置 | 接入状态与真实 AIDGP/MQTT 启停未完全联动 |
| 46 | 后台 | 网格月度上传 | 部分完成 | `/admin/grid-data/upload` | 上线需模板、覆盖、回滚、审计和前端验收 |

## 3. AIDGP 对接现状

当前项目已有 AIDGP 对接壳：

- 配置位置：`config/config.yaml` 的 `qa.sync.provider`、`qa.sync.aidgp.baseUrl/appKey/appSecret`。
- 客户端接口：`internal/logic/aidgp/client.go`。
- 后台同步入口：`/admin/sync/knowledge-bases`、`/admin/sync/documents`、`/admin/sync/grid-data`、`/admin/sync/traffic-data`、`/admin/sync/population-data`。
- 问答侧同步入口：`/qa/sync/*`。
- 同步任务表：`qa_sync_task`。
- 同步日志表：`qa_sync_log`。

但真实对接未完成：

- `NewClient` 无论配置如何都返回 `MockClient`。
- Mock 同步只写任务和日志，不写正式知识库、文档、权限、网格、人流、车流业务表。
- 获取 token、刷新 token、签名、超时、重试、幂等、分页、字段映射都未实现。

因此，"使用问数功能时，网格/车流/人流数据去 AIDGP 平台查询"目前没有实现，需要按新增对接文档落地。

## 4. 车流能力现状

已完成：

- MQTT 订阅启动：`service.Traffic().StartMqttSubscriber(ctx)`。
- 原始报文解析：`HandleRawPayload`、`parseGatePayload`。
- 车流明细表：`traffic_gate_record`。
- 设备表：`traffic_gate_device`。
- 接入日志表：`traffic_ingest_log`。
- 接入状态表：`traffic_ingest_status`。
- 聚合查询：`/traffic/aggregate`，支持 day/hour/gate/plateRegion/inDir。
- 明细查询：`/traffic/records`。
- 设备查询：`/traffic/devices`。
- 接入状态：`/traffic/ingest/status`。
- 车牌识别：`/traffic/plate/recognize`。
- **【新】参数化聚合查询**：`Aggregate()` 支持 `GateName`（卡口名称 LIKE 过滤）和 `PlateRegion`（区域名称 LIKE 过滤），"拱北口岸车流"等带参数问题不再走全局查询。
- **【新】驻留时长 Top 快路径**：基于同一车牌多卡口首末时间差，输出 Top10 车辆驻留时长（分钟），含经过卡口数、首末出现时间，bar 图表。
- **【新】参数提取**：`ExtractFastPathParams()` 提取时间范围（今天/昨天/近N天/本周/上周/本月/近七天）、卡口名称（DB加载设备名最长匹配）、区域名称（珠海10个行政区）、对比基准。

不足：

- 未接 AIDGP 车流查询。
- **【改善】驻留时长已实现快路径**，但仅限近7天 Top10，未实现单车辆轨迹详情查询。
- 未实现预聚合表，较大数据量下依赖明细实时 group by。
- 节假日参数在接口中预留但聚合逻辑未使用。
- 港澳车同比、外地车来源 Top 固定口径未完全生产化。

## 5. 人流能力现状

已有能力：

- 快路径中存在 `population.trend.recent_days`、`population.compare.holiday`、`population.rank.region`。
- 通过动态探测 `population_record`、`population_gate_record`、`population_snapshot` 表名做 SQL 查询。
- **【新】按小时人流趋势**：`fastPopHourlyTrendQuery`，今日按 HOUR(snapshot_time) 分组，输出进入/离开/合计及高峰时段，line 图表。

不足：

- 没有正式人流表 DDL。
- 没有人流同步逻辑。
- 没有 AIDGP 字段映射。
- 没有流动人口识别模型。
- 节假日对比、上一年同期、人流进出维度均缺少上线级口径。

## 6. 网格能力现状

已有能力：

- 后台上传网格 Excel/CSV/XLS 到 `case_list`。
- 管理端查询、统计、删除。
- 问数重大案件基于本地规则评分。
- Prompt 已要求网格主题优先查询库。
- **【新】案件类型分布快路径**：`fastGridCaseTypeDistQuery`，按 case_type 分组 Top10，输出占比，pie 图表。

不足：

- 没有 AIDGP 网格增量同步。
- 月度覆盖规则需要业务化，例如按月份、批次、案件编号进行幂等覆盖。
- 结案率、同比环比、时段分布等指标依赖表字段，目前 `case_list` 字段较少，难以完整覆盖功能清单第二页的全部问法。

## 7. 迭代一&二新增能力明细

### 迭代一：澄清机制 + SSE 错误处理

| 改动 | 文件 | 说明 |
|---|---|---|
| `ClarificationData` 模型 | `internal/model/chat.go` | 新增 `Question` + `Options` 结构体，作为 `clarification` SSE 事件 Data |
| 流式状态机 | `internal/logic/ai/chan_out.go` | 三态（Buffering→Clarification/Normal），前 30 字符判定是否为 `chatdb-clarify` 块；跨 chunk 正确拼接 JSON；结束标记后额外文本补发为 message 事件 |
| `sendStreamError` | `internal/logic/ai/chan_out.go` | 流读取出错时发送结构化 `error` SSE 事件，替代原来的仅日志打印 |
| Controller SSE 循环 | `internal/controller/ai_chat/ai_chat_v1_chats.go` | 新增 `isClarification` 标记，澄清事件不写入会话历史；`case error` 改为结构化 error SSE 输出 |
| Prompt 强化 | `prompt/main.md` | 新增 3 个具体澄清示例 + 规则"输出澄清时不要输出三层回答结构" |

**单元测试**：7 个（有效JSON解析、无效JSON降级、跨chunk澄清检测、正常回答无误判、流错误事件、尾部额外文本、错误发送工具函数），全部通过。

### 迭代二：参数提取 + 快路径扩展 + 查询参数化

| 改动 | 文件 | 说明 |
|---|---|---|
| `ExtractedParams` 结构体 | `internal/controller/ai_chat/fast_path_params.go` | 5 种参数：DateFrom/DateTo/Days、GateName、RegionName、CompareRef |
| `dateRangeParams` | 同上 | 支持10种时间表达式：今天/昨天/近N天/本周/上周/本月/近七天等 |
| `gateNameParams` | 同上 | 从 `traffic_gate_device` 加载设备名，最长匹配，`sync.Once` + `sync.RWMutex` 缓存，支持外部刷新 |
| `regionNameParams` | 同上 | 珠海10个行政区最长匹配 |
| `compareRefParams` | 同上 | 识别 weekend/holiday/lastweek |
| `ExtractFastPathParams` | 同上 | 综合提取入口，参数写入 `FastPathIntent.Params` |
| `paramsToMap` | `fast_path_router.go` | 将非零参数转为 map 用于 `BuildCacheKey` |
| `fastPath` 结构体 | `ai_chat_v1_chats.go` | 新增 `params ExtractedParams` 字段 |
| `TrafficAggregateQuery` | `internal/model/traffic.go` | 新增 `GateName`/`PlateRegion` 字段 |
| `applyTrafficExtraFilters` | `internal/logic/traffic/query.go` | 新增 GateName LIKE / PlateRegion LIKE 过滤 |
| 所有车流快路径函数 | `ai_chat_v1_chats.go` | 签名改为 `(ctx, p ExtractedParams)`，使用 `p.DateFrom/p.DateTo` 替代硬编码时间，传递 `p.GateName/p.RegionName` 到查询 |
| `fpTrafficDwellTop` | 同上 | 新意图：驻留时长Top10（TIMESTAMPDIFF计算），bar 图表 |
| `fpPopHourlyTrend` | 同上 | 新意图：今日按小时人流趋势，line 图表 |
| `fpGridCaseTypeDist` | 同上 | 新意图：案件类型分布Top10，pie 图表 |
| 意图名称/图表规则/缓存 | `fast_path_router.go`、`ask_number_cache.go` | 新增3个意图映射、图表规则、dateRangeHash和TTL |

**单元测试**：7 个（dateRange 10种表达式、region 6种匹配、compareRef 4种基准、默认7天、今天、区域名、N天），全部通过。

## 8. 上线风险清单

| 风险 | 等级 | 说明 | 建议 |
|---|---|---|---|
| AIDGP 真实对接缺失 | P0 | 影响网格/人流/车流真实数据查询 | 优先实现 token + 查询 + 同步 |
| 人流数据模型缺失 | P0 | P0 功能多项无法验收 | 按 AIDGP 字段设计本地模型 |
| 源码中文乱码 | P0 | 用户可见错误信息和回答可能乱码 | 统一转 UTF-8，增加中文显示测试 |
| 明文密钥在配置文件 | P0 | 安全风险 | 移入环境变量/密钥系统，并轮换已暴露密钥 |
| 联网搜索占位 | P1 | 前端打开开关也不会返回真实联网结果 | 明确下线开关或实现供应商 |
| admin token 为开发 token | P0 | 后台鉴权不可上线 | 接入 JWT 或统一身份认证 |
| 车流大数据实时聚合压力 | P1 | 明细表 group by 可能慢 | 增加日/小时/卡口预聚合 |
| 缺少 AIDGP 端到端验收脚本 | P0 | 难判断对接是否成功 | 增加 smoke/e2e 脚本和对账报表 |
| **【新】澄清前端交互未闭环** | P1 | 后端已输出 clarification SSE 事件，但前端澄清选项 UI 待实现 | 前端仓库实现选择回调 + 重新提交 |
| **【新】参数提取覆盖面有限** | P2 | 时间表达仅覆盖10种中文模式，不支持绝对日期（如"5月1日"）、卡口名依赖 DB 加载首刷延迟 | 按需扩展正则/绝对日期解析，卡口名可考虑应用启动时预热 |

## 9. 本次验证结果

已执行：

```bash
go build ./...
go test ./...
```

结果：通过。

说明：首次在沙箱内运行因 Go 构建缓存目录权限失败，提权后测试通过。迭代一&二新增测试均通过。
