# 车流主题实施开发方案

## 1. 目标和边界

本方案基于 `docs/traffic-flow-development-plan.md`、桌面文档《卡口数据推送规则.docx》以及设备档案截图补充实施细节。目标是在 ChatDB 后端接入卡口 MQTT 车流识别数据，并在 `D:\code\morphic-main` 前端问数页完成车流统计、趋势图表、港澳车占比、设备筛选和接入状态展示。

首期范围：

- 接收 `zh-iot-jinshan-kakou` MQTT 主题推送。
- 清洗并入库卡口车辆识别报文。
- 建立设备档案表，兼容截图中的设备元数据。
- 提供聚合统计、设备列表、接入状态、车牌归属识别接口。
- 前端复用 `AskNumberChat` 的 `chatdb-chart` 图表渲染能力，补齐车流主题筛选、推荐问题和状态入口。

暂缓范围：

- 分钟级实时大屏。
- 图片长期归档。
- 精准驻留时长模型，只预留同车牌进出计算字段。
- 告警推送，只做状态接口和页面可见。

## 2. 外部数据和设备档案

### 2.1 MQTT 推送规则

推送方式为 MQTT 订阅：

| 项目 | 内容 |
| --- | --- |
| Broker | `iot-mqtt.qi-cloud.com:1883` |
| Topic | `zh-iot-jinshan-kakou` |
| ClientId | `jinshan_随机字符` |
| Username | `jinshan` |
| Password | 私发配置 |

报文为 JSON 字符串，必填字段：

| 字段 | 说明 | 示例 |
| --- | --- | --- |
| `cd` | 外部入库时间 | `2024-05-31 08:51:03` |
| `date` | 采集时间 | `2024-05-31 08:50:45` |
| `DeviceID` | 设备唯一编码 | `QC00070001090100000019` |
| `SnapshotTime` | 抓拍时间 | `2024-05-31 08:50:45` |
| `PlateChar` | 车牌 | `粤CD65***` |
| `DeviceName` | 镜头名称 | `凤凰山隧道(出高新)` |

重要可选字段：

| 字段 | 说明 |
| --- | --- |
| `VehicleDir` | 车流方向 |
| `CameraIP` | 摄像头 IP |
| `InDir` | 进出方向，`0` 进，`1` 出 |
| `VehicleType` / `VehicleTypeExt` | 车辆类型和扩展类型 |
| `VehicleColor` | 车辆颜色 |
| `PlateType` / `PlateColor` | 车牌类型和样式 |
| `VehicleSpeed` | 车速 |
| `LaneID` / `LaneDesc` / `LaneDirDesc` | 车道信息 |
| `CarDrvDir` | 行车方向 |
| `CarPreBrand` / `CarSubBrand` / `CarYearBrand` | 品牌、子品牌、年份 |
| `PlatePicture` / `PanoramaPicture` / `VehiclePicture` | 图片地址 |
| `Sim` | 镜头 SIM 卡号，有线设备可忽略 |

### 2.2 设备档案补充字段

设备档案截图包含以下字段，应落到设备主数据表中：

| 截图字段 | 建议字段 | 示例/说明 |
| --- | --- | --- |
| 设备名称 | `device_name` | `洪澳岛-出方向` |
| SN码 | `device_sn` | `QC000700010901000000...` |
| 分类 | `category` | `影像` |
| 型号 | `model` | `车辆识别 AI` |
| 连接状态 | `connection_status` | `在线` |
| 工作状态 | `work_status` | `启动监测道路卡口` |
| 区域 | `region` | 行政或片区 |
| 详细地址 | `address` | `广东省珠海市香洲区洪澳南主湾公园-停车场` |
| 经度 | `longitude` | `113.626915` |
| 纬度 | `latitude` | `22.393198` |
| 负责人 | `owner_name` | `平台管理员` |
| 负责人电话 | `owner_phone` | `18666111122` |
| 创建日期 | `device_created_at` | `2022-09-11 18:09:52` |
| 最后上报时间 | `last_reported_at` | `2025-01-10 11:08:02` |
| 备注 | `remark` | 可空 |

设备档案与推送报文的关联优先级：

1. `DeviceID` 对齐 `device_id`。
2. 若推送中的 `DeviceID` 与档案 `SN码` 一致，则同时写入 `device_sn`。
3. 若档案缺失，首次推送时用 `DeviceID + DeviceName + CameraIP` 自动创建轻量设备记录。
4. 后续由后台导入或手工维护补齐经纬度、地址、负责人等信息。

## 3. 后端实施方案

### 3.1 配置

改造文件：

- `config/config.yaml`
- `internal/model/config.go`
- `.env.example`
- `manifest/deploy/kustomize/*/configmap.yaml`

新增配置：

```yaml
traffic:
  mqtt:
    enabled: false
    broker: "tcp://iot-mqtt.qi-cloud.com:1883"
    topic: "zh-iot-jinshan-kakou"
    clientIdPrefix: "jinshan_"
    username: "jinshan"
    password: ""
    qos: 1
  ingest:
    dedupeWindowDays: 7
    rawPayloadRetainDays: 30
    noDataWarnMinutes: 30
```

环境变量覆盖：

- `CHATDB_TRAFFIC_MQTT_ENABLED`
- `CHATDB_TRAFFIC_MQTT_BROKER`
- `CHATDB_TRAFFIC_MQTT_TOPIC`
- `CHATDB_TRAFFIC_MQTT_USERNAME`
- `CHATDB_TRAFFIC_MQTT_PASSWORD`
- `CHATDB_TRAFFIC_MQTT_QOS`

### 3.2 数据表

#### `traffic_gate_record`

车流识别明细表，存储推送报文和结构化结果。

关键字段：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint pk | 主键 |
| `device_id` | varchar(64) | 设备唯一编码 |
| `device_name` | varchar(128) | 镜头名称 |
| `camera_ip` | varchar(64) | 摄像头 IP |
| `plate_char` | varchar(32) | 原始车牌 |
| `plate_normalized` | varchar(32) | 标准化车牌 |
| `plate_type` | varchar(64) | 车牌类型 |
| `plate_color` | varchar(64) | 车牌样式 |
| `vehicle_type` | varchar(64) | 车辆类型 |
| `vehicle_type_ext` | varchar(64) | 车辆类型扩展 |
| `vehicle_color` | varchar(64) | 车辆颜色 |
| `vehicle_speed` | int | 车速 |
| `in_dir` | int | 0 进，1 出 |
| `vehicle_dir` | varchar(64) | 车流方向 |
| `car_drv_dir` | varchar(64) | 行车方向 |
| `lane_id` | int | 车道号 |
| `lane_desc` | varchar(64) | 车道描述 |
| `lane_dir_desc` | varchar(64) | 车道方向描述 |
| `snapshot_time` | datetime | 抓拍时间，统计主时间 |
| `collect_time` | datetime | 采集时间 |
| `source_insert_time` | datetime | 外部入库时间 |
| `plate_picture` | text | 车牌图片 |
| `panorama_picture` | text | 全景图片 |
| `vehicle_picture` | text | 车辆图片 |
| `car_pre_brand` | varchar(64) | 品牌 |
| `car_sub_brand` | varchar(64) | 子品牌 |
| `car_year_brand` | varchar(32) | 年份 |
| `plate_origin` | varchar(64) | 归属地 |
| `plate_region_type` | varchar(32) | `mainland` / `hong_kong` / `macau` / `cross_border` / `unknown` |
| `is_hk_macau` | tinyint/bool | 是否港澳车 |
| `raw_payload` | longtext/json | 原始 JSON |
| `payload_hash` | varchar(64) | 去重哈希 |
| `create_time` / `update_time` | datetime/int | 系统时间 |

索引：

```sql
INDEX idx_snapshot_time (snapshot_time),
INDEX idx_device_time (device_id, snapshot_time),
INDEX idx_plate_time (plate_normalized, snapshot_time),
INDEX idx_hk_macau_time (is_hk_macau, snapshot_time),
UNIQUE KEY uk_record_dedupe (device_id, plate_normalized, snapshot_time)
```

#### `traffic_gate_device`

设备档案表，融合截图字段和推送设备字段。

关键字段：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint pk | 主键 |
| `device_id` | varchar(64) unique | 推送设备唯一编码 |
| `device_sn` | varchar(64) | 档案 SN 码 |
| `device_name` | varchar(128) | 设备名称 |
| `category` | varchar(64) | 分类 |
| `model` | varchar(128) | 型号 |
| `connection_status` | varchar(32) | 在线/离线 |
| `work_status` | varchar(128) | 工作状态 |
| `region` | varchar(128) | 区域 |
| `address` | varchar(255) | 详细地址 |
| `longitude` / `latitude` | decimal(10,6) | 经纬度 |
| `owner_name` / `owner_phone` | varchar | 负责人 |
| `enabled` | tinyint/bool | 是否启用 |
| `latest_seen_at` | datetime | 最近接收推送时间 |
| `last_reported_at` | datetime | 档案最后上报时间 |
| `device_created_at` | datetime | 档案创建时间 |
| `remark` | varchar(255) | 备注 |
| `create_time` / `update_time` | datetime/int | 系统时间 |

#### `traffic_ingest_log`

接入日志表。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `source` | 固定 `mqtt` |
| `topic` | MQTT topic |
| `status` | `success` / `failed` / `duplicate` |
| `message` | 错误或摘要 |
| `device_id` | 设备编码 |
| `payload_hash` | 报文哈希 |
| `raw_payload` | 原始报文 |
| `create_time` | 创建时间 |

#### `traffic_ingest_status`

接入状态表，便于接口读取和页面展示。

| 字段 | 说明 |
| --- | --- |
| `source` | `mqtt` |
| `enabled` | 是否启用 |
| `connected` | 是否连接 |
| `topic` | 当前订阅主题 |
| `latest_received_at` | 最新接收时间 |
| `today_received` | 今日成功入库数 |
| `latest_error` | 最近错误 |
| `update_time` | 更新时间 |

### 3.3 服务和代码模块

新增后端模块：

- `api/traffic/v1/traffic.go`：接口定义。
- `internal/controller/traffic`：控制器。
- `internal/logic/traffic`：接入、解析、聚合、设备和状态逻辑。
- `internal/service/traffic.go`：服务接口。
- `internal/model/traffic.go`：入参、出参和领域模型。

注册入口：

- 在 `internal/cmd/cmd.go` 的 `/api/v1` 鉴权组中绑定 `traffic.NewV1()`。
- 启动 HTTP 服务前初始化数据表。
- 当 `traffic.mqtt.enabled=true` 时启动后台订阅 goroutine。

MQTT 依赖：

```text
github.com/eclipse/paho.mqtt.golang
```

### 3.4 入库流程

1. 服务启动读取 `traffic.mqtt` 配置。
2. 生成 `clientIdPrefix + 随机串`。
3. 连接 Broker，订阅 Topic。
4. 收到消息后执行：
   - JSON 解析。
   - 校验 `cd`、`date`、`DeviceID`、`SnapshotTime`、`PlateChar`、`DeviceName`。
   - 解析时间，统一写入本地时区。
   - 标准化车牌。
   - 调用抽取后的车牌识别规则，写入 `plate_origin`、`plate_region_type`、`is_hk_macau`。
   - 计算 `payload_hash`，建议使用规范化 JSON + SHA256。
   - 按 `device_id + plate_normalized + snapshot_time` 幂等入库。
   - upsert `traffic_gate_device.latest_seen_at`。
   - 写入 `traffic_ingest_log` 和 `traffic_ingest_status`。
5. 断线自动重连。

车牌识别应从 `internal/logic/mcp/mcp_tool_vehicle.go` 中抽取纯函数，避免只有 MCP 工具能调用。建议新增：

- `internal/logic/traffic/plate.go`
- 或 `internal/logic/vehicle/plate.go`

输出统一结构：

```json
{
  "normalized": "粤Z1234港",
  "origin": "香港",
  "regionType": "cross_border",
  "isHongKongMacau": true,
  "basis": "匹配粤Z跨境牌，后缀为港"
}
```

### 3.5 API 设计

#### 聚合统计

```http
GET /api/v1/traffic/aggregate
```

查询参数：

| 参数 | 说明 |
| --- | --- |
| `dateFrom` / `dateTo` | 统计时间范围，基于 `snapshot_time` |
| `deviceId` | 设备/卡口 |
| `plate` | 车牌 |
| `groupBy` | `hour` / `day` / `gate` / `plateRegion` / `inDir` |
| `holiday` | 是否节假日，可选 |

返回：

```json
{
  "summary": {
    "total": 12000,
    "inCount": 6100,
    "outCount": 5900,
    "hkMacauCount": 320,
    "hkMacauRatio": 0.0267,
    "mainlandCount": 11680
  },
  "series": [
    {
      "name": "洪澳岛-出方向",
      "total": 1200,
      "inCount": 0,
      "outCount": 1200,
      "hkMacauCount": 31
    }
  ]
}
```

#### 明细查询

```http
GET /api/v1/traffic/records
```

用于“某车牌最近经过哪些卡口”“某卡口明细”等问题，也可供前端详情表格使用。支持 `dateFrom`、`dateTo`、`deviceId`、`plate`、`isHkMacau`、`page`、`pageSize`。

#### 设备列表

```http
GET /api/v1/traffic/devices
```

返回设备档案和在线状态，用于前端卡口筛选、接入状态和后续地图展示。

#### 车牌识别

```http
GET /api/v1/traffic/plate/recognize?plate=粤Z1234港
```

#### 接入状态

```http
GET /api/v1/traffic/ingest/status
```

返回 MQTT 连接、主题、最新接收时间、今日接收数、最近错误。

### 3.6 问数 Prompt

改造文件：

- `prompt/traffic.md`
- 必要时补充 `prompt/main.md`

要求：

- 明确车流主题优先查询 `traffic_gate_record`。
- 告诉模型 `snapshot_time` 是统计时间。
- 明确 `in_dir=0` 表示进，`in_dir=1` 表示出。
- 明确 `is_hk_macau` 用于港澳车占比。
- 指导模型在合适场景输出前端已支持的代码块：

````
```chatdb-chart
{
  "type": "bar",
  "title": "今日各卡口车流量",
  "x": ["洪澳岛-出方向", "凤凰山隧道"],
  "series": [
    { "name": "车流量", "data": [1200, 980] }
  ]
}
```
````

注意：当前前端 `components/ask-number-chat.tsx` 解析的是 `chatdb-chart` 和 `chatdb-line-chart`，不是普通 `chart`。后端或 Prompt 输出必须使用 `chatdb-chart`。

## 4. 前端实施方案

前端项目路径：`D:\code\morphic-main`。

### 4.1 现状

已存在能力：

- `app/ask/page.tsx` 使用 `AskNumberChat initialTopic="traffic"`。
- `app/api/question-chat/route.ts` 代理到 ChatDB 后端 `/api/v1/chats`。
- `components/ask-number-chat.tsx` 已支持：
  - SSE 流式问答。
  - `chatdb-chart` 图表代码块解析。
  - 折线图、柱状图、饼图、表格切换。
  - 图表复制为图片。
  - topic 为 `traffic` 的会话本地缓存。

首期前端应少造新组件，优先增强现有 `AskNumberChat`。

### 4.2 前端 API 代理

新增或扩展 Next.js API：

- `app/api/chatdb/traffic/aggregate/route.ts` -> `/api/v1/traffic/aggregate`
- `app/api/chatdb/traffic/devices/route.ts` -> `/api/v1/traffic/devices`
- `app/api/chatdb/traffic/records/route.ts` -> `/api/v1/traffic/records`
- `app/api/chatdb/traffic/ingest/status/route.ts` -> `/api/v1/traffic/ingest/status`

这些路由复用 `lib/chatdb/server.ts` 的 `chatDbFetch`，继承登录态和 `CHATDB_API_BASE`。

### 4.3 车流主题筛选栏

在 `components/ask-number-chat.tsx` 中，当 `topic === 'traffic'` 时显示轻量筛选区：

- 卡口选择：来自 `/api/chatdb/traffic/devices`。
- 日期范围：默认今天，可快速选择今天、近 7 天、近 30 天。
- 节假日开关：首期作为问数上下文参数或快捷问题文本拼接。
- 图表类型不新增独立控件，继续使用图表卡片内切换。

提交问题时，将筛选条件转为自然语言上下文追加到请求，例如：

```text
当前筛选条件：卡口=洪澳岛-出方向，时间=2026-05-08 至 2026-05-08，只统计抓拍时间 snapshot_time。
```

中期再把筛选条件作为结构化参数透传到后端。

### 4.4 车流状态入口

在车流主题顶部增加一行紧凑状态：

- MQTT：已连接/未连接。
- 最新接收：时间。
- 今日入库：数量。
- 最近错误：有错误时显示提示。

数据来自 `/api/chatdb/traffic/ingest/status`。

### 4.5 推荐问题

车流主题推荐问题：

- 今天各卡口车流量是多少？
- 过去一周车流趋势如何？
- 港澳车占比是多少？
- 哪个卡口车流最高？
- 洪澳岛-出方向今天进出车辆分别多少？
- 某车牌最近经过哪些卡口？
- 节假日车流和平日相比有什么变化？

可以优先接入已有推荐问题接口或在 `AskNumberChat` 内按 topic 配置兜底问题。

### 4.6 图表输出约定

后端/LLM 输出的图表必须符合当前前端结构：

柱状图：

```json
{
  "type": "bar",
  "title": "今日各卡口车流量",
  "x": ["洪澳岛-出方向", "凤凰山隧道"],
  "series": [
    { "name": "车流量", "data": [1200, 980] }
  ]
}
```

折线图：

```json
{
  "type": "line",
  "title": "过去一周车流趋势",
  "x": ["05-02", "05-03", "05-04"],
  "series": [
    { "name": "总车流量", "data": [920, 1010, 1180] }
  ]
}
```

饼图：

```json
{
  "type": "pie",
  "title": "港澳车占比",
  "labels": ["港澳车", "内地车"],
  "series": [
    { "name": "车辆数", "data": [320, 11680] }
  ]
}
```

表格：

```json
{
  "type": "table",
  "title": "最近过车明细",
  "table": {
    "columns": ["时间", "卡口", "车牌", "方向", "车辆类型"],
    "rows": [
      ["2026-05-08 09:12:33", "洪澳岛-出方向", "粤CD65***", "出", "轿车"]
    ]
  }
}
```

## 5. 阶段计划

### 阶段 1：后端基础表、配置和设备档案

工期建议：1.5 天。

交付：

- `traffic` 配置结构和环境变量覆盖。
- `traffic_gate_record`、`traffic_gate_device`、`traffic_ingest_log`、`traffic_ingest_status` 自动建表。
- 设备档案 upsert 方法。
- 本地测试 JSON 可写入明细表和设备表。

验收：

- 后端启动不影响现有 `/api/v1/chats`。
- 模拟报文写入后可查询到设备、车牌、抓拍时间。
- 截图中的设备档案字段均有落库位置。

### 阶段 2：MQTT 接入和幂等入库

工期建议：2 天。

交付：

- MQTT 客户端连接、订阅、断线重连。
- 必填字段校验。
- 解析失败、重复数据、成功入库日志。
- 接入状态维护。

验收：

- 配置真实账号后能接收 `zh-iot-jinshan-kakou`。
- 必填字段缺失时不入库且写失败日志。
- 重复推送不会重复写明细。
- `/api/v1/traffic/ingest/status` 能返回连接和最新接收时间。

### 阶段 3：车牌归属识别

工期建议：1 天。

交付：

- 从 MCP 工具抽取车牌识别纯函数。
- 入库时写入 `plate_origin`、`plate_region_type`、`is_hk_macau`。
- `/api/v1/traffic/plate/recognize`。

验收：

- `粤Z1234港` 识别为香港跨境车。
- `粤Z1234澳` 识别为澳门跨境车。
- `粤CD65***` 能兼容脱敏车牌，至少识别广东/珠海或未知但不报错。

### 阶段 4：聚合和明细接口

工期建议：2 天。

交付：

- `/api/v1/traffic/aggregate`
- `/api/v1/traffic/records`
- `/api/v1/traffic/devices`
- 按小时、日期、卡口、进出方向、港澳车维度聚合。

验收：

- 能返回总量、进出数量、港澳车数量和占比。
- 按卡口返回柱状图所需数据。
- 按日期/小时返回折线图所需数据。
- 明细接口支持分页和车牌查询。

### 阶段 5：问数 Prompt 和车流回答

工期建议：1 天。

交付：

- 更新 `prompt/traffic.md`。
- 规范 `chatdb-chart` 输出。
- 增加常见问题回答策略。

验收：

- 问“今天各卡口车流量是多少？”返回柱状图。
- 问“过去一周车流趋势如何？”返回折线图。
- 问“港澳车占比是多少？”返回饼图或带占比的结论。
- 问“某车牌最近经过哪些卡口？”返回表格。

### 阶段 6：前端车流体验

工期建议：2 天。

交付：

- 新增车流相关 Next.js API 代理。
- 车流主题筛选栏。
- MQTT 接入状态展示。
- 车流推荐问题。
- 修正当前前端中文乱码显示，确保 topic label、图表按钮、错误提示可读。

验收：

- `/ask?topic=traffic` 或现有车流入口可直接使用。
- 选择卡口和日期后，提交问题能带上筛选上下文。
- 图表移动端不溢出，表格可横向滚动。
- 后端断开 MQTT 时页面状态可见。

### 阶段 7：联调、测试和上线

工期建议：1.5 天。

交付：

- 单元测试：解析、去重、车牌识别、聚合 SQL。
- 接口测试：状态、设备、聚合、明细。
- 前端测试：图表解析、筛选上下文、状态展示。
- 配置和部署文档。

验收：

- `go test ./...` 通过。
- `npm run typecheck` 和关键前端测试通过。
- 真实 MQTT 推送稳定运行至少 1 小时。
- 无 MQTT 密码入库或提交到仓库。

## 6. 任务拆分建议

后端任务：

1. 建表和配置。
2. MQTT 消费和入库。
3. 设备档案 upsert。
4. 车牌识别抽取。
5. 聚合/明细/状态接口。
6. Prompt 改造。

前端任务：

1. 车流 API 代理。
2. 车流筛选栏。
3. 状态条。
4. 推荐问题。
5. 图表和表格适配。
6. 中文乱码清理。

联调任务：

1. 用文档示例报文写入。
2. 用真实 MQTT 账号订阅。
3. 验证问数图表输出。
4. 验证设备档案截图字段展示和筛选。

## 7. 风险和处理

| 风险 | 处理 |
| --- | --- |
| MQTT 密码泄露 | 只通过环境变量配置，示例文件留空 |
| 重复推送 | 唯一键和 payload hash 双保险 |
| 图片 URL 过期 | 首期只存 URL，页面不依赖图片做统计证据 |
| 车牌脱敏 | 标准化和归属识别兼容 `*`，识别失败也可入库 |
| 数据量增长 | 时间、设备、车牌索引必须首期建立 |
| 前端图表协议不一致 | Prompt 必须输出 `chatdb-chart`，不要输出普通 `chart` |
| 设备档案不全 | 推送首次自动创建轻量设备，后台再补充档案 |

## 8. 首期完成标准

- 后端可持续接收 MQTT 数据并写入车流明细。
- 设备档案支持截图字段，且可被前端用于卡口筛选。
- 问数端能回答车流量、趋势、港澳车占比、过车明细四类问题。
- 前端能渲染柱状图、折线图、饼图和表格。
- 接入状态可见，断连和长时间无数据有可排查信息。
