package admin

import (
	"context"

	"ai-chat-sql/api/admin/v1"
)

type IAdminV1 interface {
	AdminLogin(ctx context.Context, req *v1.AdminLoginReq) (res *v1.AdminLoginRes, err error)
	AdminProfile(ctx context.Context, req *v1.AdminProfileReq) (res *v1.AdminProfileRes, err error)
	AdminUsers(ctx context.Context, req *v1.AdminUsersReq) (res *v1.AdminUsersRes, err error)
	AdminKnowledgeBases(ctx context.Context, req *v1.AdminKnowledgeBasesReq) (res *v1.AdminKnowledgeBasesRes, err error)
	AdminUpdateUserPermissions(ctx context.Context, req *v1.AdminUpdateUserPermissionsReq) (res *v1.AdminUpdateUserPermissionsRes, err error)
	AdminUpdateUserStatus(ctx context.Context, req *v1.AdminUpdateUserStatusReq) (res *v1.AdminUpdateUserStatusRes, err error)
	AdminExampleQuestions(ctx context.Context, req *v1.AdminExampleQuestionsReq) (res *v1.AdminExampleQuestionsRes, err error)
	AdminCreateExampleQuestion(ctx context.Context, req *v1.AdminCreateExampleQuestionReq) (res *v1.AdminCreateExampleQuestionRes, err error)
	AdminUpdateExampleQuestion(ctx context.Context, req *v1.AdminUpdateExampleQuestionReq) (res *v1.AdminUpdateExampleQuestionRes, err error)
	AdminDeleteExampleQuestion(ctx context.Context, req *v1.AdminDeleteExampleQuestionReq) (res *v1.AdminDeleteExampleQuestionRes, err error)
	AdminQuestionCandidates(ctx context.Context, req *v1.AdminQuestionCandidatesReq) (res *v1.AdminQuestionCandidatesRes, err error)
	AdminApproveQuestionCandidate(ctx context.Context, req *v1.AdminApproveQuestionCandidateReq) (res *v1.AdminApproveQuestionCandidateRes, err error)
	AdminRejectQuestionCandidate(ctx context.Context, req *v1.AdminRejectQuestionCandidateReq) (res *v1.AdminRejectQuestionCandidateRes, err error)
	AdminDataSources(ctx context.Context, req *v1.AdminDataSourcesReq) (res *v1.AdminDataSourcesRes, err error)
	AdminUpdateDataSource(ctx context.Context, req *v1.AdminUpdateDataSourceReq) (res *v1.AdminUpdateDataSourceRes, err error)
	AdminGridImportUpload(ctx context.Context, req *v1.AdminGridImportUploadReq) (res *v1.AdminGridImportUploadRes, err error)
	AdminGridImports(ctx context.Context, req *v1.AdminGridImportsReq) (res *v1.AdminGridImportsRes, err error)
	AdminGridImportDetail(ctx context.Context, req *v1.AdminGridImportDetailReq) (res *v1.AdminGridImportDetailRes, err error)
	AdminGridImportErrors(ctx context.Context, req *v1.AdminGridImportErrorsReq) (res *v1.AdminGridImportErrorsRes, err error)
	AdminCaseList(ctx context.Context, req *v1.AdminCaseListReq) (res *v1.AdminCaseListRes, err error)
	AdminCaseDelete(ctx context.Context, req *v1.AdminCaseDeleteReq) (res *v1.AdminCaseDeleteRes, err error)
	AdminCaseDeleteAll(ctx context.Context, req *v1.AdminCaseDeleteAllReq) (res *v1.AdminCaseDeleteAllRes, err error)
	AdminCaseStatistics(ctx context.Context, req *v1.AdminCaseStatisticsReq) (res *v1.AdminCaseStatisticsRes, err error)
	AdminTopicKnowledgeBindings(ctx context.Context, req *v1.AdminTopicKnowledgeBindingsReq) (res *v1.AdminTopicKnowledgeBindingsRes, err error)
	AdminCreateTopicKnowledgeBinding(ctx context.Context, req *v1.AdminCreateTopicKnowledgeBindingReq) (res *v1.AdminCreateTopicKnowledgeBindingRes, err error)
	AdminToggleTopicKnowledgeBinding(ctx context.Context, req *v1.AdminToggleTopicKnowledgeBindingReq) (res *v1.AdminToggleTopicKnowledgeBindingRes, err error)
	AdminDeleteTopicKnowledgeBinding(ctx context.Context, req *v1.AdminDeleteTopicKnowledgeBindingReq) (res *v1.AdminDeleteTopicKnowledgeBindingRes, err error)
	AdminDocumentRelations(ctx context.Context, req *v1.AdminDocumentRelationsReq) (res *v1.AdminDocumentRelationsRes, err error)
	AdminCreateDocumentRelation(ctx context.Context, req *v1.AdminCreateDocumentRelationReq) (res *v1.AdminCreateDocumentRelationRes, err error)
	AdminDeleteDocumentRelation(ctx context.Context, req *v1.AdminDeleteDocumentRelationReq) (res *v1.AdminDeleteDocumentRelationRes, err error)
	AdminAutoDocumentRelation(ctx context.Context, req *v1.AdminAutoDocumentRelationReq) (res *v1.AdminAutoDocumentRelationRes, err error)
	AdminDocumentCompliance(ctx context.Context, req *v1.AdminDocumentComplianceReq) (res *v1.AdminDocumentComplianceRes, err error)
	AdminMetrics(ctx context.Context, req *v1.AdminMetricsReq) (res *v1.AdminMetricsRes, err error)
	AdminCreateMetric(ctx context.Context, req *v1.AdminCreateMetricReq) (res *v1.AdminCreateMetricRes, err error)
	AdminUpdateMetric(ctx context.Context, req *v1.AdminUpdateMetricReq) (res *v1.AdminUpdateMetricRes, err error)
	AdminDeleteMetric(ctx context.Context, req *v1.AdminDeleteMetricReq) (res *v1.AdminDeleteMetricRes, err error)
	AdminToggleMetric(ctx context.Context, req *v1.AdminToggleMetricReq) (res *v1.AdminToggleMetricRes, err error)
}
