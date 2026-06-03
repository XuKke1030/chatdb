package v1

import "github.com/gogf/gf/v2/frame/g"

type QaSyncStatusReq struct {
	g.Meta `path:"/qa/sync/status" method:"get" tags:"V1/问答同步" sm:"同步状态" dc:"获取最近同步任务列表"`
	Limit  int `p:"limit" in:"query" d:"8" dc:"返回任务数量上限"`
}

type QaSyncStatusRes struct {
	List []QaSyncTaskItem `json:"list"`
}

type QaSyncTaskItem struct {
	TaskId       int    `json:"taskId"`
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

type QaSyncTaskLogsReq struct {
	g.Meta `path:"/qa/sync/tasks/{id}/logs" method:"get" tags:"V1/问答同步" sm:"同步日志" dc:"获取指定同步任务的日志"`
	Id     int `p:"id" in:"path" dc:"任务ID"`
}

type QaSyncTaskLogsRes struct {
	List []QaSyncLogItem `json:"list"`
}

type QaSyncLogItem struct {
	LogId      int    `json:"logId"`
	TaskId     int    `json:"taskId"`
	Provider   string `json:"provider"`
	SyncType   string `json:"syncType"`
	ExternalId string `json:"externalId"`
	LocalId    string `json:"localId"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreateTime int    `json:"createTime"`
}

type QaSyncTriggerReq struct {
	g.Meta   `path:"/qa/sync/{provider}" method:"post" tags:"V1/问答同步" sm:"触发同步" dc:"手动触发指定知识库同步"`
	Provider string `p:"provider" in:"path" dc:"知识库提供者标识"`
}

type QaSyncTriggerRes struct {
	TaskId int `json:"taskId"`
}
