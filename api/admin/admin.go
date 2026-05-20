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
	AdminGridImportTemplate(ctx context.Context, req *v1.AdminGridImportTemplateReq) (res *v1.AdminGridImportTemplateRes, err error)
	AdminGridImportRollback(ctx context.Context, req *v1.AdminGridImportRollbackReq) (res *v1.AdminGridImportRollbackRes, err error)
	AdminGridImportAudit(ctx context.Context, req *v1.AdminGridImportAuditReq) (res *v1.AdminGridImportAuditRes, err error)
	AdminSyncKnowledgeBases(ctx context.Context, req *v1.AdminSyncKnowledgeBasesReq) (res *v1.AdminSyncRes, err error)
	AdminSyncDocuments(ctx context.Context, req *v1.AdminSyncDocumentsReq) (res *v1.AdminSyncRes, err error)
	AdminSyncGridData(ctx context.Context, req *v1.AdminSyncGridDataReq) (res *v1.AdminSyncRes, err error)
	AdminSyncTrafficData(ctx context.Context, req *v1.AdminSyncTrafficDataReq) (res *v1.AdminSyncRes, err error)
	AdminSyncPopulationData(ctx context.Context, req *v1.AdminSyncPopulationDataReq) (res *v1.AdminSyncRes, err error)
	AdminSyncStatus(ctx context.Context, req *v1.AdminSyncStatusReq) (res *v1.AdminSyncStatusRes, err error)
	AdminSyncLogs(ctx context.Context, req *v1.AdminSyncLogsReq) (res *v1.AdminSyncLogsRes, err error)
	AdminSyncRetry(ctx context.Context, req *v1.AdminSyncRetryReq) (res *v1.AdminSyncRetryRes, err error)
	AdminAidgpConnectionTest(ctx context.Context, req *v1.AdminAidgpConnectionTestReq) (res *v1.AdminAidgpConnectionTestRes, err error)
	AdminSyncFreshness(ctx context.Context, req *v1.AdminSyncFreshnessReq) (res *v1.AdminSyncFreshnessRes, err error)
	AdminSyncReconcile(ctx context.Context, req *v1.AdminSyncReconcileReq) (res *v1.AdminSyncReconcileRes, err error)
	AdminCaseList(ctx context.Context, req *v1.AdminCaseListReq) (res *v1.AdminCaseListRes, err error)
	AdminCaseDelete(ctx context.Context, req *v1.AdminCaseDeleteReq) (res *v1.AdminCaseDeleteRes, err error)
	AdminCaseDeleteAll(ctx context.Context, req *v1.AdminCaseDeleteAllReq) (res *v1.AdminCaseDeleteAllRes, err error)
	AdminCaseStatistics(ctx context.Context, req *v1.AdminCaseStatisticsReq) (res *v1.AdminCaseStatisticsRes, err error)
}
