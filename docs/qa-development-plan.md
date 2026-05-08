# 问答端开发计划

## 1. 定位与边界

问答端面向文档知识库问答，核心能力是知识库选择、RAG 检索、流式回答、引用溯源、会话追问和权限校验。

它与问数端分开建设：

- 问数端：面向网格、人流、车流等结构化数据，接口主线为 `/api/v1/chats`，依赖 SQL、主题 prompt 和图表数据。
- 问答端：面向政策文档、制度文件、知识库资料，接口主线为 `/api/v1/qa/*`，依赖文档库、检索、引用和文档权限。

当前阶段暂不对接 AIDGP，先使用本地知识库和权限表完成闭环，同时预留 AIDGP 适配层。

## 当前进度

- 阶段 1 已完成：已新增 `/api/v1/qa` 命名空间、问答端本地表、知识库列表和会话重置接口。
- 阶段 2 已完成：已实现 `POST /api/v1/qa/retrieve`，使用本地关键词检索跑通知识库片段召回，并在检索前后做权限过滤。
- 阶段 3 已完成：已实现 `POST /api/v1/qa/chats`，支持会话保存、本地 RAG 召回、引用事件和 LLM SSE 流式输出。
- 阶段 4 已完成：已实现 `GET /api/v1/qa/citations/{citationId}` 和 `GET /api/v1/qa/documents/{id}/view`，引用详情和文档查看都会重新校验文档权限。
- 阶段 5 已完成：已实现 `GET /api/v1/qa/popular-questions`，按当前用户、知识库和时间窗口返回热门问题，无统计数据时返回默认推荐问题。
- 阶段 6 已完成：已实现 `POST /api/v1/qa/web-search`，当前只返回联网搜索配置状态和占位响应，不发起外部供应商调用。
- 阶段 7 已完成：已实现 `POST /api/v1/qa/sync/knowledge-bases`、`POST /api/v1/qa/sync/documents`、`POST /api/v1/qa/sync/permissions`、`GET /api/v1/qa/sync/status`，当前记录同步任务并返回本地/AIDGP 占位状态。
- AIDGP 仍未真实对接：当前只保留配置、`source_provider`、`external_id`、`sync_version` 等字段和同步任务表，后续由适配层替换实现。

## 2. 总体架构

```txt
问答前端
  -> /api/v1/qa/knowledge-bases
  -> /api/v1/qa/retrieve
  -> /api/v1/qa/chats
  -> /api/v1/qa/citations/{id}
  -> /api/v1/qa/popular-questions
  -> /api/v1/qa/web-search
  -> /api/v1/qa/sync/*
  -> /api/v1/qa/sessions/{id}/reset

问答后端
  -> 本地知识库表
  -> 文档段落表
  -> 权限表
  -> 会话/消息/引用表
  -> LLM 与 RAG 编排

后续 AIDGP
  -> qa_sync provider = aidgp
  -> 同步知识库、文档元数据、权限映射
```

## 3. 阶段计划

### 阶段 1：问答端基础框架

目标：先完成独立 `/api/v1/qa` 命名空间和基础数据模型。

开发内容：

- 新增本地表：
  - `qa_knowledge_base`
  - `qa_document`
  - `qa_document_segment`
  - `qa_document_permission`
  - `qa_session`
  - `qa_message`
  - `qa_citation`
  - `qa_question_stat`
  - `qa_sync_task`
- 新增接口：
  - `GET /api/v1/qa/knowledge-bases`
  - `POST /api/v1/qa/sessions/{sessionId}/reset`
- 本地知识库权限先复用当前用户权限体系，并预留 `external_id`、`source_provider` 等 AIDGP 字段。

验收：

- 问答端接口与问数端接口分离。
- 当前用户只能看到有权限的知识库。
- 可以重置指定问答会话。

### 阶段 2：RAG 检索闭环

目标：实现问题到文档片段召回。

接口：

- `POST /api/v1/qa/retrieve`

能力：

- 按 `knowledgeCode` 限定范围。
- 本阶段先用关键词/LIKE 检索，后续替换为向量检索。
- 返回文档 ID、标题、片段 ID、内容、页码、锚点和分数。
- 检索前后都做权限过滤。

### 阶段 3：问答流式输出

目标：实现问答端专用 SSE。

接口：

- `POST /api/v1/qa/chats`

SSE 事件：

- `start`：返回 `sessionId`
- `retrieval`：返回命中文档片段摘要
- `citation`：返回引用来源
- `message`：流式正文
- `end`：结束

### 阶段 4：引用来源定位

目标：点击引用可以定位原文。

接口：

- `GET /api/v1/qa/citations/{citationId}`
- `GET /api/v1/qa/documents/{id}/view`

要求：

- 引用详情必须再次校验权限。
- 返回文件名、页码、段落锚点和预览文本。

### 阶段 5：热门问题统计

接口：

- `GET /api/v1/qa/popular-questions?knowledgeCode=xxx&limit=3`

能力：

- 按用户、知识库和时间窗口统计高频问题。
- 无数据时返回默认推荐问题。

### 阶段 6：联网搜索补充

接口：

- `POST /api/v1/qa/web-search`

当前只预留接口和配置，不强依赖供应商。

### 阶段 7：AIDGP 对接预留

预留配置：

```yaml
qa:
  sync:
    provider: "local" # local | aidgp
    aidgp:
      baseUrl: ""
      appKey: ""
      appSecret: ""
```

预留接口：

- `POST /api/v1/qa/sync/knowledge-bases`
- `POST /api/v1/qa/sync/documents`
- `POST /api/v1/qa/sync/permissions`
- `GET /api/v1/qa/sync/status`

本地阶段 `provider=local`，同步接口返回未启用外部同步；AIDGP 阶段只替换同步实现，不改前端主流程。

## 4. 数据表设计

核心表：

```txt
qa_knowledge_base
qa_document
qa_document_segment
qa_document_permission
qa_session
qa_message
qa_citation
qa_question_stat
qa_sync_task
```

预留字段：

- `external_id`
- `source_provider`
- `sync_version`
- `last_sync_time`
- `permission_hash`

## 5. 优先级

P0：

- `/api/v1/qa/knowledge-bases`
- `/api/v1/qa/retrieve`
- `/api/v1/qa/chats`
- `/api/v1/qa/sessions/{id}/reset`
- 基础权限过滤

P1：

- 引用定位
- 热门问题统计
- 文档查看接口

P2：

- 联网搜索
- 同步任务状态
- AIDGP 适配层

## 6. 完成判定

- 能独立访问问答端页面。
- 能选择有权限的文档库。
- 能输入问题并召回文档片段。
- 能流式生成回答。
- 能保存会话并追问。
- 能点击引用定位原文。
- 无权限文档全链路不可见。
- 后续接 AIDGP 时不改前端主流程。
