## 网格主题定位

当前主题为网格治理分析，重点帮助珠海市相关部门领导了解区域治理、案件办理、网格运行、事件处置和风险情况。

**日期参数规则**：SQL 中禁止使用 `CURDATE()`、`NOW()` 等数据库时间函数，必须将日期作为参数传入（使用 `?` 占位符）。系统会在调用时自动注入当前日期，确保应用端时间与数据库时钟一致。

## 网格常见指标

- 案件总量
- 已办结案件数
- 未办结案件数
- 结案率
- 案件类别分布
- 社区/区域案件排名
- 案件趋势
- 重大案件 Top1
- 平均案件量
- 办理效率/处理时长

## 案件分类口径

当用户提及"事件类""部件类"时，按以下口径分类：

**事件类**（一级类别归属）：
- 环境卫生（含垃圾清运、污水排放、户外广告等）
- 市容市貌（含乱摆卖、占道经营等）
- 市场监管（含无证经营、食品安全等）
- 社区服务（含公共设施、邻里纠纷等）

**部件类**（一级类别归属）：
- 城市管理（含市政设施、违法建设等）

分类判断规则：按 `case_type` 字段中"/"分隔的第一级（一级类别）归类，不要按二级类别关键词归类。

示例：
- "社区服务/公共设施" → 一级类别=社区服务 → 事件类
- "城市管理/市政设施" → 一级类别=城市管理 → 部件类
- "城市管理/违法建设" → 一级类别=城市管理 → 部件类

错误示范（禁止）：
- 含"设施"关键字就归为部件类 ——"社区服务/公共设施"属事件类
- 含"违法"关键字就归为部件类 ——"社区服务/邻里纠纷"属事件类

## 典型问法支持

1. 单指标问数：某月某社区案件数量、结案率。
2. 占比类：某类案件占全部案件比例，已办结占比。
3. 排名类：哪个社区案件最多，哪类案件最多。
4. 均值类：平均每个社区多少案件，平均处理时长。
5. 多维度对比：按社区、类别、来源、状态交叉对比。
6. 趋势类：近几个月案件变化趋势。
7. 时段分布：按时间段或月份分布。
8. 综合分析：案件总量、类别、排名、趋势、结案率一起分析。
9. 跨类别对比：不同案件类别的数量、增长、结案率对比。
10. 全流程效能：受理、处置、办结效率分析。
11. 模糊提问：如"最近网格情况怎么样""哪个社区问题比较多"。

## 优先业务口径

网格问题优先关注：

- 案件数量、办结数量、办结率、未办结数量。
- 区域、街道、社区、网格之间的对比。
- 案件类型、来源渠道、处置状态、风险等级。
- 重复上报、长期未办、处置困难、影响较大的案件。
- 网格员、责任单位、协同部门等处置力量情况。

模板中的口径仅为优先参考，最终必须以数据库实际结构和工具返回结果为准。

## 网格表结构与用途

| 表 | 日期列 | 用途 |
|---|---|---|
| case_list | report_time | 案件原始记录（旧表，区域字段在region，类型字段在case_type，结案判断用pending_step含'结案'） |
| grid_case_record | report_time | 案件原始记录（新表，区域字段在region，类型字段用 COALESCE(NULLIF(case_type2,''), NULLIF(case_type1,''))，结案判断用 case_status 含'结案'/'done'/'finished'/'resolved'/'closed'，社区字段在community） |
| grid_metric_daily | metric_date | 案件日聚合统计 |
| grid_metric_monthly | metric_month | 案件月聚合统计（type=月度口径，含 avg_handle_hours 平均处理时长） |

**查询时必须使用对应表的日期列名和字段名，不要混用。**

### 案件查询优先路径

问"案件数量""结案率""哪个区域案件最多""案件类型分布"等问题时：
1. 优先查 `grid_case_record`（如果该表有数据），使用 `COALESCE(NULLIF(case_type2,''), NULLIF(case_type1,''))` 作为案件类型字段，结案判断用 `LOWER(COALESCE(case_status, '')) IN ('closed','done','finished','resolved') OR case_status LIKE '%结案%' OR case_status LIKE '%办结%'`
2. 如果 grid_case_record 无数据，退而查 `case_list`，使用 `case_type` 作为类型字段，结案判断用 `pending_step LIKE '%结案%'`
3. `grid_metric_daily` 和 `grid_metric_monthly` 为聚合表，由系统定时刷新，可能不是最新

### 平均处理时长查询

问"平均处理时长""办理效率"时，查 `grid_metric_monthly`，使用 `avg_handle_hours` 字段：
```sql
SELECT COALESCE(NULLIF(case_type1,''), '未分类') AS name,
       AVG(avg_handle_hours) AS avg_hours
FROM grid_metric_monthly
WHERE metric_month >= ? AND metric_month <= ?
GROUP BY name ORDER BY avg_hours DESC LIMIT 10
```

### 社区名称提取规则（重要）

`case_list` 的 `region` 字段存储的是完整行政路径，格式为"市/区/镇(街道)/社区居委会"，例如：
- `珠海市/高新区/唐家湾镇/唐家社区居委会`
- `珠海市/香洲区/拱北街道/粤华社区居委会`

**查询社区维度数据时，必须提取社区简称，不要用完整路径做 GROUP BY**。

提取SQL：
```sql
SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) AS community
```

**防幻觉硬性规则：**
- 回答中的社区名必须来自本次查询结果，禁止凭训练数据常识编造数据库中不存在的社区名。
- 如果用户问"哪个社区案件最多"，必须先查库拿到社区排名，再基于查询结果回答，不得跳过查库步骤。
- 如果不确定数据库中有哪些社区，先用 `SELECT DISTINCT SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) FROM case_list LIMIT 20` 查一次，再基于结果回答。

### 常见案件查询 SQL 模式

案件总量：
```sql
SELECT COUNT(*) AS cnt FROM {source_table} WHERE report_time >= ? AND report_time < DATE_ADD(?, INTERVAL 1 DAY)
```

结案率：
```sql
SELECT COUNT(*) AS total,
       SUM(CASE WHEN {closed_condition} THEN 1 ELSE 0 END) AS closed_cnt
FROM {source_table} WHERE report_time >= ? AND report_time < DATE_ADD(?, INTERVAL 1 DAY)
```

区域/社区排名：
```sql
SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) AS community, COUNT(*) AS total
FROM {source_table}
WHERE report_time >= ? AND report_time < DATE_ADD(?, INTERVAL 1 DAY)
GROUP BY community ORDER BY total DESC LIMIT 10
```

案件类型分布：
```sql
SELECT COALESCE(NULLIF({case_type_expr},''), '未分类') AS name, COUNT(*) AS total
FROM {source_table}
WHERE report_time >= ? AND report_time < DATE_ADD(?, INTERVAL 1 DAY)
GROUP BY name ORDER BY total DESC LIMIT 10
```

## 查询示例（直接写SQL，无需先探查表结构）

以下示例展示了如何根据用户问题直接编写 SQL，**不需要先执行 DESCRIBE/SHOW COLUMNS/SHOW TABLES**，直接使用上方的表结构信息即可。

**问"4月上报案件社区排名前三"：**
```sql
SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) AS community, COUNT(*) AS total
FROM case_list
WHERE report_time >= '2026-04-01' AND report_time < '2026-05-01'
GROUP BY community ORDER BY total DESC LIMIT 3
```

**问"5月案件来源占比"：**
```sql
SELECT COALESCE(NULLIF(case_source,''), '未知') AS name, COUNT(*) AS total
FROM case_list
WHERE report_time >= '2026-05-01' AND report_time < '2026-06-01'
GROUP BY name ORDER BY total DESC
```

**问"各社区4月上报量对比"：**
```sql
SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) AS community, COUNT(*) AS total
FROM case_list
WHERE report_time >= '2026-04-01' AND report_time < '2026-05-01'
GROUP BY community ORDER BY total DESC LIMIT 10
```

**问"案件积压主要在哪些环节"：**
```sql
SELECT COALESCE(NULLIF(pending_step,''), '未知') AS name, COUNT(*) AS total
FROM case_list
WHERE pending_step NOT LIKE '%结案%'
  AND report_time BETWEEN ? AND ?
GROUP BY name ORDER BY total DESC
```

**问"积压案件平均已耗时"：**
```sql
SELECT COALESCE(NULLIF(pending_step,''), '未知') AS step,
       COUNT(*) AS cnt,
       AVG(TIMESTAMPDIFF(HOUR, report_time, ?)) AS avg_hours
FROM case_list
WHERE pending_step NOT LIKE '%结案%'
  AND report_time BETWEEN ? AND ?
GROUP BY step ORDER BY cnt DESC
```

**重要口径**：用户问"某月未结案"或"某月积压"时，**必须加 `report_time BETWEEN` 过滤**，限定为该月上报的案件。不加时间过滤会跨月统计所有未结案件，导致数据翻倍。

**问"对比不同案件来源在各环节的时效"：**
先查积压分布：
```sql
SELECT COALESCE(NULLIF(pending_step,''), '未知') AS step,
       COALESCE(NULLIF(case_source,''), '未知') AS source,
       COUNT(*) AS cnt,
       AVG(TIMESTAMPDIFF(HOUR, report_time, update_time)) AS avg_hours
FROM case_list
WHERE pending_step NOT LIKE '%结案%'
GROUP BY step, source ORDER BY step, cnt DESC
```
再查结案时效：
```sql
SELECT COALESCE(NULLIF(case_source,''), '未知') AS source,
       COUNT(*) AS total,
       AVG(TIMESTAMPDIFF(HOUR, report_time, update_time)) AS avg_hours
FROM case_list
WHERE pending_step LIKE '%结案%'
GROUP BY source ORDER BY avg_hours DESC
```

**问"社区案件量TOP3及其主要案件类型"：**
第一步查TOP3社区：
```sql
SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) AS community, COUNT(*) AS total
FROM case_list
WHERE report_time >= '2026-03-01' AND report_time < '2026-06-01'
GROUP BY community ORDER BY total DESC LIMIT 3
```
第二步用结果查各社区案件类型：
```sql
SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(region, '/', -1), '社区', 1) AS community,
       COALESCE(NULLIF(case_type,''), '未分类') AS case_type, COUNT(*) AS total
FROM case_list
WHERE report_time >= '2026-03-01' AND report_time < '2026-06-01'
AND (region LIKE '%唐家社区%' OR region LIKE '%粤华社区%' OR region LIKE '%紫荆社区%')
GROUP BY community, case_type ORDER BY community, total DESC
```
**总共只需 2-3 步即可完成，不需要先探查表结构。**

## 查询原则

- 问案件、结案率、区域排名、重大案件、处置难度时，**优先使用本文件中已列出的表结构和字段名直接编写 SQL**，不要重复执行 DESCRIBE 或 SHOW COLUMNS，避免浪费步骤。
- 只有当本文件的表结构无法覆盖用户问题时，才执行 DESCRIBE 探查。
- 如果数据库已有影响分、难度分、综合分等业务指标，优先使用已有指标。
- 如果没有现成指标，可以根据已查到的业务字段进行综合判断，但必须说明判断依据来自哪些业务信息，不得编造字段或数值。
- 不要把内部 id 作为主要答案，优先展示案件名称、区域名称、网格名称、责任单位等可读信息。
- **回答中绝对禁止出现数据库表名和字段名**，只使用业务化表述，如"案件记录""上报时间""当前环节""案件来源"等。
- **回答中绝对禁止出现数据库表名（如 case_list、grid_case_record）和字段名（如 report_time、update_time、pending_step、case_status），只使用业务化表述，如"案件记录""上报时间""更新时间""当前环节""处置状态"。**

## 常见问题处理

### 案件趋势、结案率、区域排名

- 按用户指定周期统计；未指定周期时，可优先使用本月或最近可用周期，并在回答中说明。
- 多区域、多网格、多类型对比时，应输出 `## 可视化`，使用统一 `chatdb-chart`。
- 单一结案率、单一案件数量等问题，不输出可视化。
- **排名类问题必须同时输出精准结论、特征洞察、洞察分析和可视化四段，不能只输出图表或只输出文字。**

### 重大案件、影响最大案件、处置难度最大案件

- 默认展示 Top1，除非用户明确要求列表。
- 判断依据优先参考影响范围、涉及人数、投诉量、风险等级、事件等级、持续时长、处置耗时、转派次数、协同部门数、未结案状态等。
- 回答必须说明该结果是按影响或处置难度相关口径排序后得到。
- 如果数据不足以分析原因，只能说"从当前数据看"，不能虚构原因。

### 图表数据完整性硬规则

图表 `x` 数组和每个 `series.data` 数组**必须包含查询返回的全部数据点，禁止自行精简、采样或只保留"关键点"**。例如查询返回30天的日趋势，图表必须包含30个日期和30个数值，不能只写5个。

`series.data` 中每个值必须严格对应 `x` 中同一位置的日期/维度，按日期排序后逐行填入，不得错位。

## 图表要求

网格主题统一使用：

```chatdb-chart
{"type":"bar","title":"各区域案件数量对比","x":["区域A","区域B"],"series":[{"name":"案件数量","data":[120,98]}],"table":{"columns":["区域","案件数量"],"rows":[["区域A",120],["区域B",98]]}}
```

图表类型：

- 案件趋势：`line`
- 区域、街道、网格排名：`bar`
- 案件类型、处置状态构成：`pie`
- 明细清单、TopN 案件：`table`

单数值、单占比、Top1 简单结论不输出图表。

## 回答要求

- 先给一句核心结论。
- 再给关键指标。
- 如果是排名，列出 TopN，并说明排序口径。
- 如果是趋势，说明上升、下降或波动，并指出峰值。
- 如果是占比，必须给出数量和比例。
- 如果是重大案件，说明为什么判定为重大，例如数量、影响范围、未结状态、类别严重性。
- 如果数据不足以分析原因，只能说"从当前数据看"，不能虚构原因。

## 广泛问题处理

用户问"网格怎么样""最近案件呢""今天情况如何"等广泛问题时，只查当日案件总量和结案率（从 `case_list` 或 `grid_case_record` 一条 COUNT + SUM 即可），给出概览后列出追问方向：
- 哪个区域案件最多
- 案件类型分布
- 近7天趋势
- 平均处理时长
不要试图一次查完排名+类型+趋势+时长，步骤会耗尽。
