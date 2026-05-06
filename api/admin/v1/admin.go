package v1

import "github.com/gogf/gf/v2/frame/g"

type TopicPermission struct {
	Topic   string `json:"topic"`
	Enabled bool   `json:"enabled"`
}

type KnowledgePermission struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
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
	UserId        int                   `json:"userId"`
	Username      string                `json:"username"`
	DisplayName   string                `json:"displayName"`
	Department    string                `json:"department"`
	Enabled       bool                  `json:"enabled"`
	RuleLevel     int                   `json:"ruleLevel"`
	Permissions   []TopicPermission     `json:"permissions"`
	QaPermissions []KnowledgePermission `json:"qaPermissions"`
	LastLoginAt   int                   `json:"lastLoginAt"`
	UpdateTime    int                   `json:"updateTime"`
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
