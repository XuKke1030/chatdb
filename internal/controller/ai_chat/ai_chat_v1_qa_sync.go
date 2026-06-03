package ai_chat

import (
	"context"

	v1 "ai-chat-sql/api/ai_chat/v1"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func (c *ControllerV1) QaSyncStatus(ctx context.Context, req *v1.QaSyncStatusReq) (res *v1.QaSyncStatusRes, err error) {
	records, err := g.DB("master").Model("qa_sync_task").Ctx(ctx).
		OrderDesc("task_id").
		Limit(req.Limit).
		All()
	if err != nil {
		return
	}

	list := make([]v1.QaSyncTaskItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.QaSyncTaskItem{
			TaskId:       r["task_id"].Int(),
			Provider:     r["provider"].String(),
			SyncType:     r["sync_type"].String(),
			Status:       r["status"].String(),
			Message:      r["message"].String(),
			SuccessCount: r["success_count"].Int(),
			FailureCount: r["failure_count"].Int(),
			SkippedCount: r["skipped_count"].Int(),
			StartedAt:    r["started_at"].Int(),
			FinishedAt:   r["finished_at"].Int(),
			CreateTime:   r["create_time"].Int(),
			UpdateTime:   r["update_time"].Int(),
		})
	}
	return &v1.QaSyncStatusRes{List: list}, nil
}

func (c *ControllerV1) QaSyncTaskLogs(ctx context.Context, req *v1.QaSyncTaskLogsReq) (res *v1.QaSyncTaskLogsRes, err error) {
	records, err := g.DB("master").Model("qa_sync_log").Ctx(ctx).
		Where("task_id = ?", req.Id).
		OrderAsc("log_id").
		All()
	if err != nil {
		return
	}

	list := make([]v1.QaSyncLogItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.QaSyncLogItem{
			LogId:      r["log_id"].Int(),
			TaskId:     r["task_id"].Int(),
			Provider:   r["provider"].String(),
			SyncType:   r["sync_type"].String(),
			ExternalId: r["external_id"].String(),
			LocalId:    r["local_id"].String(),
			Action:     r["action"].String(),
			Status:     r["status"].String(),
			Message:    r["message"].String(),
			CreateTime: r["create_time"].Int(),
		})
	}
	return &v1.QaSyncTaskLogsRes{List: list}, nil
}

func (c *ControllerV1) QaSyncTrigger(ctx context.Context, req *v1.QaSyncTriggerReq) (res *v1.QaSyncTriggerRes, err error) {
	now := int(gtime.Timestamp())
	result, err := g.DB("master").Model("qa_sync_task").Ctx(ctx).Data(g.Map{
		"provider":      req.Provider,
		"sync_type":     "full",
		"status":        "running",
		"message":        "手动触发同步",
		"success_count": 0,
		"failure_count": 0,
		"skipped_count": 0,
		"started_at":    now,
		"finished_at":   0,
		"create_time":   now,
		"update_time":   now,
	}).Insert()
	if err != nil {
		return
	}
	taskId, _ := result.LastInsertId()
	return &v1.QaSyncTriggerRes{TaskId: int(taskId)}, nil
}
