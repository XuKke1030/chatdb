package precipitate

import (
	"context"
	"time"

	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/dlock"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func StartCandidateScanner(ctx context.Context, interval time.Duration, hitThreshold int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lockKey := "lock:precipitate:scan"
			acquired, err := dlock.TryAcquire(ctx, lockKey, 15)
			if err != nil {
				consts.Logger.Warningf(ctx, "tryAcquire %s failed: %v", lockKey, err)
				continue
			}
			if !acquired {
				continue
			}
			scanAndUpsert(ctx, hitThreshold)
			dlock.Release(ctx, lockKey)
		}
	}
}

func scanAndUpsert(ctx context.Context, hitThreshold int) {
	now := int(gtime.Timestamp())
	windowStart := now - 30*24*3600

	records, err := g.DB("master").Model("ask_number_question_stat").Ctx(ctx).
		Fields("user_id, topic, question_normalized, SUM(hit_count) AS total_hits, MAX(last_asked_at) AS last_seen").
		Where("last_asked_at >= ?", windowStart).
		Group("user_id, topic, question_normalized").
		Having("total_hits >= ?", hitThreshold).
		All()
	if err != nil {
		consts.Logger.Warningf(ctx, "scanAndUpsert query failed: %v", err)
		return
	}

	for _, record := range records {
		question := record["question_normalized"].String()
		totalHits := record["total_hits"].Int()
		lastSeen := record["last_seen"].Int()
		if question == "" {
			continue
		}

		existing, err := g.DB("master").Model("ask_number_question_candidate").Ctx(ctx).
			Where("question = ?", question).One()
		if err != nil {
			continue
		}

		if existing != nil {
			if _, err := g.DB("master").Model("ask_number_question_candidate").Ctx(ctx).
				Where("id = ?", existing["id"].Int64()).
				Data(g.Map{
					"count":        totalHits,
					"last_seen_at": lastSeen,
					"update_time":  now,
				}).Update(); err != nil {
				consts.Logger.Warningf(ctx, "update candidate failed: %v", err)
			}
			continue
		}

		if _, err := g.DB("master").Model("ask_number_question_candidate").Ctx(ctx).Data(g.Map{
			"topic":        "qa",
			"question":     question,
			"status":       "pending",
			"count":        totalHits,
			"last_seen_at": lastSeen,
			"create_time":  now,
			"update_time":  now,
		}).Insert(); err != nil {
			consts.Logger.Warningf(ctx, "insert candidate failed: %v", err)
		}
	}
}
