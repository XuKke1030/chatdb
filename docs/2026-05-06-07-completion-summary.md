# 2026-05-06 至 2026-05-07 功能完成情况汇总

## 一、整体概览

这两天主要围绕“问数”和“问答”两条产品线补齐后端接口、前端联动、权限控制、移动端样式，以及车流主题的开发计划。

已完成内容可以归纳为五类：

| 模块 | 完成情况 |
| --- | --- |
| 问数语音输入 | 已完成生产语音转写接口，前端切换到 MediaRecorder + ASR 调用链路 |
| 问答 RAG 能力 | 已完成知识库、检索、问答流式输出、引用定位、会话、热门问题、联网搜索、同步预留等阶段能力 |
| 权限体系 | 已完成问数主题权限、问答知识库权限的前后端对齐，并修复登录态与权限不一致问题 |
| 网格重大案件 | 已完成“重大案件查询展示”和“重大案件分析接口”，支持影响最大、难度最大、综合排序查询 |
| 车流主题 | 已阅读卡口数据推送规则，并输出车流主题开发计划文档 |

## 二、2026-05-06 完成事项

### 1. 问数语音输入能力

完成生产语音转写接口：

- 新增后端接口：`POST /api/v1/speech/transcribe`
- 支持前端上传录音文件，由后端统一转发 ASR 服务
- 支持通过 `CHATDB_ASR_*` 相关配置接入真实 ASR 服务
- 当前本地环境未配置可用 ASR Key 时，不做真实外部 ASR 联调

前端同步完成：

- 问数输入框由浏览器语音能力切换为 `MediaRecorder`
- 录音后通过前端代理接口调用后端语音转写
- 为后续生产 ASR 服务接入保留稳定接口路径

主要涉及文件：

- `api/ai_chat/v1/speech.go`
- `internal/controller/ai_chat/ai_chat_v1_speech.go`
- `D:\code\morphic-main\app\api\question-chat\speech\route.ts`
- `D:\code\morphic-main\components\question-input-bar.tsx`

### 2. 问答开发方案文档

完成问答模块开发计划文档：

- 区分“问答”与“问数”能力边界
- 明确当前不直接对接 AIDGP
- 预留 AIDGP 知识库同步、文档同步、权限同步接口
- 将开发阶段拆分为：知识库、检索、问答、引用、会话、热门问题、联网搜索、同步配置等能力

文档位置：

- `docs/qa-development-plan.md`

### 3. 问答后端阶段能力

完成问答后端主要接口：

| 功能 | 接口 |
| --- | --- |
| 可访问知识库列表 | `GET /api/v1/qa/knowledge-bases` |
| 本地 RAG 检索 | `POST /api/v1/qa/retrieve` |
| 问答流式输出 | `POST /api/v1/qa/chats` |
| 引用定位 | `GET /api/v1/qa/citations/{citationId}` |
| 文档原文跳转 | `GET /api/v1/qa/documents/{id}/view` |
| 热门问题 | `GET /api/v1/qa/popular-questions` |
| 联网搜索预留 | `POST /api/v1/qa/web-search` |
| 会话重置 | `POST /api/v1/qa/sessions/{sessionId}/reset` |
| 知识库同步预留 | `POST /api/v1/qa/sync/knowledge-bases` |
| 文档同步预留 | `POST /api/v1/qa/sync/documents` |
| 权限同步预留 | `POST /api/v1/qa/sync/permissions` |
| 同步状态查询 | `GET /api/v1/qa/sync/status` |

完成数据库初始化表：

- `qa_knowledge_base`
- `qa_document`
- `qa_document_segment`
- `qa_document_permission`
- `qa_session`
- `qa_message`
- `qa_citation`
- `qa_question_stat`
- `qa_sync_task`

### 4. 问答 RAG 闭环

已跑通本地关键词检索版 RAG 闭环：

- 接收用户问题和知识库范围
- 从本地文档段落中召回相关内容
- 组装 Prompt
- 通过 SSE 流式返回回答
- 返回引用信息
- 保存用户消息、助手消息、引用、热门问题统计

当前检索方式为本地关键词评分，后续可升级为数据库全文索引、向量检索或 AIDGP 检索接口。

### 5. 引用定位修复

根据代码检查发现并修复两项问题：

- 无权限知识库请求不会再先落库再报错
- `citation` SSE 事件已返回可定位的 `citationId`
- `qa_citation.message_id` 已能关联到助手回复，支持后续引用跳转

### 6. 问答前端联动

前端新增或同步以下代理接口：

- 知识库列表
- 热门问题
- 问答聊天
- 引用定位
- 文档原文跳转
- 联网搜索
- 会话重置

问答页面已支持：

- 选择文档库
- 发送问题
- 接收 SSE 流式回答
- 展示引用卡片
- 保存和复用会话 ID

主要涉及目录：

- `D:\code\morphic-main\app\api\chatdb`
- `D:\code\morphic-main\app\api\question-chat`
- `D:\code\morphic-main\components\policy-document-chat.tsx`

## 三、2026-05-07 完成事项

### 1. 登录态和权限一致性

完成用户权限接口增强：

- 返回当前用户是否已认证
- 返回用户 ID、用户名、权限等级
- 返回问数主题权限
- 返回问答知识库权限

前端登录门禁同步：

- 不再只依赖 localStorage
- 会向后端校验真实登录态
- 如果后端 Cookie/Token 已失效，会清理本地登录状态

主要涉及文件：

- `api/user/v1/user.go`
- `internal/controller/user/user_v1_user.go`
- `D:\code\morphic-main\components\chatdb-login-gate.tsx`

### 2. 问数主题权限修复

修复“管理端已配置问数权限，但首页主题不可见”的问题：

- 前端会读取后端真实权限
- `zhangsan` 等用户能正确看到已授权的网格、人流、车流主题
- 避免本地缓存登录态和后端权限状态不一致

### 3. 问答知识库权限对齐

修复问答页面和管理端权限配置不一致问题：

- 管理端问答权限来源改为 `qa_knowledge_base`
- 问答页面文档库列表和管理端可配置权限保持同一套知识库
- 兼容旧权限编码映射
- 后端权限判断优先使用管理员配置的用户权限

当前已对齐的知识库示例：

- 政策制度库
- 业务手册库

主要涉及文件：

- `internal/controller/admin/admin_v1.go`
- `internal/controller/user/user_v1_user.go`
- `internal/controller/qa/qa_v1.go`
- `D:\code\morphic-main\app\api\chatdb\permissions\route.ts`

### 4. 移动端宽度和样式统一

根据“项目是手机应用”的要求，完成多处移动端样式调整：

- 问数首页宽度收敛到手机应用宽度
- 问数对话页宽度统一
- 问答页面宽度和问数保持一致
- 问答顶部栏样式对齐问数页面
- “新对话”按钮位置和尺寸统一
- 文本气泡根据文字长短自动匹配宽度
- 联网搜索、深度思考按钮固定宽度，避免点击后布局跳动

主要涉及文件：

- `D:\code\morphic-main\components\question-platform-home.tsx`
- `D:\code\morphic-main\components\ask-number-chat.tsx`
- `D:\code\morphic-main\components\policy-document-chat.tsx`
- `D:\code\morphic-main\components\question-input-bar.tsx`

### 5. 网格重大案件查询展示

完成“网格内影响最大/难度最大/重大案件”查询能力：

前端能力：

- 用户在网格问数中提问重大案件相关问题
- 前端自动识别问题意图
- 直接调用后端重大案件分析接口
- 返回结构化结论和评分依据

后端能力：

- 新增重大案件分析接口
- 支持按照影响度、难度、综合分排序
- 返回 Top1 案件信息和判断依据
- 从数据库案件表中扫描候选案件

接口：

- `GET /api/v1/grid/major-case-analysis`

主要涉及文件：

- `api/ai_chat/v1/chats.go`
- `internal/controller/ai_chat/ai_chat_v1_chats.go`
- `D:\code\morphic-main\app\api\chatdb\grid\major-case-analysis\route.ts`
- `D:\code\morphic-main\components\ask-number-chat.tsx`

### 6. 重大案件评分逻辑优化

针对“回答不准确”的反馈，调整重大案件评分：

- 不再只取最新有限数据导致遗漏早期但重要案件
- 提高地基下沉、路面塌陷、裂缝、建筑结构安全、小区居民等高风险关键词权重
- 影响度、难度、综合分分别计算
- 返回更明确的评分维度和判断依据

已验证目标案例：

- `惠景畅园小区地基下沉及路面塌陷案件`
- 案件编号：`高新社管2025字第7843号`

### 7. 问答初始化与热门问题

完成问答首页初始化功能：

- 首次打开或历史为空时展示欢迎语和能力说明
- 展示 Top3 热门问题
- 点击热门问题后自动填入并发送
- 用户发送问题后，热门问题区域隐藏

后端支持：

- 按用户维度统计高频问题
- 按选中文档库范围返回推荐问题

### 8. 文档库切换

完成问答文档库选择弹窗：

- 输入框上方提供文档库切换入口
- 支持单选、多选、全选
- 当前选中文档库高亮
- 切换后后续问答只检索选中的文档库

后端同步：

- `knowledgeCode` 支持多个知识库编码
- 权限校验覆盖多知识库场景
- 无权限知识库不会下发给前端

### 9. 问答输入增强

完成问答输入区能力：

- 多行文本输入
- 最大高度自适应
- 字数限制
- 发送按钮
- 停止按钮状态
- 联网搜索开关
- 深度思考开关

前后端已打通：

- 前端传递 `webSearch`
- 前端传递 `deepThinking`
- 后端 SSE 返回 `thinking`、`web_search`、`retrieval`、`citation`、`message` 等事件

### 10. 联网搜索与深度思考预留

联网搜索：

- 前端开关已完成
- 后端接口和 SSE 事件已完成
- 当前默认配置中 `qa.webSearch.enabled` 可控制是否启用真实搜索
- 真实外部搜索服务后续可通过配置接入

深度思考：

- 前端开关已完成
- 后端会根据开关增强 Prompt
- SSE 会返回思考阶段提示
- 不输出模型内部完整推理链路，只展示用户可见的过程摘要

### 11. 车流主题开发计划

已阅读 `卡口数据推送规则.docx`，梳理车流数据接收规则：

- MQTT Broker：`iot-mqtt.qi-cloud.com:1883`
- Topic：`zh-iot-jinshan-kakou`
- Client ID：`jinshan_随机字符`
- Username：`jinshan`
- Password：由对接方提供
- 主要字段包括：`cd`、`date`、`DeviceID`、`SnapshotTime`、`PlateChar`、`DeviceName`

已输出车流开发计划文档：

- `docs/traffic-flow-development-plan.md`

计划内容覆盖：

- MQTT 车流数据接收
- 原始数据落库
- 车牌归属识别
- 港澳车牌识别
- 车流聚合指标
- 按车牌、日期、卡口、节假日维度统计
- 前端车流图表渲染

## 四、当前可以测试的内容

### 1. 问数主题权限

可测试：

- 不同用户登录后首页是否只显示有权限的问数主题
- `zhangsan` 是否能看到网格、人流、车流
- 取消某个主题权限后，前端是否实时不展示

### 2. 问数语音输入

可测试：

- 点击麦克风录音
- 浏览器是否能生成音频
- 前端是否调用 `/api/v1/speech/transcribe`
- 后端未配置 ASR 时是否返回明确错误
- 配置 ASR 后是否能返回真实转写文本

### 3. 网格重大案件

可测试问题：

- `网格内影响最大的案件是什么`
- `网格内难度最大的案件是什么`
- `网格内重大案件有哪些`

预期：

- 返回案件名称
- 返回案件编号
- 返回影响度/难度/综合评分
- 返回判断依据

### 4. 问答知识库权限

可测试：

- 管理端给用户配置问答知识库权限
- 问答页面只展示该用户有权限的文档库
- 选择无权限知识库时后端拒绝访问
- 多选知识库后检索范围正确变化

### 5. 问答 RAG

可测试：

- 选择文档库后提问
- 后端是否召回本地文档段落
- SSE 是否流式返回回答
- 引用卡片是否展示
- 点击引用是否能定位文档信息
- 会话重置是否生效

### 6. 热门问题

可测试：

- 首次进入问答页是否展示欢迎语
- 是否展示 Top3 热门问题
- 点击热门问题是否自动发送
- 发送问题后热门问题是否隐藏

### 7. 联网搜索和深度思考

可测试：

- 打开深度思考后是否出现思考阶段提示
- 打开联网搜索后是否出现联网搜索事件
- 未启用真实搜索配置时是否走可控降级

## 五、当前待注意事项

1. ASR 真实转写依赖外部服务配置，需提供可用的 `CHATDB_ASR_*` 配置后才能完整联调。

2. 问答当前是本地关键词检索，适合先跑通闭环；大规模知识库建议后续升级为数据库全文索引、向量检索或 AIDGP 检索。

3. 联网搜索目前已有接口和前端开关，但真实外部搜索服务需要后续配置。

4. 深度思考当前是提示增强和阶段展示，不应展示模型内部完整推理链。

5. 车流主题目前完成开发计划，还未正式开发 MQTT 接收、落库和统计接口。

6. AIDGP 当前只做预留接口和同步任务模型，尚未对接真实 AIDGP 服务。

## 六、文档与代码位置

### 后端项目

根目录：

- `D:\code\ChatDB-master`

主要新增/修改模块：

- `api/ai_chat`
- `api/qa`
- `api/user`
- `internal/controller/ai_chat`
- `internal/controller/qa`
- `internal/controller/admin`
- `internal/controller/user`
- `internal/model`
- `config/config.yaml`

### 前端项目

根目录：

- `D:\code\morphic-main`

主要新增/修改模块：

- `app/api/chatdb`
- `app/api/question-chat`
- `components/question-platform-home.tsx`
- `components/ask-number-chat.tsx`
- `components/policy-document-chat.tsx`
- `components/question-input-bar.tsx`
- `components/chatdb-login-gate.tsx`

### 已输出开发计划文档

- `docs/qa-development-plan.md`
- `docs/traffic-flow-development-plan.md`

