# 车流主题开发计划

## 1. 背景与目标

车流主题用于接收卡口车辆识别数据，并在问数端提供车流统计、趋势图表、港澳车占比、车牌归属识别等能力。

外部车流数据来源为 MQTT 推送，接收方订阅指定主题后获取 JSON 格式卡口识别报文。系统需要完成数据接入、清洗、存储、聚合统计和前端可视化展示。

## 2. 外部推送规则摘要

推送方式：MQTT 订阅。

连接信息：

| 项目 | 内容 |
| --- | --- |
| Broker | `iot-mqtt.qi-cloud.com:1883` |
| Topic | `zh-iot-jinshan-kakou` |
| ClientId | `jinshan_随机字符` |
| Username | `jinshan` |
| Password | 私发配置 |

消息格式：标准 JSON 字符串。

核心必填字段：

| 字段 | 说明 |
| --- | --- |
| `cd` | 入库时间 |
| `date` | 采集时间 |
| `DeviceID` | 设备唯一编码 |
| `SnapshotTime` | 抓拍时间 |
| `PlateChar` | 车牌 |
| `DeviceName` | 镜头名称 |

重要可选字段：

| 字段 | 说明 |
| --- | --- |
| `InDir` | 进出方向，0 进，1 出 |
| `CameraIP` | 摄像头 IP |
| `VehicleType` | 车辆类型 |
| `VehicleColor` | 车辆颜色 |
| `VehicleSpeed` | 车速 |
| `LaneID` | 车道号 |
| `PlatePicture` | 车牌图片 |
| `PanoramaPicture` | 全景图片 |
| `VehiclePicture` | 车辆图片 |

## 3. 功能范围

### 3.1 车流图表渲染

前端问数端支持车流可视化展示，并支持按以下维度切换：

- 卡口
- 日期
- 节假日

图表类型建议：

- 折线图：车流趋势
- 柱状图：卡口对比
- 饼图/环图：港澳车占比、进出方向占比
- 表格：明细和 TopN 排名

### 3.2 车流聚合数据接口

后端提供车流聚合统计接口，支持按车牌、日期、卡口、节假日维度预聚合或实时聚合核心指标。

核心指标：

- 总车流量
- 进方向数量
- 出方向数量
- 驻留时长，后续基于同车牌进出记录计算
- 港澳车数量
- 港澳车占比
- 内地车数量
- 按卡口排名
- 按小时分布
- 按日期趋势

### 3.3 车牌归属识别接口

后端提供车牌归属识别能力：

- 内地车牌按号段识别归属地区。
- 粤 Z 牌和纯港澳牌识别。
- 区分香港、澳门来源。
- 输出标准化车牌、车牌类型、归属地、是否港澳车。

项目已有 MCP 工具 `RecognizeVehiclePlate`，可以复用其规则，并沉淀为车流入库时的结构化字段。

## 4. 后端设计

### 4.1 配置项

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
```

敏感信息建议支持环境变量覆盖：

- `CHATDB_TRAFFIC_MQTT_BROKER`
- `CHATDB_TRAFFIC_MQTT_TOPIC`
- `CHATDB_TRAFFIC_MQTT_USERNAME`
- `CHATDB_TRAFFIC_MQTT_PASSWORD`

### 4.2 数据表

#### traffic_gate_record

存储卡口原始识别记录和清洗后的结构化字段。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `device_id` | 设备唯一编码 |
| `device_name` | 镜头名称 |
| `camera_ip` | 摄像头 IP |
| `plate_char` | 原始车牌 |
| `plate_normalized` | 标准化车牌 |
| `plate_type` | 车牌类型 |
| `plate_color` | 车牌颜色 |
| `vehicle_type` | 车辆类型 |
| `vehicle_color` | 车辆颜色 |
| `vehicle_speed` | 车速 |
| `in_dir` | 进出方向 |
| `vehicle_dir` | 车流方向 |
| `car_drv_dir` | 行车方向 |
| `lane_id` | 车道号 |
| `snapshot_time` | 抓拍时间 |
| `collect_time` | 采集时间 |
| `source_insert_time` | 外部入库时间 |
| `plate_picture` | 车牌图片 |
| `panorama_picture` | 全景图片 |
| `vehicle_picture` | 车辆图片 |
| `plate_origin` | 归属地 |
| `plate_region_type` | mainland / hong_kong / macau / cross_border / unknown |
| `is_hk_macau` | 是否港澳车 |
| `raw_payload` | 原始 JSON |
| `payload_hash` | 去重哈希 |
| `create_time` | 创建时间 |
| `update_time` | 更新时间 |

建议唯一约束：

```text
device_id + plate_normalized + snapshot_time
```

#### traffic_gate_device

可选，用于维护卡口设备元数据。

| 字段 | 说明 |
| --- | --- |
| `device_id` | 设备唯一编码 |
| `device_name` | 设备名称 |
| `camera_ip` | 摄像头 IP |
| `region` | 所属区域 |
| `gate_name` | 卡口名称 |
| `enabled` | 是否启用 |
| `latest_seen_at` | 最近接收时间 |

#### traffic_ingest_log

记录 MQTT 接入状态、错误和解析失败数据。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `source` | mqtt |
| `topic` | MQTT topic |
| `status` | success / failed |
| `message` | 错误信息 |
| `raw_payload` | 原始报文 |
| `create_time` | 创建时间 |

### 4.3 MQTT 接收服务

新增后端后台服务：

1. 服务启动时读取 `traffic.mqtt` 配置。
2. `enabled=true` 时连接 MQTT Broker。
3. 使用 `clientIdPrefix + 随机串` 生成 ClientId。
4. 订阅 `zh-iot-jinshan-kakou`。
5. 收到消息后：
   - 解析 JSON。
   - 校验必填字段。
   - 标准化车牌。
   - 识别车牌归属。
   - 计算 `payload_hash`。
   - 去重入库。
   - 更新设备最近接收时间。
   - 更新后台数据源 `traffic` 状态。
6. 连接断开后自动重连。

建议依赖：

```text
github.com/eclipse/paho.mqtt.golang
```

### 4.4 后端接口

#### 4.4.1 车流聚合数据接口

```http
GET /api/v1/traffic/aggregate
```

查询参数：

| 参数 | 说明 |
| --- | --- |
| `dateFrom` | 开始日期 |
| `dateTo` | 结束日期 |
| `deviceId` | 卡口设备 |
| `plate` | 车牌 |
| `groupBy` | hour / day / gate / plateRegion / inDir |
| `holiday` | 是否节假日，可选 |

返回：

```json
{
  "summary": {
    "total": 12000,
    "inCount": 6100,
    "outCount": 5900,
    "hkMacauCount": 320,
    "hkMacauRatio": 0.0267
  },
  "series": [
    {
      "name": "2026-05-01",
      "total": 1200,
      "inCount": 620,
      "outCount": 580,
      "hkMacauCount": 31
    }
  ]
}
```

#### 4.4.2 车牌归属识别接口

```http
GET /api/v1/traffic/plate/recognize?plate=粤Z1234港
```

返回：

```json
{
  "plate": "粤Z1234港",
  "normalized": "粤Z1234港",
  "origin": "香港",
  "regionType": "cross_border",
  "isHongKongMacau": true,
  "basis": "匹配粤Z跨境牌，后缀为港"
}
```

#### 4.4.3 车流接入状态接口

```http
GET /api/v1/traffic/ingest/status
```

返回：

```json
{
  "enabled": true,
  "connected": true,
  "topic": "zh-iot-jinshan-kakou",
  "latestReceivedAt": 1770000000,
  "todayReceived": 12000,
  "latestError": ""
}
```

## 5. 前端设计

### 5.1 问数回答图表

问数端在车流主题下支持图表渲染：

- 查询“今天各卡口车流量”时展示卡口柱状图。
- 查询“过去一周车流趋势”时展示折线图。
- 查询“港澳车占比”时展示饼图或环图。
- 查询“某卡口明细”时展示表格。

前端图表数据结构沿用现有问数结构化回答：

```json
```chart
{
  "type": "bar",
  "title": "今日各卡口车流量",
  "x": ["凤凰山隧道", "港湾大道"],
  "series": [
    {
      "name": "车流量",
      "data": [1200, 980]
    }
  ]
}
```
```

### 5.2 维度切换控件

车流图表支持：

- 卡口选择
- 日期范围
- 节假日切换
- 图表类型切换

短期实现可以先由问数回答返回图表，前端复用现有图表组件；中期再增加固定筛选控件。

## 6. 问数能力设计

### 6.1 推荐问题

新增车流主题推荐问题：

- 今天各卡口车流量是多少？
- 过去一周车流趋势如何？
- 港澳车占比是多少？
- 哪个卡口车流最高？
- 某车牌最近经过哪些卡口？
- 节假日车流和平日相比有什么变化？

### 6.2 SQL 查询能力

问数 Prompt 需要让 LLM 优先使用 `traffic_gate_record` 相关表，并明确字段含义：

- `snapshot_time` 用于车流统计时间。
- `device_id` / `device_name` 用于卡口维度。
- `in_dir` 用于进出方向。
- `is_hk_macau` 用于港澳车占比。
- `plate_normalized` 用于车牌查询。

## 7. 阶段计划

### 阶段 1：基础表和配置

目标：

- 新增配置结构。
- 新增车流数据表。
- 新增基础模型和入库方法。

验收：

- 后端启动时能自动建表。
- 本地可用测试 JSON 写入 `traffic_gate_record`。

### 阶段 2：MQTT 接收

目标：

- 接入 MQTT 客户端。
- 支持订阅主题并解析消息。
- 支持断线重连。
- 支持错误日志。

验收：

- 配置 MQTT 后能收到真实推送。
- 模拟消息能稳定入库。
- 必填字段缺失时不入库，并写失败日志。

### 阶段 3：车牌归属识别

目标：

- 入库时识别车牌归属。
- 新增车牌归属识别接口。
- 输出港澳车标识。

验收：

- `粤Zxxxx港` 识别为香港跨境车。
- `粤Zxxxx澳` 识别为澳门跨境车。
- 普通内地车识别省份。

### 阶段 4：聚合统计接口

目标：

- 新增 `/api/v1/traffic/aggregate`。
- 支持按日期、卡口、车牌、进出方向、港澳车维度统计。

验收：

- 能返回总量、进出数量、港澳车数量和占比。
- 支持按卡口/日期返回序列。

### 阶段 5：问数车流回答接入

目标：

- 更新车流主题 Prompt。
- 让问数可查询车流表。
- 结构化输出图表数据。

验收：

- “今天各卡口车流量”返回柱状图。
- “过去一周车流趋势”返回折线图。
- “港澳车占比”返回占比结果。

### 阶段 6：前端图表体验完善

目标：

- 复用现有问数图表组件。
- 增加车流维度切换。
- 支持卡口、日期、节假日切换。

验收：

- 图表在移动端宽度一致。
- 切换维度后图表和回答内容同步更新。

### 阶段 7：运维和监控

目标：

- 后台显示车流数据源状态。
- 显示 MQTT 连接状态、最新接收时间、今日接收量。
- 增加异常告警。

验收：

- MQTT 断线时后台状态可见。
- 当长时间无数据时产生告警。

## 8. 风险与注意事项

- MQTT 密码需要单独配置，不能写入代码仓库。
- 图片 URL 可能有有效期，不建议长期依赖外链作为唯一证据。
- 车牌可能脱敏，例如 `粤CD65***`，归属识别需兼容脱敏号牌。
- MQTT 可能重复投递，必须做幂等去重。
- 车流量大时需要按时间和卡口建索引，必要时做日级聚合表。
- 如果后续需要分钟级实时大屏，应引入 Redis 或时序聚合缓存。

## 9. 建议索引

```sql
INDEX idx_snapshot_time (snapshot_time),
INDEX idx_device_time (device_id, snapshot_time),
INDEX idx_plate_time (plate_normalized, snapshot_time),
INDEX idx_hk_macau_time (is_hk_macau, snapshot_time),
UNIQUE KEY uk_record_dedupe (device_id, plate_normalized, snapshot_time)
```

## 10. 当前项目改造点

需要新增或调整：

- `config/config.yaml`：新增 `traffic.mqtt` 配置。
- `internal/model/config.go`：新增 Traffic 配置结构。
- `internal/controller/traffic`：新增车流接口控制器。
- `api/traffic/v1`：新增接口定义。
- `internal/logic/traffic`：新增 MQTT 订阅、解析、入库、统计逻辑。
- `internal/cmd/cmd.go`：启动时注册接口和后台接收服务。
- `internal/logic/mcp/mcp_tool_vehicle.go`：复用或抽取车牌识别规则。
- 前端问数车流图表：复用现有 `AskNumberChat` 结构化图表渲染。

