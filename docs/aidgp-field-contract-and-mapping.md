# AIDGP 字段契约与本地映射说明

## 1. 总体原则

ChatDB 主动调用 AIDGP 获取数据。所有 AIDGP 返回记录先写入 `aidgp_sync_record`，再按 `sync_type` 映射到本地业务表。

幂等键优先级：

1. `externalId`
2. `external_id`
3. `id`
4. 业务主键字段，例如 `documentId`、`caseNumber`、`recordId`

版本字段优先级：

1. `syncVersion`
2. `sync_version`
3. `version`
4. `updateTime` / `updatedAt`

## 2. 通用原始记录表

表：`aidgp_sync_record`

用途：

- 保存 AIDGP 原始 payload。
- 支持接口字段变化时追溯。
- 字段不完整时仍可留痕，避免同步任务完全丢数。

唯一键：`sync_type + external_id`

## 3. 知识库映射

AIDGP 字段：

- `knowledgeCode` / `code`
- `knowledgeName` / `name`
- `description`
- `enabled`
- `sort`
- `externalId`
- `syncVersion`
- `permissionHash`

本地表：`qa_knowledge_base`

幂等键：`code`

## 4. 文档映射

AIDGP 字段：

- `documentId` / `externalId`
- `knowledgeCode`
- `title` / `documentTitle` / `name`
- `fileName`
- `fileType`
- `status`

本地表：`qa_document`

幂等键：`source_provider = aidgp + external_id`

## 5. 车流映射

AIDGP 字段：

- `deviceId` / `gateId` / `cameraId`
- `deviceName` / `gateName` / `cameraName`
- `plateNormalized` / `plateNo` / `plate` / `plateChar`
- `snapshotTime` / `captureTime` / `passTime` / `metricTime`
- `inDir` / `direction` / `directionCode`
- `originCity`
- `isHkMacau`

本地表：

- 明细：`traffic_gate_record`
- 设备：`traffic_gate_device`
- 预留聚合：`traffic_metric_hourly`、`traffic_metric_daily`、`traffic_vehicle_stay_daily`

明细幂等键沿用现有规则：`device_id + plate_normalized + snapshot_time`

## 6. 人流映射

AIDGP 字段：

- `externalId`
- `metricTime` / `snapshotTime` / `time` / `date`
- `region`
- `gridName`
- `inCount` / `enterCount`
- `outCount` / `leaveCount`
- `netInCount`
- `floatingPopulationCount`

本地表：

- 明细：`population_flow_record`
- 预留聚合：`population_metric_hourly`、`population_metric_daily`、`population_floating_daily`

幂等键：`external_id`

## 7. 网格映射

AIDGP 字段：

- `externalId`
- `caseNumber` / `caseNo`
- `region`
- `community`
- `gridName`
- `caseType1`
- `caseType2` / `caseType`
- `caseTitle` / `title`
- `caseStatus` / `status`
- `reportTime` / `createTime`
- `closeTime` / `finishTime`
- `majorScore`

本地表：

- 明细：`grid_case_record`
- 预留聚合：`grid_metric_daily`、`grid_metric_monthly`

幂等键：`external_id`

## 8. 当前已实现范围

- AIDGP HTTP 同步结果分页拉取。
- 原始 payload 幂等留痕。
- 知识库、文档、车流、人流、网格首批字段映射落库。
- 字段不完整时写同步日志，不中断整个批次。

下一步需要实现：

- 文档分段独立同步。
- AIDGP 权限 payload 到本地用户/知识库/文档权限的最终用户映射。
- 人流、网格聚合表刷新任务。

## 9. 车流生产化进展

已实现：

- AIDGP 车流记录映射到 `traffic_gate_record` 和 `traffic_gate_device`。
- AIDGP 车流同步结束后刷新最近窗口聚合表。
- 刷新表：
  - `traffic_metric_hourly`
  - `traffic_metric_daily`
  - `traffic_vehicle_stay_daily`
- `/traffic/aggregate` 在以下场景优先读取聚合表：
  - `groupBy=day`
  - `groupBy=hour`
  - `groupBy=gate`
- 当聚合表无数据，或查询条件需要明细能力时，自动回退到 `traffic_gate_record` 实时聚合。

## 10. 网格生产化进展

已实现：

- AIDGP 网格记录映射到 `grid_case_record`。
- AIDGP 网格同步结束后刷新指标表：
  - `grid_metric_daily`
  - `grid_metric_monthly`
- 问数网格快路径优先读取 `grid_case_record`：
  - 案件总量
  - 结案率
  - 区域排名
  - 案件类型分布
- 重大案件 Top1 分析优先读取 `grid_case_record`，无 AIDGP 数据时兼容旧 `case_list`。

仍需后续增强：

- `groupBy=plateRegion` 和 `groupBy=inDir` 的预聚合支持。
- 车流节假日、来源地、驻留时长问数快路径进一步改为优先读聚合表。
- 按 AIDGP 实际增量窗口精确刷新聚合，目前默认刷新最近 7 天窗口。
- 网格趋势类问法需要进一步接入 `grid_metric_daily/monthly` 输出折线图。
