package v1

import (
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

type TopicPermission struct {
	Topic   string `json:"topic"`
	Enabled bool   `json:"enabled"`
}

type KnowledgePermission struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	DocType string `json:"docType"`
}

type AdminLoginReq struct {
	g.Meta   `path:"/admin/login" method:"post" tags:"V1/Admin" sm:"admin login" noAuth:"true"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminLoginRes struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

type AdminProfileReq struct {
	g.Meta `path:"/admin/profile" method:"get" tags:"V1/Admin" sm:"admin profile"`
}

type AdminProfileRes struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AdminUserItem struct {
	UserId            int                   `json:"userId"`
	Username          string                `json:"username"`
	DisplayName       string                `json:"displayName"`
	Department        string                `json:"department"`
	Enabled           bool                  `json:"enabled"`
	RuleLevel         int                   `json:"ruleLevel"`
	PermissionVersion int                   `json:"permissionVersion"`
	Permissions       []TopicPermission     `json:"permissions"`
	QaPermissions     []KnowledgePermission `json:"qaPermissions"`
	LastLoginAt       int                   `json:"lastLoginAt"`
	UpdateTime        int                   `json:"updateTime"`
}

type AdminUsersReq struct {
	g.Meta `path:"/admin/users" method:"get" tags:"V1/Admin" sm:"admin users"`
}

type AdminUsersRes struct {
	List []AdminUserItem `json:"list"`
}

type AdminKnowledgeBasesReq struct {
	g.Meta `path:"/admin/knowledge-bases" method:"get" tags:"V1/Admin" sm:"admin knowledge bases"`
}

type AdminKnowledgeBasesRes struct {
	List []KnowledgePermission `json:"list"`
}

type AdminUpdateUserPermissionsReq struct {
	g.Meta        `path:"/admin/users/{id}/permissions" method:"put" tags:"V1/Admin" sm:"update user permissions"`
	Id            int                   `json:"id" p:"id"`
	Permissions   []TopicPermission     `json:"permissions"`
	QaPermissions []KnowledgePermission `json:"qaPermissions"`
	RuleLevel     int                   `json:"ruleLevel"`
}

type AdminUpdateUserPermissionsRes struct {
	User AdminUserItem `json:"user"`
}

type AdminUpdateUserStatusReq struct {
	g.Meta  `path:"/admin/users/{id}/status" method:"put" tags:"V1/Admin" sm:"update user status"`
	Id      int  `json:"id" p:"id"`
	Enabled bool `json:"enabled"`
}

type AdminUpdateUserStatusRes struct {
	User AdminUserItem `json:"user"`
}

type ExampleQuestionItem struct {
	Id          int    `json:"id"`
	Topic       string `json:"topic"`
	Question    string `json:"question"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
	UpdateTime  int    `json:"updateTime"`
}

type AdminExampleQuestionsReq struct {
	g.Meta `path:"/admin/example-questions" method:"get" tags:"V1/Admin" sm:"example questions"`
	Topic  string `json:"topic" p:"topic"`
}

type AdminExampleQuestionsRes struct {
	List []ExampleQuestionItem `json:"list"`
}

type AdminCreateExampleQuestionReq struct {
	g.Meta      `path:"/admin/example-questions" method:"post" tags:"V1/Admin" sm:"create example question"`
	Topic       string `json:"topic"`
	Question    string `json:"question"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
}

type AdminCreateExampleQuestionRes struct {
	Item ExampleQuestionItem `json:"item"`
}

type AdminUpdateExampleQuestionReq struct {
	g.Meta      `path:"/admin/example-questions/{id}" method:"put" tags:"V1/Admin" sm:"update example question"`
	Id          int    `json:"id" p:"id"`
	Topic       string `json:"topic"`
	Question    string `json:"question"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
}

type AdminUpdateExampleQuestionRes struct {
	Item ExampleQuestionItem `json:"item"`
}

type AdminDeleteExampleQuestionReq struct {
	g.Meta `path:"/admin/example-questions/{id}" method:"delete" tags:"V1/Admin" sm:"delete example question"`
	Id     int `json:"id" p:"id"`
}

type AdminDeleteExampleQuestionRes struct{}

type QuestionCandidateItem struct {
	Id         int    `json:"id"`
	Topic      string `json:"topic"`
	Question   string `json:"question"`
	Status     string `json:"status"`
	Count      int    `json:"count"`
	LastSeenAt int    `json:"lastSeenAt"`
	UpdateTime int    `json:"updateTime"`
}

type AdminQuestionCandidatesReq struct {
	g.Meta `path:"/admin/question-candidates" method:"get" tags:"V1/Admin" sm:"question candidates"`
	Status string `json:"status" p:"status"`
}

type AdminQuestionCandidatesRes struct {
	List []QuestionCandidateItem `json:"list"`
}

type AdminApproveQuestionCandidateReq struct {
	g.Meta `path:"/admin/question-candidates/{id}/approve" method:"put" tags:"V1/Admin" sm:"approve question candidate"`
	Id     int `json:"id" p:"id"`
}

type AdminApproveQuestionCandidateRes struct {
	Item QuestionCandidateItem `json:"item"`
}

type AdminRejectQuestionCandidateReq struct {
	g.Meta `path:"/admin/question-candidates/{id}/reject" method:"put" tags:"V1/Admin" sm:"reject question candidate"`
	Id     int `json:"id" p:"id"`
}

type AdminRejectQuestionCandidateRes struct {
	Item QuestionCandidateItem `json:"item"`
}

type DataSourceItem struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Status     string `json:"status"`
	LatestSync int    `json:"latestSync"`
	UpdateTime int    `json:"updateTime"`
}

type AdminDataSourcesReq struct {
	g.Meta `path:"/admin/data-sources" method:"get" tags:"V1/Admin" sm:"data sources"`
}

type AdminDataSourcesRes struct {
	List []DataSourceItem `json:"list"`
}

type AdminUpdateDataSourceReq struct {
	g.Meta  `path:"/admin/data-sources/{type}/status" method:"put" tags:"V1/Admin" sm:"update data source"`
	Type    string `json:"type" p:"type"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type AdminUpdateDataSourceRes struct {
	Item DataSourceItem `json:"item"`
}

type GridImportItem struct {
	Id           int    `json:"id"`
	Month        string `json:"month"`
	FileName     string `json:"fileName"`
	Status       string `json:"status"`
	TotalRows    int    `json:"totalRows"`
	SuccessRows  int    `json:"successRows"`
	FailedRows   int    `json:"failedRows"`
	Operator     string `json:"operator"`
	CreateTime   int    `json:"createTime"`
	CompleteTime int    `json:"completeTime"`
}

type AdminGridImportUploadReq struct {
	g.Meta    `path:"/admin/grid-data/upload" method:"post" tags:"V1/Admin" sm:"upload grid data"`
	Month     string `json:"month"`
	FileName  string `json:"fileName"`
	TotalRows int    `json:"totalRows"`
	Operator  string `json:"operator"`
}

type AdminGridImportUploadRes struct {
	Item GridImportItem `json:"item"`
}

type AdminGridImportsReq struct {
	g.Meta `path:"/admin/grid-data/imports" method:"get" tags:"V1/Admin" sm:"grid imports"`
}

type AdminGridImportsRes struct {
	List []GridImportItem `json:"list"`
}

type AdminGridImportDetailReq struct {
	g.Meta `path:"/admin/grid-data/imports/{id}" method:"get" tags:"V1/Admin" sm:"grid import detail"`
	Id     int `json:"id" p:"id"`
}

type AdminGridImportDetailRes struct {
	Item GridImportItem `json:"item"`
}

type GridImportErrorItem struct {
	Id       int    `json:"id"`
	ImportId int    `json:"importId"`
	RowIndex int    `json:"rowIndex"`
	Reason   string `json:"reason"`
	RawData  string `json:"rawData"`
}

type AdminGridImportErrorsReq struct {
	g.Meta `path:"/admin/grid-data/imports/{id}/errors" method:"get" tags:"V1/Admin" sm:"grid import errors"`
	Id     int `json:"id" p:"id"`
}

type AdminGridImportErrorsRes struct {
	List []GridImportErrorItem `json:"list"`
}

type AdminGridImportTemplateReq struct {
	g.Meta `path:"/admin/grid-data/template" method:"get" tags:"V1/Admin" sm:"download grid import template"`
}

type AdminGridImportTemplateRes struct{}

type AdminGridImportRollbackReq struct {
	g.Meta    `path:"/admin/grid-data/imports/{id}/rollback" method:"post" tags:"V1/Admin" sm:"rollback grid import"`
	Id       int    `json:"id" p:"id"`
	Operator string `json:"operator"`
}

type AdminGridImportRollbackRes struct {
	Item GridImportItem `json:"item"`
}

type GridImportAuditItem struct {
	Id         int    `json:"id"`
	ImportId   int    `json:"importId"`
	Action     string `json:"action"`
	Operator   string `json:"operator"`
	Detail     string `json:"detail"`
	CreateTime int    `json:"createTime"`
}

type AdminGridImportAuditReq struct {
	g.Meta `path:"/admin/grid-data/imports/{id}/audit" method:"get" tags:"V1/Admin" sm:"grid import audit trail"`
	Id     int `json:"id" p:"id"`
}

type AdminGridImportAuditRes struct {
	List []GridImportAuditItem `json:"list"`
}

type AdminSyncKnowledgeBasesReq struct {
	g.Meta `path:"/admin/sync/knowledge-bases" method:"post" tags:"V1/Admin" sm:"sync knowledge bases"`
}

type AdminSyncDocumentsReq struct {
	g.Meta        `path:"/admin/sync/documents" method:"post" tags:"V1/Admin" sm:"sync documents"`
	KnowledgeCode string `json:"knowledgeCode"`
}

type AdminSyncGridDataReq struct {
	g.Meta `path:"/admin/sync/grid-data" method:"post" tags:"V1/Admin" sm:"sync grid data"`
}

type AdminSyncTrafficDataReq struct {
	g.Meta `path:"/admin/sync/traffic-data" method:"post" tags:"V1/Admin" sm:"sync traffic data"`
}

type AdminRefreshTrafficAggregatesReq struct {
	g.Meta   `path:"/admin/traffic/refresh-aggregates" method:"post" tags:"V1/Admin" sm:"refresh traffic aggregates"`
	DateFrom string `json:"dateFrom" d:"" dc:"起始日期 Y-m-d"`
	DateTo   string `json:"dateTo" d:"" dc:"截止日期 Y-m-d"`
}

type AdminRefreshPopulationAggregatesReq struct {
	g.Meta   `path:"/admin/population/refresh-aggregates" method:"post" tags:"V1/Admin" sm:"refresh population aggregates"`
	DateFrom string `json:"dateFrom" d:"" dc:"起始日期 Y-m-d"`
	DateTo   string `json:"dateTo" d:"" dc:"截止日期 Y-m-d"`
}

type AdminSyncHolidayReq struct {
	g.Meta `path:"/admin/traffic/sync-holidays" method:"post" tags:"V1/Admin" sm:"sync holidays from code"`
}

type AdminRefreshTrafficAggregatesRes struct {
	Ok bool `json:"ok"`
}

type AdminRefreshPopulationAggregatesRes struct {
	Ok bool `json:"ok"`
}

type AdminSyncHolidayRes struct {
	Ok bool `json:"ok"`
}

type AdminPlateVerifyReq struct {
	g.Meta     `path:"/admin/traffic/plate/verify" method:"post" tags:"V1/Admin" sm:"batch verify plate recognition"`
	SampleSize int `json:"sampleSize" d:"500" dc:"抽样数量"`
}

type AdminPlateVerifyRes struct {
	model.VerifyReport
}

type AdminSyncPopulationDataReq struct {
	g.Meta `path:"/admin/sync/population-data" method:"post" tags:"V1/Admin" sm:"sync population data"`
}

type AdminSyncTaskItem struct {
	TaskId       int64  `json:"taskId"`
	Provider     string `json:"provider"`
	SyncType     string `json:"syncType"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	SuccessCount int    `json:"successCount"`
	FailureCount int    `json:"failureCount"`
	SkippedCount int    `json:"skippedCount"`
	StartedAt    int    `json:"startedAt"`
	FinishedAt   int    `json:"finishedAt"`
	CreateTime   int    `json:"createTime"`
	UpdateTime   int    `json:"updateTime"`
}

type AdminSyncRes AdminSyncTaskItem

type AdminSyncStatusReq struct {
	g.Meta   `path:"/admin/sync/status" method:"get" tags:"V1/Admin" sm:"sync status"`
	Limit    int    `json:"limit"`
	Provider string `json:"provider"`
	SyncType string `json:"syncType"`
}

type AdminSyncStatusRes struct {
	Provider string              `json:"provider"`
	Enabled  bool                `json:"enabled"`
	List     []AdminSyncTaskItem `json:"list"`
}

type AdminSyncLogsReq struct {
	g.Meta `path:"/admin/sync/tasks/{taskId}/logs" method:"get" tags:"V1/Admin" sm:"sync logs"`
	TaskId int64  `json:"taskId" p:"taskId"`
	Status string `json:"status"`
	Limit  int    `json:"limit"`
}

type AdminSyncLogItem struct {
	LogId      int64  `json:"logId"`
	TaskId     int64  `json:"taskId"`
	Provider   string `json:"provider"`
	SyncType   string `json:"syncType"`
	ExternalId string `json:"externalId"`
	LocalId    string `json:"localId"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreateTime int    `json:"createTime"`
}

type AdminSyncLogsRes struct {
	TaskId int64              `json:"taskId"`
	List   []AdminSyncLogItem `json:"list"`
}

type AdminSyncRetryReq struct {
	g.Meta `path:"/admin/sync/tasks/{taskId}/retry" method:"post" tags:"V1/Admin" sm:"retry sync task"`
	TaskId int64 `json:"taskId" p:"taskId"`
}

type AdminSyncRetryRes AdminSyncTaskItem

type AdminAidgpConnectionTestReq struct {
	g.Meta `path:"/admin/aidgp/connection-test" method:"post" tags:"V1/Admin" sm:"test AIDGP connection"`
}

type AdminAidgpConnectionTestRes struct {
	Provider string `json:"provider"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	CostMs   int64  `json:"costMs"`
}

type AdminSyncFreshnessReq struct {
	g.Meta `path:"/admin/sync/freshness" method:"get" tags:"V1/Admin" sm:"sync freshness"`
}

type AdminSyncFreshnessItem struct {
	SyncType       string `json:"syncType"`
	LatestTaskAt   int    `json:"latestTaskAt"`
	LatestRecordAt int    `json:"latestRecordAt"`
	RecordCount    int    `json:"recordCount"`
	Status         string `json:"status"`
}

type AdminSyncFreshnessRes struct {
	List []AdminSyncFreshnessItem `json:"list"`
}

type AdminSyncReconcileReq struct {
	g.Meta `path:"/admin/sync/reconcile" method:"get" tags:"V1/Admin" sm:"sync reconcile report"`
}

type AdminSyncReconcileItem struct {
	SyncType    string `json:"syncType"`
	RawCount    int    `json:"rawCount"`
	LocalCount  int    `json:"localCount"`
	Difference  int    `json:"difference"`
	LatestRawAt int    `json:"latestRawAt"`
	Status      string `json:"status"`
}

type AdminSyncReconcileRes struct {
	List []AdminSyncReconcileItem `json:"list"`
}

// CaseListItem ?????
type CaseListItem struct {
	Id                 int    `json:"id"`
	ResponsibilityUnit string `json:"responsibilityUnit"`
	CaseNumber         string `json:"caseNumber"`
	CaseSource         string `json:"caseSource"`
	ReportTime         string `json:"reportTime"`
	PendingStep        string `json:"pendingStep"`
	CaseType           string `json:"caseType"`
	Region             string `json:"region"`
	CaseLocation       string `json:"caseLocation"`
	Description        string `json:"description"`
	CreateTime         string `json:"createTime"`
	UpdateTime         string `json:"updateTime"`
}

type AdminCaseListReq struct {
	g.Meta     `path:"/admin/cases" method:"get" tags:"V1/Admin" sm:"case list"`
	Page       int    `json:"page" d:"1"`
	PageSize   int    `json:"pageSize" d:"20"`
	CaseNumber string `json:"caseNumber"`
	CaseType   string `json:"caseType"`
	Region     string `json:"region"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

type AdminCaseListRes struct {
	List     []CaseListItem `json:"list"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

type AdminCaseDeleteReq struct {
	g.Meta `path:"/admin/cases/{id}" method:"delete" tags:"V1/Admin" sm:"delete case"`
	Id     int `json:"id" p:"id"`
}

type AdminCaseDeleteRes struct{}

type AdminCaseDeleteAllReq struct {
	g.Meta `path:"/admin/cases/all" method:"delete" tags:"V1/Admin" sm:"delete all cases"`
}

type AdminCaseDeleteAllRes struct {
	Deleted int `json:"deleted"`
}

// CaseStatistics ??????
type CaseStatistics struct {
	Total     int            `json:"total"`
	ByType    []CaseTypeStat `json:"byType"`
	ByRegion  []CaseTypeStat `json:"byRegion"`
	BySource  []CaseTypeStat `json:"bySource"`
	ByPending []CaseTypeStat `json:"byPending"`
}

type CaseTypeStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type AdminCaseStatisticsReq struct {
	g.Meta `path:"/admin/cases/statistics" method:"get" tags:"V1/Admin" sm:"case statistics"`
}

type AdminCaseStatisticsRes struct {
	Statistics CaseStatistics `json:"statistics"`
}

type AdminLogItem struct {
	Id         int    `json:"id"`
	LogType    string `json:"logType"`
	Username   string `json:"username"`
	ActionType string `json:"actionType"`
	Content    string `json:"content"`
	Result     string `json:"result"`
	CreateTime int    `json:"createTime"`
}

type AdminLogsReq struct {
	g.Meta  `path:"/admin/logs" method:"get" tags:"V1/Admin" sm:"admin logs"`
	LogType string `json:"logType" p:"logType"`
}

type AdminLogsRes struct {
	List []AdminLogItem `json:"list"`
}

type AdminMetricItem struct {
	ID                int64    `json:"id"`
	Topic             string   `json:"topic"`
	MetricName        string   `json:"metricName"`
	DisplayName       string   `json:"displayName"`
	Description       string   `json:"description"`
	Unit              string   `json:"unit"`
	Dimensions        []string `json:"dimensions"`
	DefaultThreshold  *float64 `json:"defaultThreshold"`
	ThresholdDirection string   `json:"thresholdDirection"`
	RelatedFastPath   *int     `json:"relatedFastPath"`
	ChartTypeHint     string   `json:"chartTypeHint"`
	IsActive          bool     `json:"isActive"`
	CreateTime        int      `json:"createTime"`
	UpdateTime        int      `json:"updateTime"`
}

type AdminMetricsReq struct {
	g.Meta `path:"/admin/metrics" method:"get" tags:"V1/Admin" sm:"list metrics"`
	Topic  string `json:"topic" p:"topic"`
}

type AdminMetricsRes struct {
	List []AdminMetricItem `json:"list"`
}

type AdminMetricCreateReq struct {
	g.Meta             `path:"/admin/metrics" method:"post" tags:"V1/Admin" sm:"create metric"`
	Topic              string    `json:"topic" v:"required"`
	MetricName         string    `json:"metricName" v:"required"`
	DisplayName        string    `json:"displayName" v:"required"`
	Description        string    `json:"description"`
	Unit               string    `json:"unit"`
	Dimensions         []string  `json:"dimensions"`
	DefaultThreshold   *float64  `json:"defaultThreshold"`
	ThresholdDirection string    `json:"thresholdDirection" d:"above"`
	RelatedFastPath    *int      `json:"relatedFastPath"`
	ChartTypeHint      string    `json:"chartTypeHint" d:"line"`
}

type AdminMetricCreateRes struct {
	Id int64 `json:"id"`
}

type AdminMetricUpdateReq struct {
	g.Meta             `path:"/admin/metrics/{id}" method:"put" tags:"V1/Admin" sm:"update metric"`
	Id                 int64
	DisplayName        *string   `json:"displayName"`
	Description         *string   `json:"description"`
	Unit               *string   `json:"unit"`
	Dimensions         *[]string `json:"dimensions"`
	DefaultThreshold   **float64 `json:"defaultThreshold"`
	ThresholdDirection *string   `json:"thresholdDirection"`
	RelatedFastPath    **int     `json:"relatedFastPath"`
	ChartTypeHint      *string   `json:"chartTypeHint"`
}

type AdminMetricUpdateRes struct{}

type AdminMetricDeleteReq struct {
	g.Meta `path:"/admin/metrics/{id}" method:"delete" tags:"V1/Admin" sm:"delete metric"`
	Id     int64
}

type AdminMetricDeleteRes struct{}

type AdminMetricToggleReq struct {
	g.Meta `path:"/admin/metrics/{id}/toggle" method:"put" tags:"V1/Admin" sm:"toggle metric active"`
	Id     int64
}

type AdminMetricToggleRes struct {
	IsActive bool `json:"isActive"`
}

type TopicKnowledgeBindingItem struct {
	Id            int    `json:"id"`
	Topic         string `json:"topic"`
	KnowledgeCode string `json:"knowledgeCode"`
	KnowledgeName string `json:"knowledgeName"`
	Enabled       bool   `json:"enabled"`
	CreateTime     int    `json:"createTime"`
	UpdateTime     int    `json:"updateTime"`
}

type AdminTopicKnowledgeBindingsReq struct {
	g.Meta `path:"/admin/topic-knowledge-bindings" method:"get" tags:"V1/Admin" sm:"list topic-knowledge bindings"`
	Topic  string `json:"topic" p:"topic" dc:"按主题筛选"`
}

type AdminTopicKnowledgeBindingsRes struct {
	List []TopicKnowledgeBindingItem `json:"list"`
}

type AdminBindTopicKnowledgeReq struct {
	g.Meta         `path:"/admin/topic-knowledge-bindings" method:"post" tags:"V1/Admin" sm:"bind knowledge base to topic"`
	Topic          string `json:"topic" v:"required#主题不能为空"`
	KnowledgeCode  string `json:"knowledgeCode" v:"required#知识库编码不能为空"`
}

type AdminBindTopicKnowledgeRes struct {
	Item TopicKnowledgeBindingItem `json:"item"`
}

type AdminUnbindTopicKnowledgeReq struct {
	g.Meta `path:"/admin/topic-knowledge-bindings/{id}" method:"delete" tags:"V1/Admin" sm:"unbind knowledge base from topic"`
	Id     int `json:"id" p:"id"`
}

type AdminUnbindTopicKnowledgeRes struct{}

type AdminToggleTopicKnowledgeReq struct {
	g.Meta  `path:"/admin/topic-knowledge-bindings/{id}/toggle" method:"put" tags:"V1/Admin" sm:"toggle topic-knowledge binding"`
	Id      int  `json:"id" p:"id"`
	Enabled bool `json:"enabled"`
}

type AdminToggleTopicKnowledgeRes struct {
	Enabled bool `json:"enabled"`
}

type AdminUpdateDocumentVersionReq struct {
	g.Meta        `path:"/admin/documents/{id}/version" method:"put" tags:"V1/Admin" sm:"update document version info"`
	Id            int64  `json:"id" p:"id"`
	EffectiveDate string `json:"effectiveDate" dc:"生效日期，格式 YYYY-MM-DD"`
	RepealDate    string `json:"repealDate" dc:"废止日期，格式 YYYY-MM-DD"`
	RepealedBy    string `json:"repealedBy" dc:"废止该文档的文档标题或ID"`
	TitleGroup    string `json:"titleGroup" dc:"同名多版本分组键"`
	Status        string `json:"status" dc:"文档状态：active/repealed/draft"`
}

type AdminUpdateDocumentVersionRes struct {
	DocumentId int64 `json:"documentId"`
}

type AdminDocumentVersionsReq struct {
	g.Meta     `path:"/admin/documents/versions" method:"get" tags:"V1/Admin" sm:"list document version groups"`
	TitleGroup string `json:"titleGroup" p:"titleGroup" dc:"按同名分组键筛选"`
}

type AdminDocumentVersionsRes struct {
	Groups []DocumentVersionGroup `json:"groups"`
}

type DocumentVersionGroup struct {
	TitleGroup string                  `json:"titleGroup"`
	Versions   []DocumentVersionDetail `json:"versions"`
}

type DocumentVersionDetail struct {
	DocumentId    int64  `json:"documentId"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	EffectiveDate string `json:"effectiveDate,omitempty"`
	RepealDate    string `json:"repealDate,omitempty"`
	RepealedBy    string `json:"repealedBy,omitempty"`
}

type AdminDocumentRelationsReq struct {
	g.Meta     `path:"/admin/document-relations" method:"get" tags:"V1/Admin" sm:"list document relations"`
	DocumentId int64  `json:"documentId" p:"documentId" dc:"按文档ID筛选"`
	RelType    string `json:"relType" p:"relType" dc:"按关联类型筛选：reference|supplement|repeal|related"`
}

type AdminDocumentRelationsRes struct {
	List []DocumentRelationItem `json:"list"`
}

type DocumentRelationItem struct {
	Id           int64  `json:"id"`
	FromDocId    int64  `json:"fromDocId"`
	FromDocTitle string `json:"fromDocTitle"`
	ToDocId      int64  `json:"toDocId"`
	ToDocTitle   string `json:"toDocTitle"`
	RelType      string `json:"relType"`
	Description  string `json:"description,omitempty"`
	Enabled      bool   `json:"enabled"`
}

type AdminCreateDocumentRelationReq struct {
	g.Meta      `path:"/admin/document-relations" method:"post" tags:"V1/Admin" sm:"create document relation"`
	FromDocId   int64  `json:"fromDocId" v:"required#来源文档ID不能为空"`
	ToDocId     int64  `json:"toDocId" v:"required#目标文档ID不能为空"`
	RelType     string `json:"relType" v:"required|in:reference,supplement,repeal,related#关联类型不能为空|关联类型须为reference/supplement/repeal/related"`
	Description string `json:"description"`
}

type AdminCreateDocumentRelationRes struct {
	Id int64 `json:"id"`
}

type AdminDeleteDocumentRelationReq struct {
	g.Meta `path:"/admin/document-relations/{id}" method:"delete" tags:"V1/Admin" sm:"delete document relation"`
	Id     int64 `json:"id" p:"id"`
}

type AdminDeleteDocumentRelationRes struct{}

type AdminDocumentRecommendationsReq struct {
	g.Meta     `path:"/admin/documents/{id}/recommendations" method:"get" tags:"V1/Admin" sm:"get related document recommendations"`
	Id         int64 `json:"id" p:"id"`
	TopN       int   `json:"topN" p:"topN" dc:"返回条数，默认5"`
}

type AdminDocumentRecommendationsRes struct {
	List []DocumentRelationItem `json:"list"`
}
