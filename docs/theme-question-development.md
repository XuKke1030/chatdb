# 问数主题化功能开发文档

## 1. 背景与目标

当前 ChatDB 已具备 AI 问答、数据库配置、主题选择、权限位、SSE 流式回复、MCP 工具调用等基础能力。后续目标是将系统升级为面向网格、人流、车流三类业务场景的主题化问数产品。

建设目标：

- 首页固定展示网格、人流、车流三大主题入口，并按用户权限自动隐藏不可访问主题。
- 支持异常告警展示、关闭、点击带入问题并跳转问答。
- 问答页携带主题上下文，使 AI 按不同主题规则生成 SQL、分析结论和可视化。
- 支持结构化回答、图表展示、澄清选项、洞察展开、复制、新问题开启、上下文追问。
- 建立后续数据接入、聚合指标库、权限配置、网格月度导入能力。

## 2. 当前项目基础

已有能力：

- 后端：Go + GoFrame，接口统一挂载在 `/api/v1`。
- 前端：Vue 3 + Vite + Element Plus。
- 主题：已有 `grid`、`population`、`traffic` 三个主题 prompt。
- 权限：用户表已有 `rule_level`，当前可用位掩码控制主题权限。
- AI：支持 OpenAI / DeepSeek，使用 Eino ReAct Agent + MCP tools。
- 工具：已有 SQL 查询、Excel 导出、时间、Redis、安全 Shell 等 MCP 工具。
- 流式回复：聊天接口使用 SSE 输出。

需要补齐：

- 独立首页和问答页路由结构。
- 告警数据模型与接口。
- 前端结构化图表、澄清选项、洞察分析组件。
- 更完整的用户主题权限配置。
- 数据接入与预聚合任务。

## 3. 实现可能性评估

| 功能 | 可实现性 | 难度 | 说明 |
|---|---:|---:|---|
| 首页主题入口 | 高 | 低 | 基于已有主题和权限位即可实现 |
| 无权限主题隐藏 | 高 | 低 | 前端根据用户权限过滤 |
| 点击主题进入问答 | 高 | 低 | 路由携带 topic 或写入 settings store |
| 实时异常告警 | 中 | 中-高 | 需要告警表、规则任务、前端告警区域 |
| 网格重大案件分析 | 高 | 中 | 依赖案件表字段，prompt + SQL 排序可完成 |
| 人流进出双折线图 | 高 | 中 | 前端需图表渲染协议 |
| 节假日人流对比 | 中 | 中 | 需要节假日维表或日期规则 |
| 流动人口识别 | 中 | 中-高 | 依赖户籍与运营商位置数据 |
| 车流预聚合 | 中 | 高 | 需要数据接入、批处理或实时聚合 |
| 车牌识别 | 高 | 中 | 规则识别可先落地，后续补规则 |
| 100 字输入限制 | 高 | 低 | 前端输入框控制 |
| 语音提问 | 中 | 中-高 | 需要录音、静默检测和 ASR 服务 |
| 澄清选项 | 中 | 中 | 需要 AI 返回结构化 clarify 数据 |
| 三级结构回复 | 高 | 中 | prompt 约束 + 前端分区渲染 |
| 洞察展开收起 | 高 | 中 | 前端组件 + AI 分区输出 |
| 复制文字和图表 | 高 | 中 | 文字复制简单，图表需 SVG/canvas 转图片 |
| 上下文追问 | 中 | 中 | 前端已有历史展示，后端需传入上下文 |
| 新问题开启 | 高 | 低 | 清空当前会话上下文 |
| AI 聚合与指标库 | 中 | 高 | 需要指标元数据、沉淀和复用机制 |
| 用户权限配置 | 中 | 中 | 后台管理页面和接口需补 |
| 数据接入 | 中 | 高 | 取决于外部系统接口和数据质量 |
| 网格月度导入 | 高 | 中 | CSV/Excel 上传、校验、覆盖、明细报告 |

## 4. 分期建议

### 4.1 一期：主题入口与问答闭环

目标：完成可演示、可使用的主题问数闭环。

范围：

- 首页展示三大主题入口。
- 无权限主题隐藏。
- 点击主题进入问答页并携带主题上下文。
- 输入框 100 字限制、回车发送、发送后清空。
- 回答中再次点击发送按钮停止回答。
- 新问题按钮清空当前上下文。
- 三级结构回答：精准结论、特征洞察、可视化。
- 趋势/对比类图表展示。
- 重大案件分析、人流进出对比、车牌识别基础能力。

验收：

- 不同权限账号看到的主题入口不同。
- 点击主题后，问答请求中带有对应 `topic`。
- 问“网格内影响最大的案件”时返回 Top1 案件和判断依据。
- 问“过去一周人流对比”时展示进/出双折线图。
- 问“粤C 是哪里的车牌”时能识别为珠海。
- 输入超过 100 字时阻止或提示。
- 回答过程中可停止。

### 4.2 二期：告警与权限配置

目标：完成告警展示、告警跳转问答和权限配置闭环。

范围：

- 首页主题入口下方展示告警区域。
- 告警支持关闭。
- 点击告警自动填入问题并跳转问答。
- 未授权主题的告警隐藏。
- 用户主题权限配置页面。
- 权限变更后前端刷新生效。

验收：

- 告警按主题权限过滤。
- 告警关闭后当前用户不再展示。
- 点击告警后问答页自动携带主题和问题。
- 修改用户权限后，重新拉取权限即可更新入口和告警。

### 4.3 三期：数据接入、聚合与导入

目标：补齐真实业务数据链路。

范围：

- 人流数据每日更新。
- 车流数据实时或准实时接入。
- 车流按车牌、日期、卡口、节假日预聚合。
- 流动人口识别任务。
- 常用指标沉淀为指标库。
- 网格月度 CSV/Excel 导入，支持覆盖更新和明细报告。

验收：

- 人流数据每天可自动入库。
- 车流数据可持续写入或同步。
- 聚合表可支持日期、卡口、车牌、节假日等维度查询。
- 流动人口统计口径清晰可复用。
- 网格导入成功/失败明细可下载或查看。

## 5. 前端设计

### 5.1 路由建议

建议拆分为：

```txt
/home
  首页：主题入口 + 告警区域

/chat
  问答页：聊天、图表、澄清、洞察、工具面板

/settings
  设置：数据库配置、模型配置

/admin/permissions
  用户主题权限配置

/admin/import/grid
  网格月度导入
```

当前项目的 `Home.vue` 已承担问答页职责，后续建议将其重命名或迁移为 `Chat.vue`，再新建真正的首页。

### 5.2 首页主题入口

主题配置：

```ts
const allTopics = [
  { label: '网格', value: 'grid', icon: 'ri-grid-line' },
  { label: '人流', value: 'population', icon: 'ri-user-location-line' },
  { label: '车流', value: 'traffic', icon: 'ri-roadster-line' }
];
```

展示逻辑：

```ts
const visibleTopics = computed(() =>
  allTopics.filter(topic => permissions.value.includes(topic.value))
);
```

点击逻辑：

```ts
settingsStore.updateSettings({ topic: topic.value });
router.push({ path: '/chat', query: { topic: topic.value } });
```

### 5.3 告警区域

展示位置：固定在主题入口下方。

交互：

- 点击关闭按钮：调用关闭接口。
- 点击告警主体：进入问答页，并把 `questionTemplate` 写入输入框或直接发起提问。
- 无权限主题告警不展示。

建议组件：

```txt
TopicEntryPanel.vue
AlertPanel.vue
AlertCard.vue
```

### 5.4 问题输入区

规则：

- 输入限制 100 字。
- 超出后提示或截断，推荐提示并阻止继续输入。
- Enter 发送，Shift + Enter 换行。如产品坚持“回车发送”，则输入框应改为单行或明确处理组合键。
- 发送后清空。
- 回答中发送按钮变为停止按钮。

状态：

```ts
const isLoading = ref(false);
const inputMessage = ref('');

const primaryAction = computed(() => isLoading.value ? 'stop' : 'send');
```

### 5.5 图表协议

AI 返回统一图表代码块，前端识别后渲染：

```markdown
```chatdb-chart
{
  "type": "line",
  "title": "过去一周人流进出对比",
  "x": ["04-18", "04-19", "04-20"],
  "series": [
    { "name": "进入人数", "data": [1200, 1350, 1100] },
    { "name": "离开人数", "data": [980, 1100, 1250] }
  ]
}
```
```

图表类型：

- `line`：趋势、时间序列。
- `bar`：排名、多分类对比。
- `pie`：占比结构。
- `table`：明细或多字段展示。

自动选择规则：

- 单数值、单占比：不展示图表。
- 趋势：折线图。
- 多分类排名：柱状图。
- 构成占比：饼图。
- 明细结果：表格。

后续支持用户手动切换图表类型时，前端保留同一份 chart data，切换渲染器即可。

### 5.6 澄清选项协议

当问题模糊时，AI 返回：

```markdown
```chatdb-clarify
{
  "question": "你想按哪个维度统计车流最多？",
  "options": [
    "按卡口统计",
    "按道路统计",
    "按行政区域统计"
  ],
  "hint": "都不准确重新输入即可"
}
```
```

前端渲染为 2-5 个按钮。用户点击选项后，将选项作为补充问题提交。

### 5.7 洞察展开收起

AI 回复建议分区：

```markdown
## 精准结论

## 特征洞察

## 可视化

## 推导分析
```

前端默认收起“推导分析”，点击展开后展示完整内容。

注意：不要展示模型内部不可验证推理，应展示可审计的数据口径、查询路径、字段选择、计算公式、排序规则。

## 6. 后端设计

### 6.1 主题权限

短期可继续使用 `user.rule_level` 位掩码：

```txt
grid       = 1
population = 2
traffic    = 4
```

长期建议独立表：

```sql
CREATE TABLE theme_permission (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  user_id INTEGER NOT NULL,
  topic VARCHAR(32) NOT NULL,
  enabled TINYINT NOT NULL DEFAULT 1,
  create_time INTEGER NOT NULL,
  update_time INTEGER NOT NULL,
  UNIQUE KEY uk_user_topic (user_id, topic)
);
```

推荐接口：

```txt
GET /api/v1/topics
返回当前用户可访问主题

GET /api/v1/permissions/users/:userId/topics
查询指定用户主题权限

PUT /api/v1/permissions/users/:userId/topics
更新指定用户主题权限
```

### 6.2 告警

告警表：

```sql
CREATE TABLE alert_event (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  topic VARCHAR(32) NOT NULL,
  title VARCHAR(128) NOT NULL,
  content TEXT,
  question_template VARCHAR(255) NOT NULL,
  alert_level VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  create_time INTEGER NOT NULL,
  update_time INTEGER NOT NULL
);
```

用户关闭记录：

```sql
CREATE TABLE alert_user_state (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  alert_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  status VARCHAR(32) NOT NULL,
  create_time INTEGER NOT NULL,
  update_time INTEGER NOT NULL,
  UNIQUE KEY uk_alert_user (alert_id, user_id)
);
```

接口：

```txt
GET /api/v1/alerts
返回当前用户可见告警

POST /api/v1/alerts/:id/close
关闭当前用户的指定告警
```

初期告警刷新方案：前端每 30 秒轮询。后期升级 WebSocket 或 SSE 推送。

### 6.3 聊天上下文

当前聊天请求只有单条 message，建议扩展：

```json
{
  "ai": "deepseek",
  "model": "deepseek-chat",
  "topic": "traffic",
  "databaseId": 1,
  "message": "过去一周车流趋势",
  "sessionId": "optional",
  "history": [
    { "role": "user", "content": "先看拱北口岸" },
    { "role": "assistant", "content": "..." }
  ]
}
```

处理规则：

- 默认保留当前会话上下文。
- 新问题按钮清空前端历史并生成新 session。
- 后端按最大轮数或 token 长度截断历史。

### 6.4 停止回答

短期：前端中断 `fetch` 请求。

推荐实现：

```ts
const controller = new AbortController();
fetch(url, { signal: controller.signal });
controller.abort();
```

后端如需显式停止，可增加：

```txt
POST /api/v1/chats/:traceId/stop
```

## 7. 主题能力设计

### 7.1 网格：重大案件分析

触发问题：

- 网格内影响最大的案件
- 最难处理的案件
- 重大案件 Top1
- 哪个案件影响最大

查询策略：

1. 查询案件表结构。
2. 识别案件名称、网格名称、影响范围、涉及人数、处置耗时、风险等级、状态、转派次数、协同部门数等字段。
3. 优先使用已有综合分、影响分、难度分。
4. 如无现成分数，则按可用字段构造排序。
5. 默认取 Top1。

回答必须包含：

- 单一案件名称。
- 判断依据。
- 说明按“影响 / 难度”排序取 Top1。

示例回答结构：

```markdown
## 精准结论
当前网格内影响 / 难度最高的案件是：XX 案件。

## 特征洞察
判断依据：涉及人数 320 人，影响 4 个小区，处置耗时 72 小时，协同 5 个部门，当前仍处置中。系统按影响范围、涉及人数、处置复杂度综合排序后取 Top1。
```

### 7.2 人流：进出对比

触发问题：

- 过去一周人流对比
- 近 7 天进出人数趋势
- 节假日人流对比

查询策略：

- 原始记录：按日期 + 进出方向聚合。
- 汇总表：直接查询日汇总。
- 节假日：对比当前节假日、上一年同期、节前 7 天。

回答要求：

- 给出趋势结论。
- 给出进入人数、离开人数双折线图。
- 给出峰值、低谷、进出差值。

### 7.3 人流：流动人口识别

定义：

户籍不在本地，但手机运营商数据显示当前位于本地的用户。

识别条件：

```txt
户籍地 != 本地
AND 当前所在地 / 运营商定位 / 基站定位 / 最近驻留地 = 本地
```

统计维度：

- 区域 / 网格。
- 来源地。
- 年龄段。
- 停留时长。
- 日期。

回答要求：

- 必须说明识别口径。
- 不得混淆户籍人口、常住人口、流动人口。

### 7.4 车流：数据聚合

聚合维度：

- 车牌。
- 日期。
- 卡口。
- 方向。
- 节假日。
- 港澳车来源。

推荐聚合表：

```sql
CREATE TABLE traffic_daily_agg (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  stat_date DATE NOT NULL,
  checkpoint_code VARCHAR(64),
  checkpoint_name VARCHAR(128),
  direction VARCHAR(32),
  vehicle_count INTEGER NOT NULL DEFAULT 0,
  unique_plate_count INTEGER NOT NULL DEFAULT 0,
  avg_stay_minutes DECIMAL(10,2),
  hk_vehicle_count INTEGER NOT NULL DEFAULT 0,
  mo_vehicle_count INTEGER NOT NULL DEFAULT 0,
  hk_mo_vehicle_count INTEGER NOT NULL DEFAULT 0,
  hk_mo_vehicle_ratio DECIMAL(8,4),
  holiday_flag TINYINT NOT NULL DEFAULT 0,
  create_time INTEGER NOT NULL,
  update_time INTEGER NOT NULL
);
```

### 7.5 车流：车牌识别

识别范围：

- 内地车牌：根据省份简称 + 字母判断地区，例如粤C为珠海。
- 粤 Z 跨境牌：识别带“粤 Z + 港 / 澳”后缀的跨境车辆。
- 纯港澳牌：识别香港 / 澳门本地车牌格式。

建议 MCP 工具：

```txt
RecognizeVehiclePlate
输入：plateNumber
输出：车牌类型、来源地、省份、城市、是否港澳车、判断依据、置信度
```

批量统计港澳车时：

- 粤Z港计入香港跨境车。
- 粤Z澳计入澳门跨境车。
- 香港本地牌计入香港车。
- 澳门本地牌计入澳门车。
- 无法确定时归为未知境外牌或未知车牌。

## 8. 数据接入与导入

### 8.1 人流数据

频率：每日更新一次。

推荐字段：

```txt
date
region_code
region_name
grid_code
grid_name
in_count
out_count
source_type
create_time
```

### 8.2 车流数据

频率：实时或准实时。

推荐原始表字段：

```txt
pass_time
plate_number
checkpoint_code
checkpoint_name
direction
vehicle_type
speed
image_url
create_time
```

### 8.3 网格月度导入

能力：

- 支持 CSV / Excel 上传。
- 支持按月份和网格编码覆盖更新。
- 重复导入以最新数据为准。
- 记录成功/失败明细。

推荐接口：

```txt
POST /api/v1/import/grid/monthly
GET  /api/v1/import/tasks
GET  /api/v1/import/tasks/:id/errors
```

导入任务表：

```sql
CREATE TABLE import_task (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  topic VARCHAR(32) NOT NULL,
  import_type VARCHAR(64) NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  success_count INTEGER NOT NULL DEFAULT 0,
  fail_count INTEGER NOT NULL DEFAULT 0,
  error_detail TEXT,
  create_time INTEGER NOT NULL,
  update_time INTEGER NOT NULL
);
```

## 9. 指标库设计

目标：

将 AI 经常实时计算的指标沉淀为可复用指标，减少重复 SQL 生成和重复计算。

指标定义表：

```sql
CREATE TABLE metric_definition (
  id INTEGER PRIMARY KEY AUTO_INCREMENT,
  topic VARCHAR(32) NOT NULL,
  metric_code VARCHAR(64) NOT NULL,
  metric_name VARCHAR(128) NOT NULL,
  description TEXT,
  dimensions TEXT,
  compute_sql TEXT,
  enabled TINYINT NOT NULL DEFAULT 1,
  create_time INTEGER NOT NULL,
  update_time INTEGER NOT NULL,
  UNIQUE KEY uk_metric_code (metric_code)
);
```

沉淀策略：

- 高频问题对应的 SQL 人工审核后沉淀。
- 车流、人流等高成本查询优先沉淀。
- 指标口径必须写清楚数据源、过滤条件、时间范围和维度。

## 10. AI Prompt 规范

主 prompt 要求：

- 不清楚表结构时必须先查表结构。
- 需要数据时必须调用 SQL 工具。
- 返回结果必须使用用户语言。
- 不直接暴露内部 id。
- 模糊问题应先生成澄清选项。
- 回答遵循三级结构。

主题 prompt 要求：

- 网格主题：强化重大案件排序和 Top1 输出。
- 人流主题：强化进出维度、节假日对比、流动人口识别口径。
- 车流主题：强化聚合维度、港澳车口径、车牌识别工具调用。

## 11. 复制能力

文字复制：

- 复制纯文本。
- 去除按钮、图表控制区等 UI 文案。

图表复制：

- SVG 图表可序列化为 Blob。
- Canvas 图表可调用 `toBlob`。
- 复制到剪贴板时使用 `ClipboardItem`。

兼容方案：

- 如果浏览器不支持图片复制，则提供下载图片按钮。

## 12. 验收清单

一期：

- [ ] 首页展示三大主题入口。
- [ ] 无权限主题入口自动隐藏。
- [ ] 点击主题进入问答页并携带 topic。
- [ ] 输入框限制 100 字。
- [ ] 回车发送。
- [ ] 发送后输入框清空。
- [ ] 回答中可停止。
- [ ] 新问题按钮可清空上下文。
- [ ] 网格重大案件返回 Top1 和依据。
- [ ] 人流对比显示进/出双折线图。
- [ ] 车牌识别支持粤C、粤Z港、粤Z澳、港澳本地牌基础规则。

二期：

- [ ] 首页告警区域展示在主题入口下方。
- [ ] 告警支持手动关闭。
- [ ] 点击告警自动进入问答并填入问题。
- [ ] 无权限用户不展示对应告警。
- [ ] 支持配置用户主题权限。
- [ ] 权限变更后前端可刷新生效。

三期：

- [ ] 人流数据每日更新。
- [ ] 车流数据实时或准实时接入。
- [ ] 车流聚合表可按车牌、日期、卡口、节假日查询。
- [ ] 流动人口识别口径落库或可复用。
- [ ] 网格月度导入支持 CSV / Excel。
- [ ] 重复导入覆盖最新数据。
- [ ] 导入后可查看成功/失败明细。
- [ ] 高频指标可沉淀到指标库。

## 13. 风险与注意事项

- 数据质量决定问答质量。上线前必须确认每个主题的真实表结构、字段含义和数据更新频率。
- 节假日对比需要稳定的节假日维表，否则容易出现同期口径不一致。
- 流动人口识别涉及个人位置数据，必须确认脱敏、授权和合规要求。
- 车牌识别规则要持续维护，港澳本地牌格式存在边界情况，建议返回置信度。
- AI 生成 SQL 必须受只读模式、权限范围和 SQL 安全规则约束。
- 告警推送初期建议轮询，等规则稳定后再上 WebSocket。
- 指标库 SQL 必须经过人工审核，不建议直接沉淀未经确认的 AI SQL。

## 14. 推荐实施顺序

1. 拆分首页和问答页。
2. 完成主题入口与权限过滤。
3. 完成问答页 topic 传递和新问题按钮。
4. 完成输入区限制、停止回答。
5. 完成结构化回复和图表协议。
6. 完成网格、人流、车流主题 prompt 升级。
7. 完成车牌识别工具。
8. 完成告警表、接口和首页告警区。
9. 完成用户权限配置。
10. 完成人流、车流、网格导入与聚合任务。
11. 建立指标库和指标审核流程。
