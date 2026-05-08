package qa

import (
	"context"

	"ai-chat-sql/api/qa/v1"
)

type IQaV1 interface {
	KnowledgeBases(ctx context.Context, req *v1.KnowledgeBasesReq) (res *v1.KnowledgeBasesRes, err error)
	Retrieve(ctx context.Context, req *v1.RetrieveReq) (res *v1.RetrieveRes, err error)
	Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error)
	CitationDetail(ctx context.Context, req *v1.CitationDetailReq) (res *v1.CitationDetailRes, err error)
	DocumentView(ctx context.Context, req *v1.DocumentViewReq) (res *v1.DocumentViewRes, err error)
	PopularQuestions(ctx context.Context, req *v1.PopularQuestionsReq) (res *v1.PopularQuestionsRes, err error)
	WebSearch(ctx context.Context, req *v1.WebSearchReq) (res *v1.WebSearchRes, err error)
	SyncKnowledgeBases(ctx context.Context, req *v1.QaSyncReq) (res *v1.QaSyncRes, err error)
	SyncDocuments(ctx context.Context, req *v1.QaSyncDocumentsReq) (res *v1.QaSyncRes, err error)
	SyncPermissions(ctx context.Context, req *v1.QaSyncPermissionsReq) (res *v1.QaSyncRes, err error)
	SyncStatus(ctx context.Context, req *v1.QaSyncStatusReq) (res *v1.QaSyncStatusRes, err error)
	SessionReset(ctx context.Context, req *v1.SessionResetReq) (res *v1.SessionResetRes, err error)
}
