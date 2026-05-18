package qa

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	v1 "ai-chat-sql/api/qa/v1"
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/aidgp"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
)

func (c *ControllerV1) KnowledgeBases(ctx context.Context, req *v1.KnowledgeBasesReq) (res *v1.KnowledgeBasesRes, err error) {
	userId := currentUserId(ctx)
	records, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).
		Where("enabled = ?", 1).
		OrderAsc("sort").
		All()
	if err != nil {
		return nil, err
	}

	adminScoped, adminHasPerm, adminEnabled, qaAllowed := batchKnowledgePermissions(ctx, userId)

	list := make([]v1.KnowledgeBaseItem, 0, len(records))
	for _, record := range records {
		code := record["code"].String()
		if !checkKnowledgeAccess(code, adminScoped, adminHasPerm, adminEnabled, qaAllowed) {
			continue
		}
		list = append(list, v1.KnowledgeBaseItem{
			Code:           code,
			Name:           record["name"].String(),
			Description:    record["description"].String(),
			Enabled:        record["enabled"].Int() == 1,
			SourceProvider: record["source_provider"].String(),
			ExternalId:     record["external_id"].String(),
			UpdateTime:     record["update_time"].Int(),
		})
	}
	return &v1.KnowledgeBasesRes{List: list}, nil
}

func (c *ControllerV1) Retrieve(ctx context.Context, req *v1.RetrieveReq) (res *v1.RetrieveRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return nil, gerror.New("问题不能为空")
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}
	items, err := retrieveQaItems(ctx, userId, question, strings.TrimSpace(req.KnowledgeCode), topK)
	if err != nil {
		return nil, err
	}
	return &v1.RetrieveRes{List: items}, nil
}

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	totalStart := time.Now()
	stageStart := totalStart
	logStage := func(stage string) {
		consts.Logger.Infof(ctx, "perf qa_chat stage=%s knowledgeCode=%s sessionId=%s costMs=%d totalMs=%d", stage, strings.TrimSpace(req.KnowledgeCode), strings.TrimSpace(req.SessionId), time.Since(stageStart).Milliseconds(), time.Since(totalStart).Milliseconds())
		stageStart = time.Now()
	}
	defer func() {
		consts.Logger.Infof(ctx, "perf qa_chat stage=total knowledgeCode=%s sessionId=%s costMs=%d", strings.TrimSpace(req.KnowledgeCode), strings.TrimSpace(req.SessionId), time.Since(totalStart).Milliseconds())
	}()
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil, gerror.New("问题不能为空")
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}
	knowledgeCode := strings.TrimSpace(req.KnowledgeCode)
	items, err := retrieveQaItems(ctx, userId, message, knowledgeCode, topK)
	if err != nil {
		return nil, err
	}
	logStage("retrieval")
	sessionId, err := ensureQaSession(ctx, userId, strings.TrimSpace(req.SessionId), knowledgeCode, message)
	if err != nil {
		return nil, err
	}
	req.SessionId = sessionId
	logStage("session")
	storedHistory, err := loadQaSessionHistory(ctx, userId, sessionId, 12)
	if err != nil {
		return nil, err
	}
	history := mergeQaHistory(storedHistory, req.History)
	logStage("history")
	if _, err = appendQaMessage(ctx, userId, sessionId, knowledgeCode, "user", message); err != nil {
		return nil, err
	}
	logStage("save_user_message")
	if err = upsertQaQuestionStat(ctx, userId, knowledgeCode, message); err != nil {
		consts.Logger.Errorf(ctx, "更新问答热门问题统计失败: %s", err.Error())
	}
	logStage("question_stat")
	citations, err := createQaCitations(ctx, userId, sessionId, items)
	if err != nil {
		return nil, err
	}
	logStage("citation")

	respChan := make(chan any)
	g.Go(ctx, func(ctx context.Context) {
		streamQaChat(ctx, req, userId, sessionId, knowledgeCode, message, history, items, citations, respChan, totalStart)
	}, func(ctx context.Context, exception error) {
		close(respChan)
		consts.Logger.Error(ctx, exception)
	})

	r := g.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	logStage("first_sse_ready")

	var jsonData []byte
	for v := range respChan {
		jsonData, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
		r.Response.Writef("data: %s\n\n", jsonData)
		r.Response.Flush()
	}
	return nil, nil
}

func (c *ControllerV1) CitationDetail(ctx context.Context, req *v1.CitationDetailReq) (res *v1.CitationDetailRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	citation, err := g.DB("master").Model("qa_citation").Ctx(ctx).
		Fields("id, session_id, user_id, message_id, document_id, segment_id, preview_text").
		Where("id = ? AND user_id = ?", req.CitationId, userId).
		One()
	if err != nil {
		return nil, err
	}
	if citation == nil {
		return nil, gerror.New("引用不存在或无权访问")
	}
	documentId := citation["document_id"].Int64()
	if !canAccessDocument(ctx, userId, documentId) {
		return nil, gerror.New("无权访问引用文档")
	}
	document, err := loadQaDocument(ctx, documentId)
	if err != nil {
		return nil, err
	}
	if document == nil {
		return nil, gerror.New("引用文档不存在")
	}
	segment, err := loadQaDocumentSegment(ctx, citation["segment_id"].Int64())
	if err != nil {
		return nil, err
	}
	if segment == nil || segment["document_id"].Int64() != documentId {
		return nil, gerror.New("引用段落不存在")
	}
	return &v1.CitationDetailRes{
		CitationId:    citation["id"].Int64(),
		SessionId:     citation["session_id"].String(),
		MessageId:     citation["message_id"].Int64(),
		DocumentId:    documentId,
		DocumentTitle: document["title"].String(),
		FileName:      document["file_name"].String(),
		FileType:      document["file_type"].String(),
		KnowledgeCode: document["knowledge_code"].String(),
		SegmentId:     segment["id"].Int64(),
		SegmentIndex:  segment["segment_index"].Int(),
		Page:          segment["page"].Int(),
		Anchor:        segment["anchor"].String(),
		PreviewText:   citation["preview_text"].String(),
		Content:       segment["content"].String(),
	}, nil
}

func (c *ControllerV1) DocumentView(ctx context.Context, req *v1.DocumentViewReq) (res *v1.DocumentViewRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	if !canAccessDocument(ctx, userId, req.DocumentId) {
		return nil, gerror.New("文档不存在或无权访问")
	}
	document, err := loadQaDocument(ctx, req.DocumentId)
	if err != nil {
		return nil, err
	}
	if document == nil || document["status"].String() != "active" {
		return nil, gerror.New("文档不存在或不可用")
	}
	segments, err := loadQaDocumentSegments(ctx, req.DocumentId)
	if err != nil {
		return nil, err
	}
	items := make([]v1.DocumentSegmentItem, 0, len(segments))
	for _, segment := range segments {
		items = append(items, v1.DocumentSegmentItem{
			SegmentId:    segment["id"].Int64(),
			SegmentIndex: segment["segment_index"].Int(),
			Content:      segment["content"].String(),
			Page:         segment["page"].Int(),
			Anchor:       segment["anchor"].String(),
		})
	}
	return &v1.DocumentViewRes{
		DocumentId:     req.DocumentId,
		KnowledgeCode:  document["knowledge_code"].String(),
		Title:          document["title"].String(),
		FileName:       document["file_name"].String(),
		FileType:       document["file_type"].String(),
		SourceProvider: document["source_provider"].String(),
		ExternalId:     document["external_id"].String(),
		Status:         document["status"].String(),
		Segments:       items,
	}, nil
}

func (c *ControllerV1) PopularQuestions(ctx context.Context, req *v1.PopularQuestionsReq) (res *v1.PopularQuestionsRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 3
	}
	if limit > 20 {
		limit = 20
	}
	days := req.Days
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	knowledgeCode := strings.TrimSpace(req.KnowledgeCode)
	requestedCodes := requestedKnowledgeCodeSet(knowledgeCode)
	knowledgeCodes, err := accessibleKnowledgeCodes(ctx, userId, knowledgeCode)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(knowledgeCodes)+1)
	for _, code := range knowledgeCodes {
		allowed[code] = true
	}
	if knowledgeCode == "" {
		allowed[""] = true
	}

	since := int(gtime.Timestamp()) - days*24*60*60
	records, err := g.DB("master").Model("qa_question_stat").Ctx(ctx).
		Fields("knowledge_code, question, hit_count, last_asked_at").
		Where("user_id = ? AND last_asked_at >= ?", userId, since).
		OrderDesc("hit_count").
		OrderDesc("last_asked_at").
		Limit(200).
		All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.PopularQuestionItem, 0, limit)
	for _, record := range records {
		code := record["knowledge_code"].String()
		if len(requestedCodes) > 0 && !requestedCodes[code] {
			continue
		}
		if !allowed[code] {
			continue
		}
		question := strings.TrimSpace(record["question"].String())
		if question == "" {
			continue
		}
		list = append(list, v1.PopularQuestionItem{
			Question:      question,
			KnowledgeCode: code,
			HitCount:      record["hit_count"].Int(),
			LastAskedAt:   record["last_asked_at"].Int(),
			Source:        "stat",
		})
		if len(list) >= limit {
			break
		}
	}
	if len(list) == 0 {
		list = defaultPopularQuestions(knowledgeCode, knowledgeCodes, limit)
	}
	return &v1.PopularQuestionsRes{List: list}, nil
}

func (c *ControllerV1) WebSearch(ctx context.Context, req *v1.WebSearchReq) (res *v1.WebSearchRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, gerror.New("搜索问题不能为空")
	}
	knowledgeCode := strings.TrimSpace(req.KnowledgeCode)
	if knowledgeCode != "" {
		if _, err = accessibleKnowledgeCodes(ctx, userId, knowledgeCode); err != nil {
			return nil, err
		}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	cfg := qaWebSearchConfig()
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" {
		provider = "local"
	}
	if !cfg.Enabled {
		return &v1.WebSearchRes{
			Enabled:  false,
			Provider: provider,
			Message:  "联网搜索未启用，当前仅使用本地知识库检索。",
			List:     []v1.WebSearchItem{},
		}, nil
	}
	return &v1.WebSearchRes{
		Enabled:  true,
		Provider: provider,
		Message:  fmt.Sprintf("联网搜索供应商 %s 已配置占位，当前版本不发起外部搜索调用。", provider),
		List:     []v1.WebSearchItem{},
	}, nil
}

func qaWebSearchResult(ctx context.Context, query string, knowledgeCode string, limit int) *v1.WebSearchRes {
	cfg := qaWebSearchConfig()
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" {
		provider = "local"
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	if !cfg.Enabled {
		return &v1.WebSearchRes{
			Enabled:  false,
			Provider: provider,
			Message:  "联网搜索未启用，当前仅使用本地知识库检索。",
			List:     []v1.WebSearchItem{},
		}
	}
	return &v1.WebSearchRes{
		Enabled:  true,
		Provider: provider,
		Message:  fmt.Sprintf("联网搜索供应商 %s 已配置占位，当前版本不发起外部搜索调用。", provider),
		List:     []v1.WebSearchItem{},
	}
}

func (c *ControllerV1) SyncKnowledgeBases(ctx context.Context, req *v1.QaSyncReq) (res *v1.QaSyncRes, err error) {
	return executeQaSync(ctx, aidgp.SyncKnowledgeBases, "")
}

func (c *ControllerV1) SyncDocuments(ctx context.Context, req *v1.QaSyncDocumentsReq) (res *v1.QaSyncRes, err error) {
	return executeQaSync(ctx, aidgp.SyncDocuments, strings.TrimSpace(req.KnowledgeCode))
}

func (c *ControllerV1) SyncPermissions(ctx context.Context, req *v1.QaSyncPermissionsReq) (res *v1.QaSyncRes, err error) {
	return executeQaSync(ctx, aidgp.SyncPermissions, strings.TrimSpace(req.KnowledgeCode))
}

func (c *ControllerV1) SyncGridData(ctx context.Context, req *v1.QaSyncGridDataReq) (res *v1.QaSyncRes, err error) {
	return executeQaSync(ctx, aidgp.SyncGridData, "")
}

func (c *ControllerV1) SyncTrafficData(ctx context.Context, req *v1.QaSyncTrafficDataReq) (res *v1.QaSyncRes, err error) {
	return executeQaSync(ctx, aidgp.SyncTrafficData, "")
}

func (c *ControllerV1) SyncPopulationData(ctx context.Context, req *v1.QaSyncPopulationDataReq) (res *v1.QaSyncRes, err error) {
	return executeQaSync(ctx, aidgp.SyncPopulationData, "")
}

func (c *ControllerV1) SyncStatus(ctx context.Context, req *v1.QaSyncStatusReq) (res *v1.QaSyncStatusRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = qaSyncProvider()
	}
	query := g.DB("master").Model("qa_sync_task").Ctx(ctx).
		Fields("id, provider, sync_type, status, message, success_count, failure_count, skipped_count, started_at, finished_at, create_time, update_time")
	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if syncType := strings.TrimSpace(req.SyncType); syncType != "" {
		query = query.Where("sync_type = ?", syncType)
	}
	records, err := query.OrderDesc("id").Limit(limit).All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.QaSyncTaskItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.QaSyncTaskItem{
			TaskId:       record["id"].Int64(),
			Provider:     record["provider"].String(),
			SyncType:     record["sync_type"].String(),
			Status:       record["status"].String(),
			Message:      record["message"].String(),
			SuccessCount: record["success_count"].Int(),
			FailureCount: record["failure_count"].Int(),
			SkippedCount: record["skipped_count"].Int(),
			StartedAt:    record["started_at"].Int(),
			FinishedAt:   record["finished_at"].Int(),
			CreateTime:   record["create_time"].Int(),
			UpdateTime:   record["update_time"].Int(),
		})
	}
	return &v1.QaSyncStatusRes{
		Provider: provider,
		Enabled:  provider == "aidgp",
		List:     list,
	}, nil
}

func (c *ControllerV1) SyncLogs(ctx context.Context, req *v1.QaSyncLogsReq) (res *v1.QaSyncLogsRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := g.DB("master").Model("qa_sync_log").Ctx(ctx).
		Fields("id, task_id, provider, sync_type, external_id, local_id, action, status, message, create_time").
		Where("task_id = ?", req.TaskId)
	if status := strings.TrimSpace(req.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	records, err := query.OrderAsc("id").Limit(limit).All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.QaSyncLogItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.QaSyncLogItem{
			LogId:      record["id"].Int64(),
			TaskId:     record["task_id"].Int64(),
			Provider:   record["provider"].String(),
			SyncType:   record["sync_type"].String(),
			ExternalId: record["external_id"].String(),
			LocalId:    record["local_id"].String(),
			Action:     record["action"].String(),
			Status:     record["status"].String(),
			Message:    record["message"].String(),
			CreateTime: record["create_time"].Int(),
		})
	}
	return &v1.QaSyncLogsRes{TaskId: req.TaskId, List: list}, nil
}

func (c *ControllerV1) SessionReset(ctx context.Context, req *v1.SessionResetReq) (res *v1.SessionResetRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	count, err := g.DB("master").Model("qa_session").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gerror.New("会话不存在或无权操作")
	}
	now := int(gtime.Timestamp())
	if _, err = g.DB("master").Model("qa_message").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Delete(); err != nil {
		return nil, err
	}
	if _, err = g.DB("master").Model("qa_citation").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Delete(); err != nil {
		return nil, err
	}
	if _, err = g.DB("master").Model("qa_session").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Data(g.Map{"status": "reset", "update_time": now}).
		Update(); err != nil {
		return nil, err
	}
	return &v1.SessionResetRes{SessionId: req.SessionId}, nil
}

type qaCitationEvent struct {
	CitationId     int64
	Index          int
	KnowledgeCode  string
	DocumentId     int64
	DocumentTitle  string
	FileName       string
	SegmentId      int64
	Page           int
	Anchor         string
	Score          float64
	Content        string
	SourceProvider string
}

func streamQaChat(ctx context.Context, req *v1.ChatReq, userId int64, sessionId string, knowledgeCode string, question string, history []model.ChatHistoryItem, items []v1.RetrieveItem, citations []qaCitationEvent, respChan chan any, requestStart time.Time) {
	totalStart := time.Now()
	stageStart := totalStart
	logStage := func(stage string) {
		consts.Logger.Infof(ctx, "perf qa_stream stage=%s knowledgeCode=%s sessionId=%s costMs=%d streamMs=%d requestMs=%d", stage, knowledgeCode, sessionId, time.Since(stageStart).Milliseconds(), time.Since(totalStart).Milliseconds(), time.Since(requestStart).Milliseconds())
		stageStart = time.Now()
	}
	defer func() {
		consts.Logger.Infof(ctx, "perf qa_stream stage=total knowledgeCode=%s sessionId=%s costMs=%d requestMs=%d", knowledgeCode, sessionId, time.Since(totalStart).Milliseconds(), time.Since(requestStart).Milliseconds())
	}()
	defer close(respChan)
	if err := model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "start",
		Data: g.Map{
			"sessionId": sessionId,
		},
	}, respChan); err != nil {
		return
	}
	logStage("send_start")
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "retrieval",
		Data: g.Map{
			"list": items,
		},
	}, respChan)
	logStage("send_retrieval")
	var webItems []v1.WebSearchItem
	if req.WebSearch {
		webResult := qaWebSearchResult(ctx, question, knowledgeCode, 5)
		webItems = webResult.List
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event: "web_search",
			Data:  webResult,
		}, respChan)
	}
	logStage("web_search")
	if req.DeepThinking {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event: "thinking",
			Data: g.Map{
				"steps": []string{
					"读取当前会话问题和历史上下文",
					"校验用户可访问的文档库范围",
					"召回内部文档片段并准备引用来源",
					"按开关补充联网搜索结果",
					"基于可用来源生成回答",
				},
			},
		}, respChan)
	}
	logStage("thinking")
	for _, citation := range citations {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event: "citation",
			Data: g.Map{
				"citationId":     citation.CitationId,
				"index":          citation.Index,
				"knowledgeCode":  citation.KnowledgeCode,
				"documentId":     citation.DocumentId,
				"documentTitle":  citation.DocumentTitle,
				"fileName":       citation.FileName,
				"segmentId":      citation.SegmentId,
				"page":           citation.Page,
				"anchor":         citation.Anchor,
				"score":          citation.Score,
				"content":        citation.Content,
				"sourceProvider": citation.SourceProvider,
			},
		}, respChan)
	}
	logStage("send_citations")

	aiProvider := strings.TrimSpace(req.Ai)
	if aiProvider == "" {
		aiProvider = "deepseek"
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" && aiProvider == "deepseek" {
		modelName = "deepseek-chat"
	}
	llm, err := service.AI().GetChatModel(aiProvider, modelName)
	if err != nil {
		sendQaError(ctx, respChan, err)
		return
	}
	logStage("model")
	messages := buildQaMessages(question, history, items, webItems, req.DeepThinking)
	logStage("prompt")
	stream, err := llm.Stream(ctx, messages)
	if err != nil {
		sendQaError(ctx, respChan, err)
		return
	}
	defer stream.Close()
	logStage("model_stream")

	var assistantBuilder strings.Builder
	firstTokenLogged := false
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if assistantBuilder.Len() > 0 {
				assistantMessageId, saveErr := appendQaMessage(ctx, userId, sessionId, knowledgeCode, "assistant", assistantBuilder.String())
				if saveErr != nil {
					consts.Logger.Errorf(ctx, "保存问答会话回复失败: %s", saveErr.Error())
				} else if linkErr := updateQaCitationMessageIds(ctx, userId, sessionId, citationEventIds(citations), assistantMessageId); linkErr != nil {
					consts.Logger.Errorf(ctx, "关联问答引用消息失败: %s", linkErr.Error())
				}
			}
			_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
			return
		}
		if err != nil {
			sendQaError(ctx, respChan, err)
			return
		}
		if chunk == nil || chunk.Content == "" {
			continue
		}
		if !firstTokenLogged {
			firstTokenLogged = true
			consts.Logger.Infof(ctx, "perf qa_stream stage=first_token knowledgeCode=%s sessionId=%s requestMs=%d", knowledgeCode, sessionId, time.Since(requestStart).Milliseconds())
		}
		assistantBuilder.WriteString(chunk.Content)
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event:   "message",
			Role:    gconv.String(chunk.Role),
			Content: chunk.Content,
		}, respChan)
	}
}

func retrieveQaItems(ctx context.Context, userId int64, question string, knowledgeCode string, topK int) ([]v1.RetrieveItem, error) {
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}
	knowledgeCodes, err := accessibleKnowledgeCodes(ctx, userId, knowledgeCode)
	if err != nil {
		return nil, err
	}
	if len(knowledgeCodes) == 0 {
		return []v1.RetrieveItem{}, nil
	}

	terms := retrieveTerms(question)
	sqlTerms := retrieveSQLTerms(question, terms)
	records, err := queryQaRetrieveRecords(ctx, userId, knowledgeCodes, sqlTerms)
	if err == nil && len(records) == 0 && len(sqlTerms) > 0 {
		records, err = queryQaRetrieveRecords(ctx, userId, knowledgeCodes, nil)
	}
	if err != nil {
		return nil, err
	}

	items := make([]v1.RetrieveItem, 0, topK)
	for _, record := range records {
		content := strings.TrimSpace(record["content"].String())
		if content == "" {
			continue
		}
		score := retrieveScore(question, terms, content)
		if score <= 0 {
			continue
		}
		if title := record["document_title"].String(); title != "" {
			score += retrieveScore(question, terms, title) * 0.25
		}
		items = append(items, v1.RetrieveItem{
			KnowledgeCode: record["knowledge_code"].String(),
			DocumentId:    record["document_id"].Int64(),
			DocumentTitle: record["document_title"].String(),
			FileName:      record["file_name"].String(),
			SegmentId:     record["segment_id"].Int64(),
			Content:       content,
			Page:          record["page"].Int(),
			Anchor:        record["anchor"].String(),
			Score:         score,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	if len(items) > topK {
		items = items[:topK]
	}
	return items, nil
}

func queryQaRetrieveRecords(ctx context.Context, userId int64, knowledgeCodes []string, sqlTerms []string) (gdb.Result, error) {
	args := make([]any, 0, 1+len(knowledgeCodes)+len(sqlTerms)*2)
	args = append(args, userId)
	args = append(args, convertToAnySlice(knowledgeCodes)...)

	keywordClause := ""
	if len(sqlTerms) > 0 {
		likeParts := make([]string, 0, len(sqlTerms))
		for _, term := range sqlTerms {
			likeParts = append(likeParts, "(s.content LIKE ? OR d.title LIKE ?)")
			likeArg := "%" + term + "%"
			args = append(args, likeArg, likeArg)
		}
		keywordClause = " AND (" + strings.Join(likeParts, " OR ") + ")"
	}

	return g.DB("master").Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT DISTINCT s.id AS segment_id, s.knowledge_code, s.document_id,
		       s.content, s.page, s.anchor,
		       d.title AS document_title, d.file_name
		FROM qa_document_segment s
		INNER JOIN qa_document d ON d.id = s.document_id AND d.status = 'active'
		INNER JOIN qa_document_permission dp ON dp.document_id = s.document_id
		                                      AND dp.enabled = 1
		                                      AND dp.user_id IN (?, 0)
		WHERE s.knowledge_code IN (%s)%s
		ORDER BY s.id DESC
		LIMIT 500`, inPlaceholders(len(knowledgeCodes)), keywordClause),
		args...,
	).All()
}

// inPlaceholders returns comma-separated "?" placeholders for SQL IN clauses.
func inPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func convertToAnySlice(items []string) []any {
	result := make([]any, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

const maxSegmentContentRunes = 800

func buildQaMessages(question string, history []model.ChatHistoryItem, items []v1.RetrieveItem, webItems []v1.WebSearchItem, deepThinking bool) []*schema.Message {
	var contextBuilder strings.Builder
	for index, item := range items {
		content := item.Content
		if len([]rune(content)) > maxSegmentContentRunes {
			content = string([]rune(content)[:maxSegmentContentRunes]) + "..."
		}
		contextBuilder.WriteString(fmt.Sprintf("【引用%d】知识库:%s 文档:%s 文件:%s 页码:%d 锚点:%s\n%s\n\n",
			index+1,
			item.KnowledgeCode,
			item.DocumentTitle,
			item.FileName,
			item.Page,
			item.Anchor,
			content,
		))
	}
	if contextBuilder.Len() == 0 {
		contextBuilder.WriteString("未召回到可用知识库片段。")
	}
	var webBuilder strings.Builder
	for index, item := range webItems {
		webBuilder.WriteString(fmt.Sprintf("【联网%d】标题:%s 链接:%s 来源:%s\n%s\n\n",
			index+1,
			item.Title,
			item.Url,
			item.Source,
			item.Snippet,
		))
	}
	if webBuilder.Len() == 0 {
		webBuilder.WriteString("未提供可用联网搜索结果。\n")
	}
	systemPrompt := "你是问答端知识库助手。回答必须基于已提供的知识库片段；如果片段不足以回答，请明确说明未在当前知识库中找到依据。回答中涉及事实、制度、流程时使用 [引用1] 这样的格式标注来源。不要编造外部材料；当前阶段不调用 AIDGP。"
	if deepThinking {
		systemPrompt += " 已开启深度思考：请先在内部梳理问题、检索依据、适用范围和不确定点，再输出清晰结论；不要输出冗长推理链，只输出可核验的依据和结论。"
	}
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.System,
			Content: "知识库片段如下：\n" + contextBuilder.String(),
		},
		{
			Role:    schema.System,
			Content: "联网搜索补充如下：\n" + webBuilder.String(),
		},
	}
	for _, item := range history {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		role := schema.User
		if item.Role == "assistant" {
			role = schema.Assistant
		}
		messages = append(messages, &schema.Message{Role: role, Content: content})
	}
	messages = append(messages, &schema.Message{Role: schema.User, Content: question})
	return messages
}

func sendQaError(ctx context.Context, respChan chan any, err error) {
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "error",
		Data: g.Map{
			"message": err.Error(),
		},
	}, respChan)
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
}

func ensureQaSession(ctx context.Context, userId int64, sessionId string, knowledgeCode string, firstMessage string) (string, error) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		sessionId = newQaSessionId()
	}
	db := g.DB("master")
	count, err := db.Model("qa_session").Ctx(ctx).
		Where("session_id = ?", sessionId).
		Count()
	if err != nil {
		return "", err
	}
	now := int(gtime.Timestamp())
	title := strings.TrimSpace(firstMessage)
	if len([]rune(title)) > 30 {
		title = string([]rune(title)[:30])
	}
	if count == 0 {
		_, err = db.Model("qa_session").Ctx(ctx).Data(g.Map{
			"session_id":     sessionId,
			"user_id":        userId,
			"knowledge_code": knowledgeCode,
			"title":          title,
			"status":         "active",
			"create_time":    now,
			"update_time":    now,
		}).Insert()
		return sessionId, err
	}
	record, err := db.Model("qa_session").Ctx(ctx).
		Fields("user_id").
		Where("session_id = ?", sessionId).
		One()
	if err != nil {
		return "", err
	}
	if record != nil && record["user_id"].Int64() != userId {
		return "", gerror.New("会话不存在或无权访问")
	}
	_, err = db.Model("qa_session").Ctx(ctx).
		Where("session_id = ?", sessionId).
		Data(g.Map{
			"knowledge_code": knowledgeCode,
			"status":         "active",
			"update_time":    now,
		}).
		Update()
	return sessionId, err
}

func loadQaSessionHistory(ctx context.Context, userId int64, sessionId string, limit int) ([]model.ChatHistoryItem, error) {
	if limit <= 0 {
		limit = 12
	}
	records, err := g.DB("master").Model("qa_message").Ctx(ctx).
		Fields("role, content").
		Where("session_id = ? AND user_id = ?", sessionId, userId).
		OrderDesc("id").
		Limit(limit).
		All()
	if err != nil {
		return nil, err
	}
	history := make([]model.ChatHistoryItem, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		role := records[i]["role"].String()
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(records[i]["content"].String())
		if content == "" {
			continue
		}
		history = append(history, model.ChatHistoryItem{Role: role, Content: content})
	}
	return history, nil
}

func appendQaMessage(ctx context.Context, userId int64, sessionId string, knowledgeCode string, role string, content string) (int64, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0, nil
	}
	now := int(gtime.Timestamp())
	messageId, err := g.DB("master").Model("qa_message").Ctx(ctx).Data(g.Map{
		"session_id":     sessionId,
		"user_id":        userId,
		"knowledge_code": knowledgeCode,
		"role":           role,
		"content":        content,
		"create_time":    now,
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	_, err = g.DB("master").Model("qa_session").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", sessionId, userId).
		Data(g.Map{"update_time": now}).
		Update()
	return messageId, err
}

func createQaCitations(ctx context.Context, userId int64, sessionId string, items []v1.RetrieveItem) ([]qaCitationEvent, error) {
	citations := make([]qaCitationEvent, 0, len(items))
	for index, item := range items {
		citationId, err := appendQaCitation(ctx, userId, sessionId, item.DocumentId, item.SegmentId, item.Content)
		if err != nil {
			return nil, err
		}
		citations = append(citations, qaCitationEvent{
			CitationId:     citationId,
			Index:          index + 1,
			KnowledgeCode:  item.KnowledgeCode,
			DocumentId:     item.DocumentId,
			DocumentTitle:  item.DocumentTitle,
			FileName:       item.FileName,
			SegmentId:      item.SegmentId,
			Page:           item.Page,
			Anchor:         item.Anchor,
			Score:          item.Score,
			Content:        item.Content,
			SourceProvider: "local",
		})
	}
	return citations, nil
}

func appendQaCitation(ctx context.Context, userId int64, sessionId string, documentId int64, segmentId int64, previewText string) (int64, error) {
	previewText = strings.TrimSpace(previewText)
	if len([]rune(previewText)) > 500 {
		previewText = string([]rune(previewText)[:500])
	}
	return g.DB("master").Model("qa_citation").Ctx(ctx).Data(g.Map{
		"session_id":   sessionId,
		"user_id":      userId,
		"message_id":   0,
		"document_id":  documentId,
		"segment_id":   segmentId,
		"preview_text": previewText,
		"create_time":  int(gtime.Timestamp()),
	}).InsertAndGetId()
}

func citationEventIds(citations []qaCitationEvent) []int64 {
	ids := make([]int64, 0, len(citations))
	for _, citation := range citations {
		if citation.CitationId > 0 {
			ids = append(ids, citation.CitationId)
		}
	}
	return ids
}

func updateQaCitationMessageIds(ctx context.Context, userId int64, sessionId string, citationIds []int64, messageId int64) error {
	if messageId <= 0 || len(citationIds) == 0 {
		return nil
	}
	_, err := g.DB("master").Model("qa_citation").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", sessionId, userId).
		WhereIn("id", citationIds).
		Data(g.Map{"message_id": messageId}).
		Update()
	return err
}

func loadQaDocument(ctx context.Context, documentId int64) (gdb.Record, error) {
	return g.DB("master").Model("qa_document").Ctx(ctx).
		Fields("id, knowledge_code, title, file_name, file_type, source_provider, external_id, status").
		Where("id = ?", documentId).
		One()
}

func loadQaDocumentSegment(ctx context.Context, segmentId int64) (gdb.Record, error) {
	return g.DB("master").Model("qa_document_segment").Ctx(ctx).
		Fields("id, document_id, knowledge_code, segment_index, content, page, anchor").
		Where("id = ?", segmentId).
		One()
}

func loadQaDocumentSegments(ctx context.Context, documentId int64) (gdb.Result, error) {
	return g.DB("master").Model("qa_document_segment").Ctx(ctx).
		Fields("id, document_id, knowledge_code, segment_index, content, page, anchor").
		Where("document_id = ?", documentId).
		OrderAsc("segment_index").
		All()
}

func mergeQaHistory(stored []model.ChatHistoryItem, client []model.ChatHistoryItem) []model.ChatHistoryItem {
	merged := make([]model.ChatHistoryItem, 0, len(stored)+len(client))
	seen := make(map[string]bool)
	appendItems := func(items []model.ChatHistoryItem) {
		for _, item := range items {
			role := strings.TrimSpace(item.Role)
			content := strings.TrimSpace(item.Content)
			if content == "" || (role != "user" && role != "assistant") {
				continue
			}
			key := role + "\x00" + content
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, model.ChatHistoryItem{Role: role, Content: content})
		}
	}
	appendItems(stored)
	appendItems(client)
	if len(merged) > 20 {
		merged = merged[len(merged)-20:]
	}
	return merged
}

func upsertQaQuestionStat(ctx context.Context, userId int64, knowledgeCode string, question string) error {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil
	}
	if len([]rune(question)) > 120 {
		question = string([]rune(question)[:120])
	}
	db := g.DB("master")
	record, err := db.Model("qa_question_stat").Ctx(ctx).
		Fields("id, hit_count").
		Where("user_id = ? AND knowledge_code = ? AND question = ?", userId, knowledgeCode, question).
		One()
	if err != nil {
		return err
	}
	now := int(gtime.Timestamp())
	if record == nil {
		_, err = db.Model("qa_question_stat").Ctx(ctx).Data(g.Map{
			"user_id":        userId,
			"knowledge_code": knowledgeCode,
			"question":       question,
			"hit_count":      1,
			"last_asked_at":  now,
			"create_time":    now,
			"update_time":    now,
		}).Insert()
		return err
	}
	_, err = db.Model("qa_question_stat").Ctx(ctx).
		Where("id = ?", record["id"].Int64()).
		Data(g.Map{
			"hit_count":     record["hit_count"].Int() + 1,
			"last_asked_at": now,
			"update_time":   now,
		}).
		Update()
	return err
}

func defaultPopularQuestions(knowledgeCode string, knowledgeCodes []string, limit int) []v1.PopularQuestionItem {
	if limit <= 0 {
		limit = 3
	}
	candidates := make([]v1.PopularQuestionItem, 0, limit)
	appendDefaults := func(code string, questions []string) {
		for _, question := range questions {
			if len(candidates) >= limit {
				return
			}
			candidates = append(candidates, v1.PopularQuestionItem{
				Question:      question,
				KnowledgeCode: code,
				HitCount:      0,
				Source:        "default",
			})
		}
	}
	defaults := map[string][]string{
		"policy": {
			"有哪些政策制度需要重点关注？",
			"这份制度的适用范围是什么？",
			"相关流程和责任分工是什么？",
		},
		"manual": {
			"这个业务流程应该如何办理？",
			"操作手册里有哪些注意事项？",
			"遇到异常情况应该怎么处理？",
		},
	}
	general := []string{
		"当前知识库有哪些重点内容？",
		"请总结相关文档的核心要求。",
		"这个问题在知识库中有哪些依据？",
	}
	if knowledgeCode != "" {
		if questions, ok := defaults[knowledgeCode]; ok {
			appendDefaults(knowledgeCode, questions)
		} else {
			appendDefaults(knowledgeCode, general)
		}
		return candidates
	}
	for _, code := range knowledgeCodes {
		if questions, ok := defaults[code]; ok {
			appendDefaults(code, questions)
		}
		if len(candidates) >= limit {
			return candidates
		}
	}
	if len(candidates) < limit {
		appendDefaults("", general)
	}
	return candidates
}

func createQaSyncPlaceholder(ctx context.Context, syncType string, knowledgeCode string) (*v1.QaSyncRes, error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	if knowledgeCode != "" {
		if _, err := accessibleKnowledgeCodes(ctx, userId, knowledgeCode); err != nil {
			return nil, err
		}
	}
	provider := qaSyncProvider()
	message := qaSyncPlaceholderMessage(provider, syncType, knowledgeCode)
	status := "skipped"
	taskId, err := insertQaSyncTask(ctx, provider, syncType, status, message)
	if err != nil {
		return nil, err
	}
	return &v1.QaSyncRes{
		TaskId:   taskId,
		Provider: provider,
		SyncType: syncType,
		Status:   status,
		Message:  message,
	}, nil
}

func qaSyncProvider() string {
	if consts.Config == nil || consts.Config.QaConfig == nil || consts.Config.QaConfig.Sync == nil {
		return aidgp.ProviderMock
	}
	provider := strings.TrimSpace(consts.Config.QaConfig.Sync.Provider)
	if provider == "" {
		return aidgp.ProviderMock
	}
	return provider
}

func qaWebSearchConfig() *model.QaWebSearchConfig {
	if consts.Config == nil || consts.Config.QaConfig == nil || consts.Config.QaConfig.WebSearch == nil {
		return &model.QaWebSearchConfig{Provider: "local"}
	}
	return consts.Config.QaConfig.WebSearch
}

func qaSyncPlaceholderMessage(provider string, syncType string, knowledgeCode string) string {
	scope := "全部知识库"
	if knowledgeCode != "" {
		scope = "知识库 " + knowledgeCode
	}
	if provider != "aidgp" {
		return fmt.Sprintf("当前同步 provider=%s，外部同步未启用；已跳过 %s 的 %s 同步。", provider, scope, syncType)
	}
	if consts.Config == nil || consts.Config.QaConfig == nil || consts.Config.QaConfig.Sync == nil || consts.Config.QaConfig.Sync.Aidgp == nil {
		return fmt.Sprintf("AIDGP 同步适配层已预留，但配置不完整；未发起 %s 的 %s 同步。", scope, syncType)
	}
	aidgp := consts.Config.QaConfig.Sync.Aidgp
	if strings.TrimSpace(aidgp.BaseUrl) == "" || strings.TrimSpace(aidgp.AppKey) == "" || strings.TrimSpace(aidgp.AppSecret) == "" {
		return fmt.Sprintf("AIDGP 同步适配层已预留，但 baseUrl/appKey/appSecret 未完整配置；未发起 %s 的 %s 同步。", scope, syncType)
	}
	return fmt.Sprintf("AIDGP 同步适配层已预留；当前版本不发起外部调用，已跳过 %s 的 %s 同步。", scope, syncType)
}

func insertQaSyncTask(ctx context.Context, provider string, syncType string, status string, message string) (int64, error) {
	now := int(gtime.Timestamp())
	return g.DB("master").Model("qa_sync_task").Ctx(ctx).Data(g.Map{
		"provider":    provider,
		"sync_type":   syncType,
		"status":      status,
		"message":     message,
		"create_time": now,
		"update_time": now,
	}).InsertAndGetId()
}

func executeQaSync(ctx context.Context, syncType string, knowledgeCode string) (*v1.QaSyncRes, error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	if knowledgeCode != "" {
		if _, err := accessibleKnowledgeCodes(ctx, userId, knowledgeCode); err != nil {
			return nil, err
		}
	}
	provider := qaSyncProvider()
	taskId, err := insertQaSyncTaskV2(ctx, provider, syncType, "running", "同步任务已开始")
	if err != nil {
		return nil, err
	}
	client := qaAidgpClient(provider)
	result, syncErr := callAidgpSync(ctx, client, syncType, knowledgeCode)
	status := "success"
	message := result.Message
	if syncErr != nil {
		status = "failed"
		message = syncErr.Error()
		result.FailureCount++
		result.Logs = append(result.Logs, aidgp.SyncLog{
			Action:  "execute",
			Status:  "failed",
			Message: syncErr.Error(),
		})
	} else if result.FailureCount > 0 {
		status = "partial_failed"
	} else if result.SuccessCount == 0 && result.SkippedCount > 0 {
		status = "skipped"
	}
	if err = insertQaSyncLogs(ctx, taskId, provider, syncType, result.Logs); err != nil {
		return nil, err
	}
	if err = finishQaSyncTask(ctx, taskId, status, message, result.SuccessCount, result.FailureCount, result.SkippedCount); err != nil {
		return nil, err
	}
	return &v1.QaSyncRes{
		TaskId:       taskId,
		Provider:     provider,
		SyncType:     syncType,
		Status:       status,
		Message:      message,
		SuccessCount: result.SuccessCount,
		FailureCount: result.FailureCount,
		SkippedCount: result.SkippedCount,
	}, nil
}

func qaAidgpClient(provider string) aidgp.Client {
	cfg := aidgp.Config{Provider: provider}
	if consts.Config != nil && consts.Config.QaConfig != nil && consts.Config.QaConfig.Sync != nil && consts.Config.QaConfig.Sync.Aidgp != nil {
		aidgpCfg := consts.Config.QaConfig.Sync.Aidgp
		cfg.BaseUrl = aidgpCfg.BaseUrl
		cfg.AppKey = aidgpCfg.AppKey
		cfg.AppSecret = aidgpCfg.AppSecret
		cfg.TokenPath = aidgpCfg.TokenPath
		cfg.TrafficQueryPath = aidgpCfg.TrafficQueryPath
		cfg.PopulationQueryPath = aidgpCfg.PopulationQueryPath
		cfg.GridQueryPath = aidgpCfg.GridQueryPath
		cfg.KnowledgeBasesPath = aidgpCfg.KnowledgeBasesPath
		cfg.DocumentsPath = aidgpCfg.DocumentsPath
		cfg.DocumentSegmentsPath = aidgpCfg.DocumentSegmentsPath
		cfg.KnowledgePermissionsPath = aidgpCfg.KnowledgePermissionsPath
		cfg.TimeoutSeconds = aidgpCfg.TimeoutSeconds
		cfg.RetryTimes = aidgpCfg.RetryTimes
		cfg.TokenExpireSkewSeconds = aidgpCfg.TokenExpireSkewSeconds
	}
	return aidgp.NewClient(cfg)
}

func callAidgpSync(ctx context.Context, client aidgp.Client, syncType string, knowledgeCode string) (aidgp.SyncResult, error) {
	scope := aidgp.SyncScope{KnowledgeCode: knowledgeCode}
	switch syncType {
	case aidgp.SyncKnowledgeBases:
		return client.SyncKnowledgeBases(ctx, scope)
	case aidgp.SyncDocuments:
		return client.SyncDocuments(ctx, scope)
	case aidgp.SyncPermissions:
		return client.SyncPermissions(ctx, scope)
	case aidgp.SyncGridData:
		return client.SyncGridData(ctx, scope)
	case aidgp.SyncTrafficData:
		return client.SyncTrafficData(ctx, scope)
	case aidgp.SyncPopulationData:
		return client.SyncPopulationData(ctx, scope)
	default:
		return aidgp.SyncResult{}, fmt.Errorf("unsupported sync type: %s", syncType)
	}
}

func insertQaSyncTaskV2(ctx context.Context, provider string, syncType string, status string, message string) (int64, error) {
	now := int(gtime.Timestamp())
	return g.DB("master").Model("qa_sync_task").Ctx(ctx).Data(g.Map{
		"provider":      provider,
		"sync_type":     syncType,
		"status":        status,
		"message":       message,
		"success_count": 0,
		"failure_count": 0,
		"skipped_count": 0,
		"started_at":    now,
		"finished_at":   0,
		"create_time":   now,
		"update_time":   now,
	}).InsertAndGetId()
}

func finishQaSyncTask(ctx context.Context, taskId int64, status string, message string, successCount int, failureCount int, skippedCount int) error {
	now := int(gtime.Timestamp())
	_, err := g.DB("master").Model("qa_sync_task").Ctx(ctx).
		Where("id = ?", taskId).
		Data(g.Map{
			"status":        status,
			"message":       message,
			"success_count": successCount,
			"failure_count": failureCount,
			"skipped_count": skippedCount,
			"finished_at":   now,
			"update_time":   now,
		}).
		Update()
	return err
}

func insertQaSyncLogs(ctx context.Context, taskId int64, provider string, syncType string, logs []aidgp.SyncLog) error {
	now := int(gtime.Timestamp())
	if len(logs) == 0 {
		logs = []aidgp.SyncLog{{Action: "execute", Status: "success", Message: "同步执行完成，无明细记录"}}
	}
	for _, item := range logs {
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "success"
		}
		if _, err := g.DB("master").Model("qa_sync_log").Ctx(ctx).Data(g.Map{
			"task_id":     taskId,
			"provider":    provider,
			"sync_type":   syncType,
			"external_id": item.ExternalId,
			"local_id":    item.LocalId,
			"action":      item.Action,
			"status":      status,
			"message":     item.Message,
			"create_time": now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func newQaSessionId() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("qa_%d", gtime.TimestampNano())
	}
	return fmt.Sprintf("qa_%x", buf)
}

func currentUserId(ctx context.Context) int64 {
	userIdVal := ctx.Value(model.UserGroup{})
	if userIdVal == nil {
		return 0
	}
	return gconv.Int64(userIdVal)
}

func normalizeQaKnowledgeCode(code string) string {
	switch strings.TrimSpace(code) {
	case "policy_files":
		return "policy"
	case "laws":
		return "manual"
	default:
		return strings.TrimSpace(code)
	}
}

func accessibleKnowledgeCodes(ctx context.Context, userId int64, requested string) ([]string, error) {
	requestedSet := requestedKnowledgeCodeSet(requested)

	// 1. Get all enabled knowledge base codes
	records, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).
		Fields("code").
		Where("enabled = ?", 1).
		OrderAsc("sort").
		All()
	if err != nil {
		return nil, err
	}
	allCodes := make([]string, 0, len(records))
	for _, record := range records {
		code := record["code"].String()
		if len(requestedSet) > 0 && !requestedSet[code] {
			continue
		}
		allCodes = append(allCodes, code)
	}
	if len(allCodes) == 0 {
		if len(requestedSet) > 0 {
			return nil, gerror.New("无权访问指定知识库")
		}
		return []string{}, nil
	}

	adminScoped, adminHasPerm, adminEnabled, qaAllowed := batchKnowledgePermissions(ctx, userId)

	codes := make([]string, 0, len(allCodes))
	for _, code := range allCodes {
		if checkKnowledgeAccess(code, adminScoped, adminHasPerm, adminEnabled, qaAllowed) {
			codes = append(codes, code)
		}
	}
	if len(requestedSet) > 0 && len(codes) != len(requestedSet) {
		return nil, gerror.New("无权访问指定知识库")
	}
	return codes, nil
}

// batchKnowledgePermissions queries all permission data in 2 batch queries.
// For userId <= 0, only public qa permissions are returned.
func batchKnowledgePermissions(ctx context.Context, userId int64) (adminScoped bool, adminHasPerm, adminEnabled, qaAllowed map[string]bool) {
	adminHasPerm = make(map[string]bool)
	adminEnabled = make(map[string]bool)
	qaAllowed = make(map[string]bool)

	if userId > 0 {
		adminPerms, _ := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).
			Fields("knowledge_code, enabled").
			Where("user_id = ?", userId).
			All()
		adminScoped = len(adminPerms) > 0
		for _, perm := range adminPerms {
			rawCode := perm["knowledge_code"].String()
			enabled := perm["enabled"].Int() != 0
			normalized := normalizeQaKnowledgeCode(rawCode)
			adminHasPerm[rawCode] = true
			adminHasPerm[normalized] = true
			adminEnabled[rawCode] = enabled
			adminEnabled[normalized] = enabled
		}
	}

	// Query qa_knowledge_base_permission for public access (user_id=0)
	// and user-specific access when userId > 0
	queryModel := g.DB("master").Model("qa_knowledge_base_permission").Ctx(ctx).
		Fields("knowledge_code").
		Where("user_id = ? AND enabled = ?", 0, 1)
	if userId > 0 {
		queryModel = queryModel.WhereOr("user_id = ? AND enabled = ?", userId, 1)
	}
	qaPerms, _ := queryModel.All()
	for _, perm := range qaPerms {
		qaAllowed[perm["knowledge_code"].String()] = true
	}

	return
}

// checkKnowledgeAccess determines if a user can access a specific knowledge base code.
func checkKnowledgeAccess(code string, adminScoped bool, adminHasPerm, adminEnabled, qaAllowed map[string]bool) bool {
	if adminScoped {
		return adminEnabled[code]
	}
	if adminHasPerm[code] {
		return adminEnabled[code]
	}
	return qaAllowed[code]
}

func requestedKnowledgeCodeSet(requested string) map[string]bool {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return nil
	}
	set := make(map[string]bool)
	for _, part := range strings.Split(requested, ",") {
		code := strings.TrimSpace(part)
		code = normalizeQaKnowledgeCode(code)
		if code == "" || set[code] {
			continue
		}
		set[code] = true
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func canAccessDocument(ctx context.Context, userId int64, documentId int64) bool {
	if documentId <= 0 || userId <= 0 {
		return false
	}
	count, err := g.DB("master").Model("qa_document_permission").Ctx(ctx).
		Where("document_id = ? AND user_id = ? AND enabled = ?", documentId, userId, 1).
		Count()
	if err == nil && count > 0 {
		return true
	}
	publicCount, err := g.DB("master").Model("qa_document_permission").Ctx(ctx).
		Where("document_id = ? AND user_id = ? AND enabled = ?", documentId, 0, 1).
		Count()
	return err == nil && publicCount > 0
}

func retrieveTerms(question string) []string {
	normalized := strings.TrimSpace(question)
	terms := make([]string, 0, 8)
	if normalized != "" {
		terms = append(terms, strings.ToLower(normalized))
	}
	for _, part := range strings.FieldsFunc(normalized, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("，。！？；：、,.!?;:()（）[]【】\"'“”", r)
	}) {
		part = strings.TrimSpace(part)
		if len([]rune(part)) >= 2 {
			terms = append(terms, strings.ToLower(part))
		}
	}
	for _, term := range retrieveCJKBigrams(normalized, 8) {
		terms = append(terms, term)
	}
	return uniqueStrings(terms)
}

func retrieveSQLTerms(question string, terms []string) []string {
	candidates := make([]string, 0, len(terms)+8)
	for _, term := range terms {
		term = strings.TrimSpace(strings.ToLower(term))
		runeCount := len([]rune(term))
		if runeCount >= 2 && runeCount <= 32 {
			candidates = append(candidates, term)
		}
	}
	for _, term := range retrieveCJKBigrams(question, 8) {
		candidates = append(candidates, term)
	}
	candidates = uniqueStrings(candidates)
	if len(candidates) > 8 {
		return candidates[:8]
	}
	return candidates
}

func retrieveCJKBigrams(text string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	var token []rune
	terms := make([]string, 0, limit)
	flush := func() {
		if len(token) < 2 || len(terms) >= limit {
			token = token[:0]
			return
		}
		if len(token) <= 4 {
			terms = append(terms, strings.ToLower(string(token)))
			token = token[:0]
			return
		}
		for i := 0; i+1 < len(token) && len(terms) < limit; i += 2 {
			terms = append(terms, strings.ToLower(string(token[i:i+2])))
		}
		token = token[:0]
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			token = append(token, r)
			continue
		}
		flush()
	}
	flush()
	return uniqueStrings(terms)
}

func retrieveScore(question string, terms []string, content string) float64 {
	contentLower := strings.ToLower(content)
	score := 0.0
	for _, term := range terms {
		if term == "" {
			continue
		}
		count := strings.Count(contentLower, term)
		if count > 0 {
			weight := 10.0
			if term == strings.ToLower(strings.TrimSpace(question)) {
				weight = 20.0
			}
			score += float64(count) * weight
		}
	}
	if score > 0 {
		score += 1.0 / float64(len([]rune(content))+1)
	}
	return score
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func CreateQaTables(ctx context.Context, db gdb.DB) error {
	for _, sql := range mysqlQaTableSQL() {
		if _, err := db.Exec(ctx, sql); err != nil {
			return err
		}
	}
	// Performance indexes for retrieval optimization
	indexes := []struct {
		table string
		name  string
		sql   string
	}{
		{"qa_document_segment", "idx_qa_segment_kb_id", "ADD INDEX idx_qa_segment_kb_id (knowledge_code, id)"},
		{"qa_document", "idx_qa_doc_status", "ADD INDEX idx_qa_doc_status (id, status)"},
		{"qa_document_permission", "idx_qa_doc_perm_user", "ADD INDEX idx_qa_doc_perm_user (user_id, enabled, document_id)"},
		{"admin_user_knowledge_permission", "idx_admin_kb_perm_user", "ADD INDEX idx_admin_kb_perm_user (user_id, knowledge_code)"},
		{"qa_sync_task", "idx_qa_sync_task_status", "ADD INDEX idx_qa_sync_task_status (status, update_time)"},
		{"qa_sync_log", "idx_qa_sync_log_task", "ADD INDEX idx_qa_sync_log_task (task_id, status)"},
	}
	for _, idx := range indexes {
		// Use ALTER TABLE ... ADD INDEX which is widely supported;
		// ignore duplicate key errors since index may already exist
		if _, err := db.Exec(ctx, fmt.Sprintf("ALTER TABLE %s %s", idx.table, idx.sql)); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				consts.Logger.Debugf(ctx, "index %s creation skipped: %s", idx.name, err.Error())
			}
		}
	}
	migrations := []struct {
		table string
		sql   string
	}{
		{"qa_sync_task", "ADD COLUMN success_count INT NOT NULL DEFAULT 0"},
		{"qa_sync_task", "ADD COLUMN failure_count INT NOT NULL DEFAULT 0"},
		{"qa_sync_task", "ADD COLUMN skipped_count INT NOT NULL DEFAULT 0"},
		{"qa_sync_task", "ADD COLUMN started_at INT NOT NULL DEFAULT 0"},
		{"qa_sync_task", "ADD COLUMN finished_at INT NOT NULL DEFAULT 0"},
	}
	for _, item := range migrations {
		if _, err := db.Exec(ctx, fmt.Sprintf("ALTER TABLE %s %s", item.table, item.sql)); err != nil {
			errText := strings.ToLower(err.Error())
			if !strings.Contains(errText, "duplicate") {
				consts.Logger.Debugf(ctx, "qa migration skipped: table=%s sql=%s err=%s", item.table, item.sql, err.Error())
			}
		}
	}
	return nil
}

func SeedQaTables(ctx context.Context, db gdb.DB) error {
	count, err := db.Model("qa_knowledge_base").Ctx(ctx).Count()
	if err != nil {
		return err
	}
	now := int(gtime.Timestamp())
	bases := []g.Map{
		{"code": "policy", "name": "政策制度库", "description": "本地政策、制度、规范类文档知识库", "sort": 10},
		{"code": "manual", "name": "业务手册库", "description": "本地业务流程、操作手册类文档知识库", "sort": 20},
	}
	if count == 0 {
		for _, item := range bases {
			if _, err = db.Model("qa_knowledge_base").Ctx(ctx).Data(g.Map{
				"code":            item["code"],
				"name":            item["name"],
				"description":     item["description"],
				"enabled":         1,
				"sort":            item["sort"],
				"source_provider": "local",
				"external_id":     "",
				"sync_version":    "",
				"last_sync_time":  0,
				"permission_hash": "",
				"create_time":     now,
				"update_time":     now,
			}).Insert(); err != nil {
				return err
			}
			if _, err = db.Model("qa_knowledge_base_permission").Ctx(ctx).Data(g.Map{
				"knowledge_code": item["code"],
				"user_id":        0,
				"enabled":        1,
				"create_time":    now,
				"update_time":    now,
			}).Insert(); err != nil {
				return err
			}
		}
	}
	return seedQaDemoDocuments(ctx, db, now)
}

func seedQaDemoDocuments(ctx context.Context, db gdb.DB, now int) error {
	count, err := db.Model("qa_document").Ctx(ctx).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	docs := []struct {
		KnowledgeCode string
		Title         string
		FileName      string
		Segments      []string
	}{
		{
			KnowledgeCode: "policy",
			Title:         "问答端本地政策示例文档",
			FileName:      "qa-policy-demo.md",
			Segments: []string{
				"问答端用于政策制度、规范文件和业务材料的知识库问答。系统应先检索用户有权限访问的文档，再基于相关片段生成回答。",
				"文档权限需要贯穿知识库列表、RAG 检索、引用定位和原文查看。无权限文档不得参与召回，也不得进入大模型提示词。",
				"引用来源应返回文件ID、段落锚点、页码和原文预览，便于前端跳转定位到原始文档位置。",
			},
		},
		{
			KnowledgeCode: "manual",
			Title:         "问答端业务手册示例文档",
			FileName:      "qa-manual-demo.md",
			Segments: []string{
				"用户进入问答页后，可以选择可访问的文档库，输入问题后由系统执行本地检索并组织上下文。",
				"会话上下文用于支持追问。新对话或重置会话时，应清空当前会话消息和引用记录。",
				"当前阶段暂不对接AIDGP，系统保留source_provider、external_id、sync_version等字段用于后续同步。",
			},
		},
	}
	for _, doc := range docs {
		docId, err := db.Model("qa_document").Ctx(ctx).Data(g.Map{
			"knowledge_code":  doc.KnowledgeCode,
			"title":           doc.Title,
			"file_name":       doc.FileName,
			"file_type":       "md",
			"source_provider": "local",
			"external_id":     "",
			"status":          "active",
			"create_time":     now,
			"update_time":     now,
		}).InsertAndGetId()
		if err != nil {
			return err
		}
		if _, err = db.Model("qa_document_permission").Ctx(ctx).Data(g.Map{
			"document_id":     docId,
			"user_id":         0,
			"enabled":         1,
			"source_provider": "local",
			"external_id":     "",
			"create_time":     now,
			"update_time":     now,
		}).Insert(); err != nil {
			return err
		}
		for index, content := range doc.Segments {
			if _, err = db.Model("qa_document_segment").Ctx(ctx).Data(g.Map{
				"document_id":    docId,
				"knowledge_code": doc.KnowledgeCode,
				"segment_index":  index + 1,
				"content":        content,
				"page":           index + 1,
				"anchor":         fmt.Sprintf("p%d-s1", index+1),
				"embedding_id":   "",
				"create_time":    now,
			}).Insert(); err != nil {
				return err
			}
		}
	}
	return nil
}

func mysqlQaTableSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS qa_knowledge_base (
code VARCHAR(64) PRIMARY KEY,
name VARCHAR(128) NOT NULL,
description TEXT,
enabled TINYINT NOT NULL DEFAULT 1,
sort INT NOT NULL DEFAULT 0,
source_provider VARCHAR(32) NOT NULL DEFAULT 'local',
external_id VARCHAR(128),
sync_version VARCHAR(64),
last_sync_time INT NOT NULL DEFAULT 0,
permission_hash VARCHAR(128),
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_source_external (source_provider, external_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_knowledge_base_permission (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
knowledge_code VARCHAR(64) NOT NULL,
user_id INT NOT NULL DEFAULT 0,
enabled TINYINT NOT NULL DEFAULT 1,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_qa_kb_user (knowledge_code, user_id),
INDEX idx_user_enabled (user_id, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_document (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
knowledge_code VARCHAR(64) NOT NULL,
title VARCHAR(255) NOT NULL,
file_name VARCHAR(255),
file_type VARCHAR(32),
source_provider VARCHAR(32) NOT NULL DEFAULT 'local',
external_id VARCHAR(128),
status VARCHAR(32) NOT NULL DEFAULT 'active',
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_knowledge_status (knowledge_code, status),
INDEX idx_source_external (source_provider, external_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_document_segment (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
document_id BIGINT NOT NULL,
knowledge_code VARCHAR(64) NOT NULL,
segment_index INT NOT NULL,
content LONGTEXT NOT NULL,
page INT NOT NULL DEFAULT 0,
anchor VARCHAR(128),
embedding_id VARCHAR(128),
create_time INT NOT NULL,
INDEX idx_document_segment (document_id, segment_index),
FULLTEXT KEY ft_content (content)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_document_permission (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
document_id BIGINT NOT NULL,
user_id INT NOT NULL DEFAULT 0,
enabled TINYINT NOT NULL DEFAULT 1,
source_provider VARCHAR(32) NOT NULL DEFAULT 'local',
external_id VARCHAR(128),
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_qa_doc_user (document_id, user_id),
INDEX idx_user_enabled (user_id, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_session (
session_id VARCHAR(64) PRIMARY KEY,
user_id INT NOT NULL,
knowledge_code VARCHAR(64),
title VARCHAR(128),
status VARCHAR(32) NOT NULL DEFAULT 'active',
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_user_update (user_id, update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_message (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
session_id VARCHAR(64) NOT NULL,
user_id INT NOT NULL,
knowledge_code VARCHAR(64),
role VARCHAR(32) NOT NULL,
content LONGTEXT NOT NULL,
create_time INT NOT NULL,
INDEX idx_session_id (session_id),
INDEX idx_user_session (user_id, session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_citation (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
session_id VARCHAR(64) NOT NULL,
user_id INT NOT NULL,
message_id BIGINT NOT NULL DEFAULT 0,
document_id BIGINT NOT NULL,
segment_id BIGINT NOT NULL,
preview_text TEXT,
create_time INT NOT NULL,
INDEX idx_session_id (session_id),
INDEX idx_segment_id (segment_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_question_stat (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
user_id INT NOT NULL,
knowledge_code VARCHAR(64),
question VARCHAR(255) NOT NULL,
hit_count INT NOT NULL DEFAULT 1,
last_asked_at INT NOT NULL,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_user_kb_question (user_id, knowledge_code, question),
INDEX idx_user_kb_count (user_id, knowledge_code, hit_count)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_sync_task (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
provider VARCHAR(32) NOT NULL DEFAULT 'local',
sync_type VARCHAR(64) NOT NULL,
status VARCHAR(32) NOT NULL DEFAULT 'pending',
message TEXT,
success_count INT NOT NULL DEFAULT 0,
failure_count INT NOT NULL DEFAULT 0,
skipped_count INT NOT NULL DEFAULT 0,
started_at INT NOT NULL DEFAULT 0,
finished_at INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_provider_type (provider, sync_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS qa_sync_log (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
task_id BIGINT NOT NULL,
provider VARCHAR(32) NOT NULL DEFAULT 'local',
sync_type VARCHAR(64) NOT NULL,
external_id VARCHAR(128),
local_id VARCHAR(128),
action VARCHAR(32) NOT NULL,
status VARCHAR(32) NOT NULL,
message TEXT,
create_time INT NOT NULL,
INDEX idx_task_id (task_id),
INDEX idx_provider_type (provider, sync_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
}
