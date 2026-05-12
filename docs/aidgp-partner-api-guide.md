# AIDGP 对接文档（交给对接方）

适用对象：AIDGP 平台接口负责人  
调用方系统：ChatDB 问答问数移动端后端  
对接目标：ChatDB 在问数和问答场景中调用 AIDGP 获取知识库、文档、权限、网格、人流、车流数据，并支持上线验收与问题追踪。

## 1. 对接范围

请 AIDGP 提供以下能力：

1. 获取访问 token。
2. 查询知识库列表。
3. 查询文档列表、文档正文分段、附件解析结果。
4. 查询用户/组织/角色对知识库和文档的权限。
5. 查询网格业务数据。
6. 查询人流业务数据。
7. 查询车流业务数据。
8. 支持按时间增量同步、分页、幂等标识、错误码和链路追踪。

## 2. 调用方信息

| 项目 | 内容 |
|---|---|
| 系统名称 | ChatDB 问答问数平台 |
| 调用环境 | 测试环境、生产环境 |
| 数据用途 | 移动端问数、问答检索、后台同步 |
| 调用方式 | HTTPS JSON API |
| 鉴权方式 | `client_credentials` 获取 token，业务接口使用 Bearer token |
| 字符编码 | UTF-8 |
| 时间格式 | `yyyy-MM-dd HH:mm:ss`，建议统一 Asia/Shanghai |
| 幂等字段 | `requestId`、业务 `externalId`、数据 `updateTime` |

## 3. Token 获取

### 3.1 接口

```http
POST /openapi/oauth/token
Content-Type: application/json
```

请求：

```json
{
  "appKey": "由 AIDGP 分配",
  "appSecret": "由 AIDGP 分配",
  "grantType": "client_credentials"
}
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "accessToken": "token string",
    "tokenType": "Bearer",
    "expiresIn": 7200
  }
}
```

要求：

- `expiresIn` 单位为秒。
- token 过期前 120 秒调用方会主动刷新。
- token 失效时业务接口返回 401，并提供明确错误码。

### 3.2 业务接口请求头

```http
Authorization: Bearer {accessToken}
X-App-Key: {appKey}
X-Request-Id: {uuid}
X-Timestamp: {unix_ms}
Content-Type: application/json
```

如贵方要求签名，请提供：

- 待签名字符串拼接规则。
- 摘要算法。
- HMAC 或 RSA 方案。
- 重放窗口时间。
- 示例请求和示例签名。

## 4. 统一响应格式

请所有接口统一返回：

```json
{
  "code": 0,
  "message": "success",
  "requestId": "uuid",
  "data": {}
}
```

分页响应：

```json
{
  "code": 0,
  "message": "success",
  "requestId": "uuid",
  "data": {
    "list": [],
    "page": 1,
    "pageSize": 100,
    "total": 1000,
    "hasMore": true,
    "nextCursor": "optional cursor"
  }
}
```

## 5. 错误码要求

| code | HTTP | 含义 | 调用方处理 |
|---:|---:|---|---|
| 0 | 200 | 成功 | 正常解析 |
| 400001 | 400 | 参数错误 | 记录失败日志，不重试 |
| 401001 | 401 | token 缺失 | 重新获取 token 后重试一次 |
| 401002 | 401 | token 过期 | 重新获取 token 后重试一次 |
| 403001 | 403 | 无接口权限 | 停止调用，联系配置权限 |
| 404001 | 404 | 资源不存在 | 记录 skipped |
| 429001 | 429 | 限流 | 按 `Retry-After` 重试 |
| 500001 | 500 | 平台内部错误 | 重试，仍失败则任务失败 |
| 504001 | 504 | 超时 | 重试，仍失败则降级 |

## 6. 知识库接口

### 6.1 查询知识库

```http
POST /openapi/chatdb/knowledge-bases
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "page": 1,
  "pageSize": 100,
  "updatedAfter": "2026-05-01 00:00:00"
}
```

响应 data.list 字段：

```json
[
  {
    "knowledgeCode": "policy",
    "knowledgeName": "政策制度库",
    "description": "制度、规范、政策文件",
    "enabled": true,
    "sort": 10,
    "externalId": "kb-001",
    "syncVersion": "202605120001",
    "updateTime": "2026-05-12 10:00:00"
  }
]
```

## 7. 文档接口

### 7.1 查询文档列表

```http
POST /openapi/chatdb/documents
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "knowledgeCode": "policy",
  "documentType": "",
  "updatedAfter": "2026-05-01 00:00:00",
  "page": 1,
  "pageSize": 100
}
```

响应字段：

```json
{
  "documentId": "doc-001",
  "knowledgeCode": "policy",
  "title": "某管理办法",
  "fileName": "某管理办法.pdf",
  "fileType": "pdf",
  "documentType": "阅件|办件",
  "status": "active|abolished|draft",
  "version": "v3",
  "effectiveDate": "2026-01-01",
  "expireDate": "",
  "supersededBy": "",
  "syncVersion": "202605120001",
  "updateTime": "2026-05-12 10:00:00"
}
```

### 7.2 查询文档分段

```http
POST /openapi/chatdb/document-segments
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "documentId": "doc-001",
  "includeAttachments": true,
  "page": 1,
  "pageSize": 500
}
```

响应字段：

```json
{
  "segmentId": "seg-001",
  "documentId": "doc-001",
  "segmentIndex": 1,
  "content": "可检索正文或附件解析文本",
  "page": 1,
  "anchor": "p1-s1",
  "attachmentId": "",
  "attachmentName": "",
  "updateTime": "2026-05-12 10:00:00"
}
```

要求：

- 正文和附件解析结果都必须可检索。
- 每段内容建议不超过 2000 字。
- 废止/过期文件请明确 `status`，调用方会默认过滤。

## 8. 权限接口

### 8.1 查询用户权限

```http
POST /openapi/chatdb/permissions
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "userId": "zhangsan",
  "updatedAfter": "2026-05-01 00:00:00",
  "page": 1,
  "pageSize": 100
}
```

响应字段：

```json
{
  "subjectType": "user|org|role",
  "subjectId": "zhangsan",
  "knowledgeCode": "policy",
  "documentId": "",
  "enabled": true,
  "permissionHash": "sha256",
  "updateTime": "2026-05-12 10:00:00"
}
```

要求：

- 支持按用户直接查询最终权限，或提供组织/角色关系和权限规则。
- 无权限文档不得出现在调用方的知识库列表、检索结果、引用详情和原文查看中。

## 9. 网格数据接口

### 9.1 查询网格指标/明细

```http
POST /openapi/chatdb/grid/query
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "metric": "case_count|close_rate|major_case|ranking|trend|time_distribution",
  "dateFrom": "2026-04-01",
  "dateTo": "2026-04-30",
  "region": "高新区",
  "street": "",
  "community": "",
  "gridName": "",
  "caseType": "",
  "groupBy": "region|community|grid|caseType|day|hour",
  "page": 1,
  "pageSize": 100
}
```

响应建议：

```json
{
  "summary": {
    "totalCases": 1280,
    "closedCases": 1200,
    "closeRate": 0.9375
  },
  "list": [
    {
      "caseId": "case-001",
      "caseNumber": "GX2026040001",
      "caseName": "后环社区占道经营",
      "caseType1": "事件",
      "caseType2": "占道经营",
      "source": "12345",
      "region": "高新区",
      "street": "唐家湾镇",
      "community": "后环社区",
      "gridName": "第一网格",
      "responsibilityUnit": "综合执法局",
      "reportTime": "2026-04-12 09:10:00",
      "closeTime": "2026-04-13 15:20:00",
      "status": "closed",
      "pendingStep": "结案",
      "impactScore": 80,
      "difficultyScore": 70,
      "involvedPeople": 10,
      "description": "案件描述",
      "updateTime": "2026-05-12 10:00:00"
    }
  ],
  "series": [
    {
      "name": "后环社区",
      "total": 120,
      "closed": 110,
      "closeRate": 0.9167
    }
  ]
}
```

## 10. 人流数据接口

### 10.1 查询人流指标

```http
POST /openapi/chatdb/population/query
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "dateFrom": "2026-05-06",
  "dateTo": "2026-05-12",
  "region": "高新区",
  "street": "",
  "community": "",
  "gridName": "",
  "metrics": ["inCount", "outCount", "netInCount", "floatingPopulationCount"],
  "groupBy": "day|hour|region|grid|residentType",
  "compare": {
    "type": "holiday|year_on_year|previous_period",
    "baselineDateFrom": "2025-05-06",
    "baselineDateTo": "2025-05-12"
  },
  "page": 1,
  "pageSize": 100
}
```

响应建议：

```json
{
  "summary": {
    "inCount": 120000,
    "outCount": 110000,
    "netInCount": 10000,
    "total": 230000,
    "floatingPopulationCount": 32000
  },
  "series": [
    {
      "name": "2026-05-12",
      "inCount": 18000,
      "outCount": 16000,
      "netInCount": 2000,
      "floatingPopulationCount": 4500
    }
  ]
}
```

流动人口口径请明确：

- 户籍地不在本地。
- 手机运营商当前驻留地在本地。
- 数据必须脱敏聚合，不能返回个人身份信息或明细轨迹。

## 11. 车流数据接口

### 11.1 查询车流指标

```http
POST /openapi/chatdb/traffic/query
Authorization: Bearer {token}
```

请求：

```json
{
  "requestId": "uuid",
  "dateFrom": "2026-05-12 00:00:00",
  "dateTo": "2026-05-12 23:59:59",
  "region": "高新区",
  "gateId": "",
  "plate": "",
  "metrics": ["total", "inCount", "outCount", "hkMacauCount", "foreignCount", "stayDuration"],
  "groupBy": "day|hour|gate|direction|plateRegion|originCity",
  "includeStayDuration": true,
  "page": 1,
  "pageSize": 100
}
```

响应建议：

```json
{
  "summary": {
    "total": 98000,
    "inCount": 51000,
    "outCount": 47000,
    "hkMacauCount": 4200,
    "hkMacauRatio": 0.0429,
    "mainlandCount": 93800,
    "foreignCount": 18000
  },
  "series": [
    {
      "name": "金鼎卡口",
      "gateId": "gate-001",
      "total": 12000,
      "inCount": 6500,
      "outCount": 5500,
      "hkMacauCount": 600
    }
  ],
  "stayDuration": [
    {
      "bucket": "<1h",
      "count": 1200,
      "ratio": 0.45
    },
    {
      "bucket": "1-3h",
      "count": 900,
      "ratio": 0.34
    },
    {
      "bucket": ">=3h",
      "count": 560,
      "ratio": 0.21
    }
  ]
}
```

如提供明细，请包含：

```json
{
  "recordId": "traffic-001",
  "gateId": "gate-001",
  "gateName": "金鼎卡口",
  "cameraIp": "10.0.0.1",
  "plateNo": "粤C12345",
  "direction": 0,
  "snapshotTime": "2026-05-12 10:20:30",
  "plateRegionType": "mainland|hong_kong|macau|cross_border|foreign|unknown",
  "isHkMacau": false,
  "originProvince": "广东",
  "originCity": "珠海",
  "updateTime": "2026-05-12 10:21:00"
}
```

## 12. 性能与限流要求

| 场景 | 建议要求 |
|---|---|
| token 接口 | P95 < 500ms |
| 指标查询 | P95 < 1500ms |
| 分页同步 | 单页最大 1000 条 |
| 限流 | 返回 429 和 `Retry-After` |
| 超时 | 服务端 10 秒内返回 |
| 增量查询 | 支持 `updatedAfter` 或 cursor |

## 13. 验证方式

### 13.1 联调前自测

请对接方提供以下测试数据：

- 一个有网格、人流、车流权限的测试账号。
- 一个只有人流权限的测试账号。
- 一个无车流权限的测试账号。
- 至少 7 天车流和人流数据。
- 至少 1 个月网格案件数据。
- 至少 2 个知识库、5 篇文档、每篇 3 个以上分段。

### 13.2 Token 验证

```bash
curl -X POST "https://aidgp-test.example.com/openapi/oauth/token" \
  -H "Content-Type: application/json" \
  -d '{"appKey":"***","appSecret":"***","grantType":"client_credentials"}'
```

通过标准：

- HTTP 200。
- `code=0`。
- `data.accessToken` 非空。
- `data.expiresIn > 300`。

### 13.3 业务接口验证

```bash
curl -X POST "https://aidgp-test.example.com/openapi/chatdb/traffic/query" \
  -H "Authorization: Bearer ${AIDGP_TOKEN}" \
  -H "Content-Type: application/json" \
  -H "X-Request-Id: test-traffic-001" \
  -d '{"requestId":"test-traffic-001","dateFrom":"2026-05-12 00:00:00","dateTo":"2026-05-12 23:59:59","groupBy":"gate","page":1,"pageSize":10}'
```

通过标准：

- HTTP 200。
- `code=0`。
- `requestId` 原样返回。
- `summary.total` 与 `series` 可对账。

### 13.4 对账标准

- 同一时间范围，AIDGP 总量与 ChatDB 同步后的本地总量差异为 0。
- 按天、按卡口/区域分组数据一致。
- 无权限账号返回 403 或空权限数据，不返回敏感内容。
- 重复请求同一分页不会产生重复数据。
- 所有中文字段均为 UTF-8，无乱码。

## 14. 需要对接方确认的问题

1. Token 接口路径、请求字段、响应字段是否与本文一致。
2. 是否要求接口签名，如需要请提供签名规范。
3. 网格、人流、车流是否有现成指标接口，还是只提供明细。
4. 人流“流动人口”的准确定义、统计粒度、脱敏要求。
5. 车流“驻留时长”的计算由 AIDGP 提供还是 ChatDB 本地计算。
6. 文档附件解析结果是否由 AIDGP 提供。
7. 文件版本状态字段如何表示废止、修订、最新有效。
8. 权限是按用户、组织、角色还是三者组合返回。
9. 测试环境、生产环境域名和 IP 白名单要求。
10. QPS、分页上限、超时、限流和 SLA。

