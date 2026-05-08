# ChatDB 对接 AIDGP 聚合指标开发方案

## 1. 背景与目标

当前项目的数据来源需要统一切换到 AIDGP 平台，包括网格、人流、车流和知识库数据。ChatDB 不再作为原始业务明细数据的承载系统，只保留面向问数、图表和知识问答所需的聚合数据指标。

本方案的目标是：

- AIDGP 作为权威数据源。
- ChatDB 只存储聚合指标、指标目录、同步状态和知识库索引元信息。
- 不落库原始网格事件明细、人流明细、车流过车明细。
- 问答和图表优先查询本地聚合指标。
- 本地数据缺失、过期或知识库命中不足时，可按需调用 AIDGP 实时查询。
- 保持现有问数能力和 `chatdb-chart` 图表输出协议。

## 2. 总体架构

```mermaid
flowchart LR
  User["前端问数/知识问答"] --> ChatDB["ChatDB API / LLM 问数服务"]
  ChatDB --> LocalMetric["本地聚合指标库"]
  ChatDB --> LocalKB["本地知识库索引"]
  ChatDB --> AidgpClient["AIDGP 对接适配层"]
  AidgpClient --> Aidgp["AIDGP 平台"]
  Aidgp --> SyncJob["定时同步任务"]
  SyncJob --> LocalMetric
  SyncJob --> LocalKB
```

整体分为三条链路：

1. 同步链路：定时从 AIDGP 拉取网格、人流、车流和知识库聚合数据，写入本地聚合指标库。
2. 查询链路：用户提问时，优先查询本地聚合指标表和知识库索引。
3. 兜底链路：当本地数据不存在、超过新鲜度阈值或知识库需要全文详情时，实时调用 AIDGP，并根据策略回写本地。

## 3. 改造边界

### 3.1 ChatDB 负责

- AIDGP 接口适配、鉴权、签名和错误处理。
- 聚合指标同步、存储和查询。
- 指标目录和指标口径维护。
- 知识库索引同步和检索。
- 面向前端和 LLM 的统一指标查询 API。
- Prompt 改造，约束模型只查询聚合指标。

### 3.2 AIDGP 负责

- 原始业务数据采集和治理。
- 网格、人流、车流原始明细管理。
- 指标聚合计算。
- 知识库正文和文档详情管理。
- 对外提供聚合指标、知识库目录和知识库详情接口。

### 3.3 不在本期范围

- 原始明细长期存储。
- 车流 MQTT 明细接入作为主链路。
- 人流轨迹级数据入库。
- 网格事件全量明细入库。
- 知识库全文完整复制到 ChatDB。

## 4. 配置设计

当前项目已有 `config/config.yaml` 和 `internal/model/config.go`。建议新增统一 `aidgp` 配置：

```yaml
aidgp:
  enabled: true
  baseUrl: ""
  appKey: ""
  appSecret: ""
  timeoutSeconds: 10
  sync:
    enabled: true
    intervalMinutes: 15
    lookbackDays: 7
  grid:
    enabled: true
  population:
    enabled: true
  traffic:
    enabled: true
  knowledge:
    enabled: true
```

环境变量建议：

```text
CHATDB_AIDGP_ENABLED
CHATDB_AIDGP_BASE_URL
CHATDB_AIDGP_APP_KEY
CHATDB_AIDGP_APP_SECRET
CHATDB_AIDGP_TIMEOUT_SECONDS
CHATDB_AIDGP_SYNC_ENABLED
CHATDB_AIDGP_SYNC_INTERVAL_MINUTES
CHATDB_AIDGP_SYNC_LOOKBACK_DAYS
```

## 5. 代码模块设计

建议新增 AIDGP 适配模块：

```text
internal/logic/aidgp/
  client.go
  auth.go
  grid.go
  population.go
  traffic.go
  knowledge.go
  sync.go
```

模块职责：

- `client.go`：统一 HTTP Client、请求封装、响应解析、超时控制。
- `auth.go`：AIDGP 鉴权、签名、Token 或 Header 生成。
- `grid.go`：网格聚合指标接口适配。
- `population.go`：人流聚合指标接口适配。
- `traffic.go`：车流聚合指标接口适配。
- `knowledge.go`：知识库目录、搜索和详情接口适配。
- `sync.go`：统一同步调度、批次管理和失败处理。

对内部暴露稳定方法：

```go
FetchGridMetrics(ctx, req)
FetchPopulationMetrics(ctx, req)
FetchTrafficMetrics(ctx, req)
FetchKnowledgeItems(ctx, req)
SearchKnowledge(ctx, req)
GetKnowledgeDetail(ctx, docID)
```

## 6. 数据模型设计

### 6.1 `metric_aggregate` 聚合指标表

用于统一存储网格、人流、车流等业务域的聚合指标。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `domain` | varchar(32) | 业务域：`grid` / `population` / `traffic` |
| `metric_code` | varchar(64) | 指标编码 |
| `metric_name` | varchar(128) | 指标名称 |
| `metric_value` | decimal(20,4) | 指标值 |
| `metric_unit` | varchar(32) | 单位 |
| `stat_time` | datetime | 统计时间 |
| `stat_granularity` | varchar(16) | 粒度：`hour` / `day` / `month` |
| `region_code` | varchar(64) | 区域编码 |
| `region_name` | varchar(128) | 区域名称 |
| `grid_code` | varchar(64) | 网格编码 |
| `grid_name` | varchar(128) | 网格名称 |
| `dimension_json` | json/text | 扩展维度 |
| `source` | varchar(32) | 固定 `aidgp` |
| `source_updated_at` | datetime | AIDGP 数据更新时间 |
| `sync_batch_id` | varchar(64) | 同步批次 |
| `create_time` | datetime | 创建时间 |
| `update_time` | datetime | 更新时间 |

建议索引：

```sql
CREATE INDEX idx_metric_domain_code_time ON metric_aggregate(domain, metric_code, stat_time);
CREATE INDEX idx_metric_region_time ON metric_aggregate(region_code, stat_time);
CREATE INDEX idx_metric_grid_time ON metric_aggregate(grid_code, stat_time);
CREATE INDEX idx_metric_granularity_time ON metric_aggregate(stat_granularity, stat_time);
```

建议唯一键：

```sql
CREATE UNIQUE INDEX uk_metric_unique
ON metric_aggregate(domain, metric_code, stat_time, stat_granularity, region_code, grid_code);
```

如 AIDGP 指标存在更多维度，例如卡口、车辆类型、进出方向、人群类型，可将维度写入 `dimension_json`，并在高频维度稳定后再拆独立字段。

### 6.2 `metric_catalog` 指标目录表

用于维护指标口径，供 API、Prompt 和 LLM 查询使用。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `domain` | varchar(32) | 业务域 |
| `metric_code` | varchar(64) | 指标编码 |
| `metric_name` | varchar(128) | 指标名称 |
| `description` | text | 指标解释 |
| `unit` | varchar(32) | 单位 |
| `default_chart_type` | varchar(16) | 默认图表类型 |
| `dimensions` | json/text | 支持维度 |
| `enabled` | tinyint | 是否启用 |
| `create_time` | datetime | 创建时间 |
| `update_time` | datetime | 更新时间 |

### 6.3 `aidgp_sync_state` 同步状态表

用于记录每个业务域的同步状态。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `domain` | varchar(32) | 业务域 |
| `status` | varchar(32) | `running` / `success` / `failed` |
| `last_sync_at` | datetime | 最近同步时间 |
| `last_success_at` | datetime | 最近成功时间 |
| `last_error` | text | 最近错误 |
| `batch_id` | varchar(64) | 同步批次 |
| `total_count` | int | 本批同步数量 |
| `create_time` | datetime | 创建时间 |
| `update_time` | datetime | 更新时间 |

### 6.4 `knowledge_index` 知识库索引表

本地只保存知识库检索所需的轻量信息，不保存完整正文。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `doc_id` | varchar(128) | AIDGP 文档 ID |
| `title` | varchar(255) | 标题 |
| `summary` | text | 摘要 |
| `category` | varchar(128) | 分类 |
| `tags` | varchar(512) | 标签 |
| `content_hash` | varchar(64) | 内容哈希 |
| `embedding_id` | varchar(128) | 本地向量索引 ID，可选 |
| `external_url` | varchar(512) | AIDGP 文档地址，可选 |
| `source_updated_at` | datetime | AIDGP 更新时间 |
| `create_time` | datetime | 创建时间 |
| `update_time` | datetime | 更新时间 |

## 7. 指标体系设计

### 7.1 网格指标

首期建议同步：

| 指标编码 | 指标名称 | 单位 | 推荐图表 |
| --- | --- | --- | --- |
| `grid_count` | 网格数量 | 个 | bar |
| `grid_population` | 网格覆盖人口 | 人 | bar |
| `grid_event_total` | 网格事件总数 | 件 | line/bar |
| `grid_event_pending` | 待处理事件数 | 件 | bar |
| `grid_event_closed` | 已办结事件数 | 件 | bar |
| `grid_event_close_rate` | 办结率 | % | line |
| `grid_event_overdue` | 超期事件数 | 件 | bar |
| `grid_worker_count` | 网格员数量 | 人 | bar |
| `grid_device_online_rate` | 设备在线率 | % | line |

支持维度：

- 区域
- 街道
- 社区
- 网格
- 事件类型
- 事件状态
- 时间粒度

### 7.2 人流指标

首期建议同步：

| 指标编码 | 指标名称 | 单位 | 推荐图表 |
| --- | --- | --- | --- |
| `population_flow_total` | 总人流量 | 人次 | line |
| `population_in_count` | 进入人数 | 人次 | line |
| `population_out_count` | 离开人数 | 人次 | line |
| `population_net_in_count` | 净流入人数 | 人次 | line/bar |
| `population_peak_count` | 峰值人数 | 人 | line |
| `floating_population_count` | 流动人口数量 | 人 | bar |
| `resident_population_count` | 常住人口数量 | 人 | bar |
| `visitor_count` | 游客数量 | 人 | line/bar |
| `holiday_population_yoy` | 节假日人流同比 | % | line |

支持维度：

- 区域
- 网格
- 人群类型
- 来源地
- 去向地
- 进出方向
- 时间粒度

### 7.3 车流指标

当前项目已有 `traffic` 模块和车流明细设计。新方向下，车流主链路应切换为 AIDGP 聚合指标。

首期建议同步：

| 指标编码 | 指标名称 | 单位 | 推荐图表 |
| --- | --- | --- | --- |
| `traffic_total` | 总车流量 | 辆 | line/bar |
| `traffic_in_count` | 进车数量 | 辆 | line |
| `traffic_out_count` | 出车数量 | 辆 | line |
| `traffic_net_in_count` | 净流入车辆 | 辆 | line/bar |
| `traffic_hk_macau_count` | 港澳车数量 | 辆 | bar/pie |
| `traffic_hk_macau_ratio` | 港澳车占比 | % | pie |
| `traffic_gate_rank` | 卡口车流排名 | 辆 | bar |
| `traffic_peak_hour_count` | 高峰小时车流 | 辆 | line |
| `traffic_vehicle_type_dist` | 车辆类型分布 | 辆 | pie |

支持维度：

- 区域
- 网格
- 卡口
- 进出方向
- 车辆类型
- 车牌归属类型
- 时间粒度

## 8. API 设计

### 8.1 统一指标查询

```http
GET /api/v1/metrics/query
```

请求参数：

| 参数 | 说明 |
| --- | --- |
| `domain` | 业务域：`grid` / `population` / `traffic` |
| `metricCodes` | 指标编码，多个用逗号分隔 |
| `dateFrom` | 开始时间 |
| `dateTo` | 结束时间 |
| `granularity` | 粒度：`hour` / `day` / `month` |
| `regionCode` | 区域编码 |
| `gridCode` | 网格编码 |
| `dimensions` | 扩展维度 JSON，可选 |

返回示例：

```json
{
  "summary": {
    "traffic_total": 12000,
    "traffic_in_count": 6100,
    "traffic_out_count": 5900
  },
  "series": [
    {
      "time": "2026-05-01",
      "metricCode": "traffic_total",
      "metricName": "总车流量",
      "value": 1300,
      "unit": "辆"
    }
  ]
}
```

### 8.2 主题指标接口

为方便前端调用和保持主题边界，建议提供主题化接口：

```http
GET /api/v1/grid/metrics
GET /api/v1/population/metrics
GET /api/v1/traffic/metrics
```

这些接口内部复用统一指标查询逻辑。

### 8.3 知识库接口

```http
GET /api/v1/knowledge/search
GET /api/v1/knowledge/detail
```

`search` 优先查本地 `knowledge_index`。当需要正文或详情时，通过 `detail` 实时调用 AIDGP。

### 8.4 同步状态接口

```http
GET /api/v1/aidgp/sync/status
POST /api/v1/aidgp/sync/run
```

手动同步请求示例：

```json
{
  "domain": "traffic",
  "dateFrom": "2026-05-01",
  "dateTo": "2026-05-08"
}
```

## 9. 同步机制

### 9.1 定时同步

建议策略：

- 每 15 分钟同步近 7 天聚合指标。
- 每天凌晨同步近 30 天聚合指标，修正迟到数据。
- 每天同步一次指标目录和知识库索引。

同步过程：

1. 生成 `sync_batch_id`。
2. 更新 `aidgp_sync_state.status = running`。
3. 调用 AIDGP 对应业务域接口。
4. 标准化指标编码、时间、区域、网格和维度。
5. 按唯一键 upsert `metric_aggregate`。
6. 写入同步数量、成功时间和状态。
7. 失败时记录错误，并保留上一批成功数据。

### 9.2 实时兜底

当用户问题涉及“今天”“当前”“最新”“刚刚”“实时”等语义时，系统应检查本地数据新鲜度。

建议规则：

- 本地数据更新时间小于等于 15 分钟：直接使用本地聚合指标。
- 本地数据超过 15 分钟：调用 AIDGP 实时查询。
- 实时查询成功：返回最新结果，并可回写本地。
- 实时查询失败：返回本地最近成功数据，并提示数据更新时间。

### 9.3 手动补偿

管理接口支持按业务域和时间范围补偿同步，用于接口异常、历史数据修正和上线初始化。

## 10. Prompt 改造

需要改造：

- `prompt/grid.md`
- `prompt/population.md`
- `prompt/traffic.md`
- 必要时补充 `prompt/main.md`

统一规则：

- 只查询聚合指标表，不查询原始明细表。
- 指标必须来自 `metric_catalog`。
- 统计时间统一使用 `stat_time`。
- 统计粒度统一使用 `stat_granularity`。
- 网格、人流、车流问题优先按 `domain` 和 `metric_code` 查询。
- 缺少数据时要明确说明本地聚合指标未同步到该范围。
- 图表输出使用 `chatdb-chart`，不要输出普通 `chart`。

图表示例：

```chatdb-chart
{
  "type": "line",
  "title": "过去7天车流趋势",
  "x": ["05-02", "05-03", "05-04"],
  "series": [
    { "name": "总车流量", "data": [920, 1010, 1180] }
  ]
}
```

## 11. 当前项目调整建议

当前项目已有 `traffic` 模块、`qa.sync.aidgp` 配置和多个主题 Prompt。建议调整如下：

1. 将 `qa.sync.aidgp` 升级为全局 `aidgp` 配置，覆盖网格、人流、车流和知识库。
2. 保留现有 `traffic` API 外壳，但底层查询逐步切换为 `metric_aggregate`。
3. 不再扩展 MQTT 明细入库作为主链路，历史能力可保留为可选开关。
4. 新增统一 `metrics` 查询服务，供网格、人流、车流复用。
5. Prompt 从“查业务明细表”改成“查聚合指标表”。
6. 知识库改为 AIDGP 权威来源，本地只做索引和摘要。

## 12. 开发阶段计划

### 阶段 1：AIDGP 配置和适配层

预计工期：1-2 天。

交付内容：

- `aidgp` 配置结构。
- AIDGP Client。
- 鉴权、签名、超时、错误处理。
- Mock 接口测试。

验收标准：

- 配置可通过 YAML 和环境变量覆盖。
- 能成功调用 AIDGP Mock 接口。
- 接口异常时能返回明确错误。

### 阶段 2：聚合指标表和同步状态表

预计工期：1 天。

交付内容：

- `metric_aggregate`
- `metric_catalog`
- `aidgp_sync_state`
- `knowledge_index`
- 自动建表或初始化 SQL。

验收标准：

- 服务启动可初始化表结构。
- 指标数据可按唯一键 upsert。
- 同步状态可记录成功和失败。

### 阶段 3：网格、人流、车流指标同步

预计工期：2-3 天。

交付内容：

- 网格指标同步。
- 人流指标同步。
- 车流指标同步。
- 批次号、同步数量和错误记录。

验收标准：

- 能按时间范围同步指标。
- 重复同步不会产生重复数据。
- AIDGP 接口失败不会影响已有数据查询。

### 阶段 4：指标查询 API

预计工期：1-2 天。

交付内容：

- `/api/v1/metrics/query`
- `/api/v1/grid/metrics`
- `/api/v1/population/metrics`
- `/api/v1/traffic/metrics`
- 图表数据结构适配。

验收标准：

- 能查询汇总值。
- 能按日、小时、区域、网格等维度返回趋势数据。
- 能返回前端可渲染的图表数据。

### 阶段 5：知识库对接

预计工期：1-2 天。

交付内容：

- AIDGP 知识库目录同步。
- 本地知识库索引。
- 知识库搜索接口。
- 文档详情实时拉取。

验收标准：

- 可按关键词搜索知识库。
- 搜索结果包含标题、摘要、分类和更新时间。
- 需要详情时可实时获取 AIDGP 文档内容。

### 阶段 6：Prompt 和问数链路改造

预计工期：1 天。

交付内容：

- 网格、人流、车流 Prompt 改造。
- 指标目录注入问数上下文。
- `chatdb-chart` 输出约束。
- 缺数据提示策略。

验收标准：

- 问“今天各网格事件量”返回聚合指标结果。
- 问“过去 7 天人流趋势”返回折线图。
- 问“今日各卡口车流排名”返回柱状图。
- 不生成查询原始明细表的 SQL。

### 阶段 7：联调、测试和上线

预计工期：1-2 天。

交付内容：

- 单元测试。
- 接口测试。
- AIDGP 联调测试。
- 同步任务稳定性测试。
- 上线配置文档。

验收标准：

- 网格、人流、车流指标均可从 AIDGP 同步。
- 本地库不保存原始明细数据。
- 已同步数据在 AIDGP 短暂不可用时仍可查询。
- 同步状态可查看，可手动补偿。
- 前端图表正常渲染。

## 13. 风险与处理

| 风险 | 处理方式 |
| --- | --- |
| AIDGP 接口不稳定 | 保留最近成功同步数据，失败时记录同步状态 |
| 指标口径变化 | 通过 `metric_catalog` 管理指标口径和启停状态 |
| 聚合维度频繁变化 | 首期使用 `dimension_json` 承载扩展维度 |
| 实时数据延迟 | 查询结果展示数据更新时间 |
| 知识库正文过大 | 本地只存摘要和索引，正文按需实时拉取 |
| LLM 误查明细表 | Prompt 明确禁止查询原始明细，只允许查指标表 |
| 数据重复同步 | 使用业务唯一键和批次号 upsert |
| 密钥泄露 | `appSecret` 只通过环境变量注入，不提交到仓库 |

## 14. 首期完成标准

首期完成后应达到以下效果：

- ChatDB 已具备统一 AIDGP 对接能力。
- 网格、人流、车流聚合指标可同步到本地。
- 本地只保存聚合指标和知识库索引，不保存原始明细。
- 用户可以通过自然语言查询指标趋势、排名、占比和汇总。
- 前端可以渲染柱状图、折线图、饼图和表格。
- 知识库问题可以基于 AIDGP 知识库返回答案。
- AIDGP 异常时，系统可以使用最近成功同步的数据提供降级回答。

