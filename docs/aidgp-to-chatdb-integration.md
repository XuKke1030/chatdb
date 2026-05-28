# AIDGP 对接 ChatDB 说明

适用方向：AIDGP 作为被调用方配合 ChatDB 主动拉取数据  
结论：AIDGP 数据不主动推送到 ChatDB，车流、人流、网格和知识库数据均由 ChatDB 按接口主动获取。

## 1. 对接边界

AIDGP 不需要主动调用 ChatDB 推送业务数据。AIDGP 需要提供稳定的查询、分页、增量同步和鉴权能力。

ChatDB 主动调用 AIDGP 获取：

- 车流数据。
- 人流数据。
- 网格数据。
- 知识库列表。
- 文档列表。
- 文档正文分段。
- 附件解析结果。
- 文档版本状态。
- 可选：AIDGP 侧文档权限辅助数据。

## 2. AIDGP 需要提供的能力

### 2.1 鉴权接口

```http
POST /openapi/oauth/token
Content-Type: application/json
```

请求：

```json
{
  "appKey": "CHATDB_APP_KEY",
  "appSecret": "CHATDB_APP_SECRET",
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

### 2.2 业务查询接口

AIDGP 需提供以下接口，供 ChatDB 主动调用：

```http
POST /openapi/chatdb/traffic/query
POST /openapi/chatdb/population/query
POST /openapi/chatdb/grid/query
POST /openapi/chatdb/knowledge-bases
POST /openapi/chatdb/documents
POST /openapi/chatdb/document-segments
POST /openapi/chatdb/document-view
POST /openapi/chatdb/knowledge-permissions
POST /openapi/chatdb/document-relations
```

### 2.3 文档字段对齐要求

#### 2.3.1 文档类型 doc_type

AIDGP 文档字段 `documentType` 需按以下枚举值推送，与 ChatDB 本地 `qa_knowledge_base.doc_type` 对齐：

| 枚举值 | 含义 | 说明 |
|---|---|---|
| `policy` | 政策制度 | 政策、制度、规范类文档 |
| `manual` | 业务手册 | 业务流程、操作手册类文档 |
| `form` | 表单模板 | 常用表单、申请模板 |
| `rule` | 规则标准 | 行业标准、技术规范 |
| `case` | 案例汇编 | 典型案例、经验总结 |

AIDGP 如有其他分类，需在推送前映射到上述 5 类；无法映射的归入 `policy`。

#### 2.3.2 文档版本字段映射

| AIDGP 字段 | ChatDB 本地字段 | 说明 |
|---|---|---|
| `version` | `title_group` | 同名文件不同版本共享同一 `title_group`，用于版本分组 |
| `supersededBy` | `repealed_by` | 废止该文档的新版本标题或 ID |
| `effectiveDate` | `effective_date` | 生效日期，格式 YYYY-MM-DD |
| (无) | `repeal_date` | ChatDB 根据 `supersededBy` 关联查询自动填充 |

要求：AIDGP 推送同一文件的不同版本时，`version` 字段保持一致（如文件主标题或编号），以便 ChatDB 按版本分组展示。

#### 2.3.3 文档关联

AIDGP 需提供文档间关联关系（引用、废止、补充等），通过 `document-relations` 接口返回。关联类型：

| rel_type | 含义 | 说明 |
|---|---|---|
| `reference` | 引用 | 文档 A 引用了文档 B 的内容 |
| `supplement` | 补充 | 文档 B 是文档 A 的补充说明 |
| `repeal` | 废止 | 文档 A 废止了文档 B |
| `related` | 相关 | 两文档主题相关，可供交叉参考 |

## 3. ChatDB 主动拉取流程

```mermaid
flowchart TD
  T["ChatDB 定时任务/后台手动同步"] --> A["获取 AIDGP token"]
  A --> B["按业务类型调用 AIDGP 查询接口"]
  B --> C["分页/游标拉取"]
  C --> D["字段映射、清洗、去重"]
  D --> E["写入本地明细表/知识库表"]
  E --> F["刷新本地聚合表/检索索引"]
  F --> G["问数/问答使用本地数据"]
```

实时问数仅在必要时由 ChatDB 主动查询 AIDGP：

```mermaid
flowchart TD
  Q["用户问数"] --> P["ChatDB 权限校验"]
  P --> C{"本地聚合/缓存可用?"}
  C -->|可用| L["查本地聚合表"]
  C -->|需要实时| R["ChatDB 调 AIDGP 实时查询"]
  R --> W["写短期缓存"]
  W --> A["回答"]
  L --> A
  R -->|失败| F["使用最近同步数据或返回数据源不可用"]
```

## 4. 请求头与签名

ChatDB 调用 AIDGP 时建议统一使用：

```http
Authorization: Bearer {accessToken}
X-App-Key: {appKey}
X-Request-Id: {uuid}
X-Timestamp: {unix_ms}
X-Nonce: {random}
X-Signature: {signature}
Content-Type: application/json; charset=utf-8
```

签名建议：

```text
bodySha256 = HEX(SHA256(rawBody))
canonical = method + "\n" + pathWithQuery + "\n" + timestamp + "\n" + nonce + "\n" + bodySha256
signature = BASE64(HMAC-SHA256(appSecret, canonical))
```

AIDGP 验签要求：

- `X-Timestamp` 与服务端时间偏差不超过 5 分钟。
- `X-Nonce` 在 5-10 分钟内不能重复。
- 使用原始 body 字节计算摘要。
- token、appSecret、签名原文不得写入业务日志。

如双方要求国密，可替换为：

- SM3 摘要。
- SM2 签名验签。
- SM4-GCM 字段级加密。

## 5. 分页与增量同步要求

AIDGP 所有列表接口建议支持：

```json
{
  "requestId": "uuid",
  "updatedAfter": "2026-05-18 00:00:00",
  "page": 1,
  "pageSize": 500,
  "cursor": ""
}
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "requestId": "uuid",
  "data": {
    "list": [],
    "page": 1,
    "pageSize": 500,
    "total": 1000,
    "hasMore": true,
    "nextCursor": "cursor-value"
  }
}
```

要求：

- 支持 `updatedAfter` 增量同步。
- 支持 `syncVersion` 或 `updateTime` 判断幂等。
- 支持分页或 cursor。
- 同一请求参数重复调用应返回一致结果或等价结果。
- 删除、废止、禁用数据必须能通过状态字段同步给 ChatDB。
- 文档关联关系（引用、废止、补充等）必须能通过 `document-relations` 接口返回。

### 5.1 分段附件要求

AIDGP 文档分段字段需包含 `attachmentId` 和 `attachmentName`，用于标识该段落来源于哪个附件（如 PDF、Word、图片）。ChatDB 侧将在 `qa_document_segment` 表存储这两个字段，供前端展示附件来源和下载链接。

若 AIDGP 已将附件内容解析后直接放入 segment `content`，则 `attachmentId`/`attachmentName` 可为空，ChatDB 仍能正常检索。
