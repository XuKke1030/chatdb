package precipitate

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
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
			scanAndUpsert(ctx, hitThreshold)
		}
	}
}

func scanAndUpsert(ctx context.Context, hitThreshold int) {
	db := g.DB("master")
	now := int(gtime.Timestamp())

	scanAskNumberStats(ctx, db, hitThreshold, now)
	scanQaStats(ctx, db, hitThreshold, now)
}

func scanAskNumberStats(ctx context.Context, db gdb.DB, hitThreshold, now int) {
	topics := []string{"traffic", "population", "grid"}

	for _, topic := range topics {
		records, err := db.Model("ask_number_question_stat").Ctx(ctx).
			Fields("question_normalized, SUM(hit_count) as total_hits, MAX(last_asked_at) as last_seen").
			Where("topic = ? AND hit_count >= ?", topic, hitThreshold).
			Group("question_normalized").
			Having("total_hits >= ?", hitThreshold).
			All()
		if err != nil || len(records) == 0 {
			continue
		}

		for _, r := range records {
			question := r["question_normalized"].String()
			if question == "" {
				continue
			}

			totalHits := r["total_hits"].Int()
			lastSeen := r["last_seen"].Int()

			existing, _ := db.Model("admin_question_candidate").Ctx(ctx).
				Where("topic = ? AND question = ? AND status != 'rejected'", topic, question).
				One()

			if existing != nil {
				if existing["status"].String() == "pending" {
					_, _ = db.Model("admin_question_candidate").Ctx(ctx).
						Where("id = ?", existing["id"].Int()).
						Data(g.Map{
							"count":        totalHits,
							"last_seen_at": lastSeen,
							"update_time":  now,
						}).Update()
				}
				continue
			}

			_, _ = db.Model("admin_question_candidate").Ctx(ctx).Data(g.Map{
				"topic":        topic,
				"question":     question,
				"status":       "pending",
				"count":        totalHits,
				"last_seen_at": lastSeen,
				"create_time":  now,
				"update_time":  now,
			}).Insert()
		}
	}
}

func scanQaStats(ctx context.Context, db gdb.DB, hitThreshold, now int) {
	records, err := db.Model("qa_question_stat").Ctx(ctx).
		Fields("question, SUM(hit_count) as total_hits, MAX(last_asked_at) as last_seen").
		Where("hit_count >= ?", hitThreshold).
		Group("question").
		Having("total_hits >= ?", hitThreshold).
		All()
	if err != nil || len(records) == 0 {
		return
	}

	for _, r := range records {
		raw := r["question"].String()
		question := NormalizeQuestion(raw)
		if question == "" {
			continue
		}

		totalHits := r["total_hits"].Int()
		lastSeen := r["last_seen"].Int()

		existing, _ := db.Model("admin_question_candidate").Ctx(ctx).
			Where("topic = ? AND question = ? AND status != 'rejected'", "qa", question).
			One()

		if existing != nil {
			if existing["status"].String() == "pending" {
				_, _ = db.Model("admin_question_candidate").Ctx(ctx).
					Where("id = ?", existing["id"].Int()).
					Data(g.Map{
						"count":        totalHits,
						"last_seen_at": lastSeen,
						"update_time":  now,
					}).Update()
			}
			continue
		}

		_, _ = db.Model("admin_question_candidate").Ctx(ctx).Data(g.Map{
			"topic":        "qa",
			"question":     question,
			"status":       "pending",
			"count":        totalHits,
			"last_seen_at": lastSeen,
			"create_time":  now,
			"update_time":  now,
		}).Insert()
	}
}
