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
	Token     string `json:"token" dc:"管理员Token"`
	Username  string `json:"username" dc:"管理员用户名"`
	UserToken string `json:"userToken" dc:"用户端Token"`
}

type AdminProfileReq struct {
	g.Meta `path:"/admin/profile" method:"get" tags:"V1/Admin" sm:"admin profile"`
}

type AdminProfileRes struct {
	Username string `json:"username" dc:"管理员用户名"`
	Role     string `json:"role" dc:"管理员角色"`
}

type AdminUserItem struct {
	UserId            int                   `json:"userId" dc:"用户ID"`
	Username          string                `json:"username" dc:"用户名"`
	DisplayName       string                `json:"displayName" dc:"显示名称"`
	Department        string                `json:"department" dc:"部门"`
	Enabled           bool                  `json:"enabled" dc:"是否启用"`
	RuleLevel         int                   `json:"ruleLevel" dc:"权限等级"`
	PermissionVersion int                   `json:"permissionVersion" dc:"权限版本"`
	Permissions       []TopicPermission     `json:"permissions" dc:"主题权限列表"`
	QaPermissions     []KnowledgePermission `json:"qaPermissions" dc:"问答知识库权限"`
	LastLoginAt       int                   `json:"lastLoginAt" dc:"最后登录时间"`
	UpdateTime        int                   `json:"updateTime" dc:"更新时间"`
}

type AdminUsersReq struct {
	g.Meta   `path:"/admin/users" method:"get" tags:"V1/Admin" sm:"admin users"`
	Page     int `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize int `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminUsersRes struct {
	List     []AdminUserItem `json:"list" dc:"用户列表"`
	Total    int             `json:"total" dc:"总记录数"`
	Page     int             `json:"page" dc:"当前页码"`
	PageSize int             `json:"pageSize" dc:"每页条数"`
}

type AdminKnowledgeBasesReq struct {
	g.Meta   `path:"/admin/knowledge-bases" method:"get" tags:"V1/Admin" sm:"admin knowledge bases"`
	Page     int `json:"page" p:"page" d:"1" v:"min:1"`
	PageSize int `json:"pageSize" p:"pageSize" d:"20" v:"min:1|max:200"`
}

type AdminKnowledgeBasesRes struct {
	List     []KnowledgePermission `json:"list" dc:"知识库列表"`
	Total    int                   `json:"total" dc:"总记录数"`
	Page     int                   `json:"page" dc:"当前页码"`
	PageSize int                   `json:"pageSize" dc:"每页条数"`
}

type AdminUpdateUserPermissionsReq struct {
	g.Meta        `path:"/admin/users/{id}/permissions" method:"put" tags:"V1/Admin" sm:"update user permissions"`
	Id            int                   `json:"id" p:"id" v:"min:1#用户ID不能为空"`
	Permissions   []TopicPermission     `json:"permissions"`
	QaPermissions []KnowledgePermission `json:"qaPermissions"`
	RuleLevel     int                   `json:"ruleLevel"`
}

type AdminUpdateUserPermissionsRes struct {
	User AdminUserItem `json:"user" dc:"用户信息"`
}

type AdminUpdateUserStatusReq struct {
	g.Meta  `path:"/admin/users/{id}/status" method:"put" tags:"V1/Admin" sm:"update user status"`
	Id      int  `json:"id" p:"id" v:"min:1#用户ID不能为空"`
	Enabled bool `json:"enabled"`
}

type AdminUpdateUserStatusRes struct {
	User AdminUserItem `json:"user" dc:"用户信息"`
}

type ExampleQuestionItem struct {
	Id          int    `json:"id" dc:"记录ID"`
	Topic       string `json:"topic" dc:"主题"`
	Question    string `json:"question" dc:"问题内容"`
	Description string `json:"description" dc:"描述"`
	Enabled     bool   `json:"enabled" dc:"是否启用"`
	Sort        int    `json:"sort" dc:"排序"`
	UpdateTime  int    `json:"updateTime" dc:"更新时间"`
}

type AdminExampleQuestionsReq struct {
	g.Meta   `path:"/admin/example-questions" method:"get" tags:"V1/Admin" sm:"example questions"`
	Topic    string `json:"topic" p:"topic"`
	Page     int    `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize int    `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminExampleQuestionsRes struct {
	List     []ExampleQuestionItem `json:"list" dc:"示例问题列表"`
	Total    int                   `json:"total" dc:"总记录数"`
	Page     int                   `json:"page" dc:"当前页码"`
	PageSize int                   `json:"pageSize" dc:"每页条数"`
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
	Item ExampleQuestionItem `json:"item" dc:"新建的示例问题"`
}

type AdminUpdateExampleQuestionReq struct {
	g.Meta      `path:"/admin/example-questions/{id}" method:"put" tags:"V1/Admin" sm:"update example question"`
	Id          int    `json:"id" p:"id" v:"min:1#示例问题ID不能为空"`
	Topic       string `json:"topic"`
	Question    string `json:"question"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
}

type AdminUpdateExampleQuestionRes struct {
	Item ExampleQuestionItem `json:"item" dc:"更新后的示例问题"`
}

type AdminDeleteExampleQuestionReq struct {
	g.Meta `path:"/admin/example-questions/{id}" method:"delete" tags:"V1/Admin" sm:"delete example question"`
	Id     int `json:"id" p:"id" v:"min:1#示例问题ID不能为空"`
}

type AdminDeleteExampleQuestionRes struct{}

type QuestionCandidateItem struct {
	Id         int    `json:"id" dc:"记录ID"`
	Topic      string `json:"topic" dc:"主题"`
	Question   string `json:"question" dc:"问题内容"`
	Status     string `json:"status" dc:"状态"`
	Count      int    `json:"count" dc:"出现次数"`
	LastSeenAt int    `json:"lastSeenAt" dc:"最近出现时间"`
	UpdateTime int    `json:"updateTime" dc:"更新时间"`
}

type AdminQuestionCandidatesReq struct {
	g.Meta   `path:"/admin/question-candidates" method:"get" tags:"V1/Admin" sm:"question candidates"`
	Status   string `json:"status" p:"status"`
	Page     int    `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize int    `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminQuestionCandidatesRes struct {
	List     []QuestionCandidateItem `json:"list" dc:"候选问题列表"`
	Total    int                     `json:"total" dc:"总记录数"`
	Page     int                     `json:"page" dc:"当前页码"`
	PageSize int                     `json:"pageSize" dc:"每页条数"`
}

type AdminApproveQuestionCandidateReq struct {
	g.Meta `path:"/admin/question-candidates/{id}/approve" method:"put" tags:"V1/Admin" sm:"approve question candidate"`
	Id     int `json:"id" p:"id" v:"min:1#候选问题ID不能为空"`
}

type AdminApproveQuestionCandidateRes struct {
	Item QuestionCandidateItem `json:"item" dc:"审批后的候选问题"`
}

type AdminRejectQuestionCandidateReq struct {
	g.Meta `path:"/admin/question-candidates/{id}/reject" method:"put" tags:"V1/Admin" sm:"reject question candidate"`
	Id     int `json:"id" p:"id" v:"min:1#候选问题ID不能为空"`
}

type AdminRejectQuestionCandidateRes struct {
	Item QuestionCandidateItem `json:"item" dc:"驳回后的候选问题"`
}

type DataSourceItem struct {
	Type       string `json:"type" dc:"数据源类型"`
	Name       string `json:"name" dc:"名称"`
	Enabled    bool   `json:"enabled" dc:"是否启用"`
	Status     string `json:"status" dc:"状态"`
	LatestSync int    `json:"latestSync" dc:"最近同步时间"`
	UpdateTime int    `json:"updateTime" dc:"更新时间"`
}

type AdminDataSourcesReq struct {
	g.Meta   `path:"/admin/data-sources" method:"get" tags:"V1/Admin" sm:"data sources"`
	Page     int `json:"page" p:"page" d:"1" v:"min:1"`
	PageSize int `json:"pageSize" p:"pageSize" d:"20" v:"min:1|max:200"`
}

type AdminDataSourcesRes struct {
	List     []DataSourceItem `json:"list" dc:"数据源列表"`
	Total    int              `json:"total" dc:"总记录数"`
	Page     int              `json:"page" dc:"当前页码"`
	PageSize int              `json:"pageSize" dc:"每页条数"`
}

type AdminUpdateDataSourceReq struct {
	g.Meta  `path:"/admin/data-sources/{type}/status" method:"put" tags:"V1/Admin" sm:"update data source"`
	Type    string `json:"type" p:"type"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status" dc:"状态"`
}

type AdminUpdateDataSourceRes struct {
	Item DataSourceItem `json:"item" dc:"数据源信息"`
}

type GridImportItem struct {
	Id           int    `json:"id" dc:"记录ID"`
	Month        string `json:"month" dc:"月份"`
	FileName     string `json:"fileName" dc:"文件名"`
	Status       string `json:"status" dc:"导入状态"`
	TotalRows    int    `json:"totalRows" dc:"总行数"`
	SuccessRows  int    `json:"successRows" dc:"成功行数"`
	FailedRows   int    `json:"failedRows" dc:"失败行数"`
	Operator     string `json:"operator" dc:"操作人"`
	CreateTime   int    `json:"createTime" dc:"创建时间"`
	CompleteTime int    `json:"completeTime" dc:"完成时间"`
}

type AdminGridImportUploadReq struct {
	g.Meta    `path:"/admin/grid-data/upload" method:"post" tags:"V1/Admin" sm:"upload grid data"`
	Month     string `json:"month"`
	FileName  string `json:"fileName"`
	TotalRows int    `json:"totalRows"`
	Operator  string `json:"operator"`
}

type AdminGridImportUploadRes struct {
	Item GridImportItem `json:"item" dc:"新建的导入记录"`
}

type AdminGridImportsReq struct {
	g.Meta   `path:"/admin/grid-data/imports" method:"get" tags:"V1/Admin" sm:"grid imports"`
	Page     int `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize int `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminGridImportsRes struct {
	List     []GridImportItem `json:"list" dc:"导入记录列表"`
	Total    int              `json:"total" dc:"总记录数"`
	Page     int              `json:"page" dc:"当前页码"`
	PageSize int              `json:"pageSize" dc:"每页条数"`
}

type AdminGridImportDetailReq struct {
	g.Meta `path:"/admin/grid-data/imports/{id}" method:"get" tags:"V1/Admin" sm:"grid import detail"`
	Id     int `json:"id" p:"id" v:"min:1#导入记录ID不能为空"`
}

type AdminGridImportDetailRes struct {
	Item GridImportItem `json:"item" dc:"导入记录详情"`
}

type GridImportErrorItem struct {
	Id       int    `json:"id" dc:"记录ID"`
	ImportId int    `json:"importId" dc:"导入批次ID"`
	RowIndex int    `json:"rowIndex" dc:"行号"`
	Reason   string `json:"reason" dc:"失败原因"`
	RawData  string `json:"rawData" dc:"原始数据"`
}

type AdminGridImportErrorsReq struct {
	g.Meta `path:"/admin/grid-data/imports/{id}/errors" method:"get" tags:"V1/Admin" sm:"grid import errors"`
	Id     int `json:"id" p:"id" v:"min:1#导入记录ID不能为空"`
}

type AdminGridImportErrorsRes struct {
	List []GridImportErrorItem `json:"list" dc:"导入错误列表"`
}

type AdminGridImportTemplateReq struct {
	g.Meta `path:"/admin/grid-data/template" method:"get" tags:"V1/Admin" sm:"download grid import template"`
}

type AdminGridImportTemplateRes struct{}

type AdminGridImportRollbackReq struct {
	g.Meta    `path:"/admin/grid-data/imports/{id}/rollback" method:"post" tags:"V1/Admin" sm:"rollback grid import"`
	Id       int    `json:"id" p:"id" v:"min:1#导入记录ID不能为空"`
	Operator string `json:"operator"`
}

type AdminGridImportRollbackRes struct {
	Item GridImportItem `json:"item" dc:"导入记录详情"`
}

type GridImportAuditItem struct {
	Id         int    `json:"id" dc:"记录ID"`
	ImportId   int    `json:"importId" dc:"导入批次ID"`
	Action     string `json:"action" dc:"操作类型"`
	Operator   string `json:"operator" dc:"操作人"`
	Detail     string `json:"detail" dc:"操作详情"`
	CreateTime int    `json:"createTime" dc:"创建时间"`
}

type AdminGridImportAuditReq struct {
	g.Meta `path:"/admin/grid-data/imports/{id}/audit" method:"get" tags:"V1/Admin" sm:"grid import audit trail"`
	Id     int `json:"id" p:"id" v:"min:1#导入记录ID不能为空"`
}

type AdminGridImportAuditRes struct {
	List []GridImportAuditItem `json:"list" dc:"审计轨迹列表"`
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
	Ok bool `json:"ok" dc:"操作是否成功"`
}

type AdminRefreshPopulationAggregatesRes struct {
	Ok bool `json:"ok" dc:"操作是否成功"`
}

type AdminSyncHolidayRes struct {
	Ok bool `json:"ok" dc:"操作是否成功"`
}

type AdminPlateVerifyReq struct {
	g.Meta     `path:"/admin/traffic/plate/verify" method:"post" tags:"V1/Admin" sm:"batch verify plate recognition"`
	SampleSize int `json:"sampleSize" d:"500" dc:"抽样数量" v:"min:1|max:5000"`
}

type AdminPlateVerifyRes struct {
	model.VerifyReport
}

type AdminSyncPopulationDataReq struct {
	g.Meta `path:"/admin/sync/population-data" method:"post" tags:"V1/Admin" sm:"sync population data"`
}

type AdminSyncTaskItem struct {
	TaskId       int64  `json:"taskId" dc:"任务ID"`
	Provider     string `json:"provider" dc:"提供方"`
	SyncType     string `json:"syncType" dc:"同步类型"`
	Status       string `json:"status" dc:"状态"`
	Message      string `json:"message" dc:"消息"`
	SuccessCount int    `json:"successCount" dc:"成功数"`
	FailureCount int    `json:"failureCount" dc:"失败数"`
	SkippedCount int    `json:"skippedCount" dc:"跳过数"`
	StartedAt    int    `json:"startedAt" dc:"开始时间"`
	FinishedAt   int    `json:"finishedAt" dc:"完成时间"`
	CreateTime   int    `json:"createTime" dc:"创建时间"`
	UpdateTime   int    `json:"updateTime" dc:"更新时间"`
}

type AdminSyncRes AdminSyncTaskItem

type AdminSyncStatusReq struct {
	g.Meta   `path:"/admin/sync/status" method:"get" tags:"V1/Admin" sm:"sync status"`
	Limit    int    `json:"limit" v:"min:1|max:100"`
	Provider string `json:"provider"`
	SyncType string `json:"syncType"`
}

type AdminSyncStatusRes struct {
	Provider string              `json:"provider" dc:"提供方"`
	Enabled  bool                `json:"enabled" dc:"是否启用"`
	List     []AdminSyncTaskItem `json:"list" dc:"同步任务列表"`
}

type AdminSyncLogsReq struct {
	g.Meta `path:"/admin/sync/tasks/{taskId}/logs" method:"get" tags:"V1/Admin" sm:"sync logs"`
	TaskId int64  `json:"taskId" p:"taskId" v:"min:1#任务ID不能为空"`
	Status string `json:"status"`
	Limit  int    `json:"limit" v:"min:1|max:500"`
}

type AdminSyncLogItem struct {
	LogId      int64  `json:"logId" dc:"日志ID"`
	TaskId     int64  `json:"taskId" dc:"任务ID"`
	Provider   string `json:"provider" dc:"提供方"`
	SyncType   string `json:"syncType" dc:"同步类型"`
	ExternalId string `json:"externalId" dc:"外部ID"`
	LocalId    string `json:"localId" dc:"本地ID"`
	Action     string `json:"action" dc:"操作"`
	Status     string `json:"status" dc:"状态"`
	Message    string `json:"message" dc:"消息"`
	CreateTime int    `json:"createTime" dc:"创建时间"`
}

type AdminSyncLogsRes struct {
	TaskId int64              `json:"taskId" dc:"任务ID"`
	List   []AdminSyncLogItem `json:"list" dc:"日志列表"`
}

type AdminSyncRetryReq struct {
	g.Meta `path:"/admin/sync/tasks/{taskId}/retry" method:"post" tags:"V1/Admin" sm:"retry sync task"`
	TaskId int64 `json:"taskId" p:"taskId" v:"min:1#任务ID不能为空"`
}

type AdminSyncRetryRes AdminSyncTaskItem

type AdminAidgpConnectionTestReq struct {
	g.Meta `path:"/admin/aidgp/connection-test" method:"post" tags:"V1/Admin" sm:"test AIDGP connection"`
}

type AdminAidgpConnectionTestRes struct {
	Provider string `json:"provider" dc:"提供方"`
	Success  bool   `json:"success" dc:"是否成功"`
	Message  string `json:"message" dc:"消息"`
	CostMs   int64  `json:"costMs" dc:"耗时毫秒"`
}

type AdminSyncFreshnessReq struct {
	g.Meta `path:"/admin/sync/freshness" method:"get" tags:"V1/Admin" sm:"sync freshness"`
}

type AdminSyncFreshnessItem struct {
	SyncType       string `json:"syncType" dc:"同步类型"`
	LatestTaskAt   int    `json:"latestTaskAt" dc:"最近任务时间"`
	LatestRecordAt int    `json:"latestRecordAt" dc:"最近记录时间"`
	RecordCount    int    `json:"recordCount" dc:"记录数"`
	Status         string `json:"status" dc:"状态"`
}

type AdminSyncFreshnessRes struct {
	List []AdminSyncFreshnessItem `json:"list" dc:"新鲜度列表"`
}

type AdminSyncReconcileReq struct {
	g.Meta `path:"/admin/sync/reconcile" method:"get" tags:"V1/Admin" sm:"sync reconcile report"`
}

type AdminSyncReconcileItem struct {
	SyncType    string `json:"syncType" dc:"同步类型"`
	RawCount    int    `json:"rawCount" dc:"原始数据量"`
	LocalCount  int    `json:"localCount" dc:"本地数据量"`
	Difference  int    `json:"difference" dc:"差异数"`
	LatestRawAt int    `json:"latestRawAt" dc:"最近原始时间"`
	Status      string `json:"status" dc:"状态"`
}

type AdminSyncReconcileRes struct {
	List []AdminSyncReconcileItem `json:"list" dc:"对账列表"`
}

// CaseListItem ?????
type CaseListItem struct {
	Id                 int    `json:"id" dc:"记录ID"`
	ResponsibilityUnit string `json:"responsibilityUnit" dc:"责任单位"`
	CaseNumber         string `json:"caseNumber" dc:"案件编号"`
	CaseSource         string `json:"caseSource" dc:"案件来源"`
	ReportTime         string `json:"reportTime" dc:"上报时间"`
	PendingStep        string `json:"pendingStep" dc:"待办环节"`
	CaseType           string `json:"caseType" dc:"案件类别"`
	Region             string `json:"region" dc:"所属区域"`
	CaseLocation       string `json:"caseLocation" dc:"案件位置"`
	Description        string `json:"description" dc:"问题描述"`
	CreateTime         string `json:"createTime" dc:"创建时间"`
	UpdateTime         string `json:"updateTime" dc:"更新时间"`
}

type AdminCaseListReq struct {
	g.Meta     `path:"/admin/cases" method:"get" tags:"V1/Admin" sm:"case list"`
	Page       int    `json:"page" d:"1" v:"min:1"`
	PageSize   int    `json:"pageSize" d:"20" v:"min:1|max:200"`
	CaseNumber string `json:"caseNumber"`
	CaseType   string `json:"caseType"`
	Region     string `json:"region"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

type AdminCaseListRes struct {
	List     []CaseListItem `json:"list" dc:"案件列表"`
	Total    int            `json:"total" dc:"总记录数"`
	Page     int            `json:"page" dc:"当前页码"`
	PageSize int            `json:"pageSize" dc:"每页条数"`
}

type AdminCaseDeleteReq struct {
	g.Meta `path:"/admin/cases/{id}" method:"delete" tags:"V1/Admin" sm:"delete case"`
	Id     int `json:"id" p:"id" v:"min:1#案件ID不能为空"`
}

type AdminCaseDeleteRes struct{}

type AdminCaseDeleteAllReq struct {
	g.Meta   `path:"/admin/cases/all" method:"delete" tags:"V1/Admin" sm:"delete all cases"`
	Confirm  bool `json:"confirm" v:"required#确认参数不能为空" dc:"必须为true才执行删除"`
}

type AdminCaseDeleteAllRes struct {
	Deleted int `json:"deleted" dc:"删除记录数"`
}

// CaseStatistics ??????
type CaseStatistics struct {
	Total     int            `json:"total" dc:"案件总量"`
	ByType    []CaseTypeStat `json:"byType" dc:"按案件类型"`
	ByRegion  []CaseTypeStat `json:"byRegion" dc:"按区域"`
	BySource  []CaseTypeStat `json:"bySource" dc:"按案件来源"`
	ByPending []CaseTypeStat `json:"byPending" dc:"按待办环节"`
}

type CaseTypeStat struct {
	Name  string `json:"name" dc:"名称"`
	Count int    `json:"count" dc:"数量"`
}

type AdminCaseStatisticsReq struct {
	g.Meta `path:"/admin/cases/statistics" method:"get" tags:"V1/Admin" sm:"case statistics"`
	Month  string `json:"month" p:"month" dc:"筛选月份，格式YYYY-MM" v:"length:7|regex:^\\d{4}-\\d{2}$#月份长度不合法|月份格式不合法，须为YYYY-MM"`
}

type AdminCaseStatisticsRes struct {
	Statistics CaseStatistics `json:"statistics" dc:"统计数据"`
}

type AdminLogItem struct {
	Id         int    `json:"id" dc:"记录ID"`
	LogType    string `json:"logType" dc:"日志类型"`
	Username   string `json:"username" dc:"用户名"`
	ActionType string `json:"actionType" dc:"操作类型"`
	Content    string `json:"content" dc:"操作内容"`
	Result     string `json:"result" dc:"操作结果"`
	CreateTime int    `json:"createTime" dc:"创建时间"`
}

type AdminLogsReq struct {
	g.Meta   `path:"/admin/logs" method:"get" tags:"V1/Admin" sm:"admin logs"`
	LogType  string `json:"logType" p:"logType"`
	Page     int    `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize int    `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminLogsRes struct {
	List     []AdminLogItem `json:"list" dc:"日志列表"`
	Total    int            `json:"total" dc:"总记录数"`
	Page     int            `json:"page" dc:"当前页码"`
	PageSize int            `json:"pageSize" dc:"每页条数"`
}

type AdminMetricItem struct {
	ID                int64    `json:"id" dc:"记录ID"`
	Topic             string   `json:"topic" dc:"主题"`
	MetricName        string   `json:"metricName" dc:"指标名"`
	DisplayName       string   `json:"displayName" dc:"显示名称"`
	Description       string   `json:"description" dc:"描述"`
	Unit              string   `json:"unit" dc:"单位"`
	Dimensions        []string `json:"dimensions" dc:"维度列表"`
	DefaultThreshold  *float64 `json:"defaultThreshold" dc:"默认阈值"`
	ThresholdDirection string   `json:"thresholdDirection" dc:"阈值方向"`
	RelatedFastPath   *int     `json:"relatedFastPath" dc:"关联快路径"`
	ChartTypeHint     string   `json:"chartTypeHint" dc:"图表类型提示"`
	IsActive          bool     `json:"isActive" dc:"是否启用"`
	CreateTime        int      `json:"createTime" dc:"创建时间"`
	UpdateTime        int      `json:"updateTime" dc:"更新时间"`
}

type AdminMetricsReq struct {
	g.Meta   `path:"/admin/metrics" method:"get" tags:"V1/Admin" sm:"list metrics"`
	Topic    string `json:"topic" p:"topic"`
	Page     int    `json:"page" p:"page" d:"1" v:"min:1"`
	PageSize int    `json:"pageSize" p:"pageSize" d:"20" v:"min:1|max:200"`
}

type AdminMetricsRes struct {
	List     []AdminMetricItem `json:"list" dc:"指标列表"`
	Total    int               `json:"total" dc:"总记录数"`
	Page     int               `json:"page" dc:"当前页码"`
	PageSize int               `json:"pageSize" dc:"每页条数"`
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
	Id int64 `json:"id" dc:"新建指标ID"`
}

type AdminMetricUpdateReq struct {
	g.Meta             `path:"/admin/metrics/{id}" method:"put" tags:"V1/Admin" sm:"update metric"`
	Id                 int64 `v:"min:1#指标ID不能为空"`
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
	Id     int64 `v:"min:1#指标ID不能为空"`
}

type AdminMetricDeleteRes struct{}

type AdminMetricToggleReq struct {
	g.Meta `path:"/admin/metrics/{id}/toggle" method:"put" tags:"V1/Admin" sm:"toggle metric active"`
	Id     int64 `v:"min:1#指标ID不能为空"`
}

type AdminMetricToggleRes struct {
	IsActive bool `json:"isActive" dc:"是否启用"`
}

type TopicKnowledgeBindingItem struct {
	Id            int    `json:"id" dc:"记录ID"`
	Topic         string `json:"topic" dc:"主题"`
	KnowledgeCode string `json:"knowledgeCode" dc:"知识库编码"`
	KnowledgeName string `json:"knowledgeName" dc:"知识库名称"`
	Enabled       bool   `json:"enabled" dc:"是否启用"`
	CreateTime     int    `json:"createTime" dc:"创建时间"`
	UpdateTime     int    `json:"updateTime" dc:"更新时间"`
}

type AdminTopicKnowledgeBindingsReq struct {
	g.Meta   `path:"/admin/topic-knowledge-bindings" method:"get" tags:"V1/Admin" sm:"list topic-knowledge bindings"`
	Topic    string `json:"topic" p:"topic" dc:"按主题筛选"`
	Page     int    `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize int    `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminTopicKnowledgeBindingsRes struct {
	List     []TopicKnowledgeBindingItem `json:"list" dc:"绑定列表"`
	Total    int                         `json:"total" dc:"总记录数"`
	Page     int                         `json:"page" dc:"当前页码"`
	PageSize int                         `json:"pageSize" dc:"每页条数"`
}

type AdminBindTopicKnowledgeReq struct {
	g.Meta         `path:"/admin/topic-knowledge-bindings" method:"post" tags:"V1/Admin" sm:"bind knowledge base to topic"`
	Topic          string `json:"topic" v:"required#主题不能为空"`
	KnowledgeCode  string `json:"knowledgeCode" v:"required#知识库编码不能为空"`
}

type AdminBindTopicKnowledgeRes struct {
	Item TopicKnowledgeBindingItem `json:"item" dc:"绑定信息"`
}

type AdminUnbindTopicKnowledgeReq struct {
	g.Meta `path:"/admin/topic-knowledge-bindings/{id}" method:"delete" tags:"V1/Admin" sm:"unbind knowledge base from topic"`
	Id     int `json:"id" p:"id" v:"min:1#绑定ID不能为空"`
}

type AdminUnbindTopicKnowledgeRes struct{}

type AdminToggleTopicKnowledgeReq struct {
	g.Meta  `path:"/admin/topic-knowledge-bindings/{id}/toggle" method:"put" tags:"V1/Admin" sm:"toggle topic-knowledge binding"`
	Id      int  `json:"id" p:"id" v:"min:1#绑定ID不能为空"`
	Enabled bool `json:"enabled"`
}

type AdminToggleTopicKnowledgeRes struct {
	Enabled bool `json:"enabled" dc:"是否启用"`
}

type AdminUpdateDocumentVersionReq struct {
	g.Meta        `path:"/admin/documents/{id}/version" method:"put" tags:"V1/Admin" sm:"update document version info"`
	Id            int64  `json:"id" p:"id" v:"min:1#文档ID不能为空"`
	EffectiveDate string `json:"effectiveDate" dc:"生效日期，格式 YYYY-MM-DD"`
	RepealDate    string `json:"repealDate" dc:"废止日期，格式 YYYY-MM-DD"`
	RepealedBy    string `json:"repealedBy" dc:"废止该文档的文档标题或ID"`
	TitleGroup    string `json:"titleGroup" dc:"同名多版本分组键"`
	Status        string `json:"status" dc:"文档状态：active/repealed/draft" v:"in:active,repealed,draft#状态值不合法"`
}

type AdminUpdateDocumentVersionRes struct {
	DocumentId int64 `json:"documentId" dc:"文档ID"`
}

type AdminDocumentVersionsReq struct {
	g.Meta     `path:"/admin/documents/versions" method:"get" tags:"V1/Admin" sm:"list document version groups"`
	TitleGroup string `json:"titleGroup" p:"titleGroup" dc:"按同名分组键筛选"`
	Page       int    `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize   int    `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminDocumentVersionsRes struct {
	Groups   []DocumentVersionGroup `json:"groups" dc:"版本分组列表"`
	Total    int                     `json:"total" dc:"总记录数"`
	Page     int                     `json:"page" dc:"当前页码"`
	PageSize int                     `json:"pageSize" dc:"每页条数"`
}

type DocumentVersionGroup struct {
	TitleGroup string                  `json:"titleGroup" dc:"同名分组键"`
	Versions   []DocumentVersionDetail `json:"versions" dc:"版本列表"`
}

type DocumentVersionDetail struct {
	DocumentId    int64  `json:"documentId" dc:"文档ID"`
	Title         string `json:"title" dc:"文档标题"`
	Status        string `json:"status" dc:"文档状态"`
	EffectiveDate string `json:"effectiveDate,omitempty"`
	RepealDate    string `json:"repealDate,omitempty"`
	RepealedBy    string `json:"repealedBy,omitempty"`
}

type AdminDocumentRelationsReq struct {
	g.Meta     `path:"/admin/document-relations" method:"get" tags:"V1/Admin" sm:"list document relations"`
	DocumentId int64  `json:"documentId" p:"documentId" dc:"按文档ID筛选"`
	RelType    string `json:"relType" p:"relType" dc:"按关联类型筛选：reference|supplement|repeal|related"`
	Page       int    `json:"page" d:"1" p:"page" v:"min:1"`
	PageSize   int    `json:"pageSize" d:"20" p:"pageSize" v:"min:1|max:200"`
}

type AdminDocumentRelationsRes struct {
	List     []DocumentRelationItem `json:"list" dc:"关联列表"`
	Total    int                    `json:"total" dc:"总记录数"`
	Page     int                    `json:"page" dc:"当前页码"`
	PageSize int                    `json:"pageSize" dc:"每页条数"`
}

type DocumentRelationItem struct {
	Id           int64  `json:"id" dc:"记录ID"`
	FromDocId    int64  `json:"fromDocId" dc:"来源文档ID"`
	FromDocTitle string `json:"fromDocTitle" dc:"来源文档标题"`
	ToDocId      int64  `json:"toDocId" dc:"目标文档ID"`
	ToDocTitle   string `json:"toDocTitle" dc:"目标文档标题"`
	RelType      string `json:"relType" dc:"关联类型"`
	Description  string `json:"description,omitempty"`
	Enabled      bool   `json:"enabled" dc:"是否启用"`
}

type AdminCreateDocumentRelationReq struct {
	g.Meta      `path:"/admin/document-relations" method:"post" tags:"V1/Admin" sm:"create document relation"`
	FromDocId   int64  `json:"fromDocId" v:"required#来源文档ID不能为空"`
	ToDocId     int64  `json:"toDocId" v:"required#目标文档ID不能为空"`
	RelType     string `json:"relType" v:"required|in:reference,supplement,repeal,related#关联类型不能为空|关联类型须为reference/supplement/repeal/related"`
	Description string `json:"description"`
}

type AdminCreateDocumentRelationRes struct {
	Id int64 `json:"id" dc:"新建关联ID"`
}

type AdminDeleteDocumentRelationReq struct {
	g.Meta `path:"/admin/document-relations/{id}" method:"delete" tags:"V1/Admin" sm:"delete document relation"`
	Id     int64 `json:"id" p:"id" v:"min:1#关联ID不能为空"`
}

type AdminDeleteDocumentRelationRes struct{}

type AdminDocumentRecommendationsReq struct {
	g.Meta     `path:"/admin/documents/{id}/recommendations" method:"get" tags:"V1/Admin" sm:"get related document recommendations"`
	Id         int64 `json:"id" p:"id" v:"min:1#文档ID不能为空"`
	TopN       int   `json:"topN" p:"topN" dc:"返回条数，默认5" v:"min:1|max:50"`
}

type AdminDocumentRecommendationsRes struct {
	List []DocumentRelationItem `json:"list" dc:"推荐关联列表"`
}

type AdminAutoDiscoverRelationsReq struct {
	g.Meta       `path:"/admin/document-relations/auto" method:"post" tags:"V1/Admin" sm:"auto discover document relations"`
	KnowledgeCode string `json:"knowledgeCode" dc:"仅在此知识库范围内发现，不传则全库扫描"`
	DryRun        *bool   `json:"dryRun" dc:"仅返回候选不实际写入，默认true"`
}

type AdminAutoDiscoverRelationsRes struct {
	Discovered []AutoDiscoverItem `json:"discovered" dc:"发现的关联"`
	Created    int                `json:"created" dc:"已创建数量"`
}

type AutoDiscoverItem struct {
	FromDocId    int64  `json:"fromDocId" dc:"来源文档ID"`
	FromDocTitle string `json:"fromDocTitle" dc:"来源文档标题"`
	ToDocId      int64  `json:"toDocId" dc:"目标文档ID"`
	ToDocTitle   string `json:"toDocTitle" dc:"目标文档标题"`
	RelType      string `json:"relType" dc:"关联类型"`
	Reason       string `json:"reason" dc:"发现原因"`
}

type AdminComplianceCheckReq struct {
	g.Meta       `path:"/admin/documents/compliance" method:"post" tags:"V1/Admin" sm:"check document compliance issues"`
	KnowledgeCode string `json:"knowledgeCode" dc:"仅检查此知识库，不传则全库"`
}

type AdminComplianceCheckRes struct {
	Issues []ComplianceIssue `json:"issues" dc:"合规问题列表"`
}

type ComplianceIssue struct {
	DocumentId    int64  `json:"documentId" dc:"文档ID"`
	DocumentTitle string `json:"documentTitle" dc:"文档标题"`
	IssueType     string `json:"issueType" dc:"问题类型"`
	Description   string `json:"description" dc:"问题描述"`
	Severity      string `json:"severity" dc:"严重程度"`
}
