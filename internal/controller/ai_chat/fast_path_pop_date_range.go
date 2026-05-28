package ai_chat

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func popEffectiveRange(days int) (from, to string) {
	from = gtime.Now().AddDate(0, 0, -(days - 1)).Format("Y-m-d")
	to = gtime.Now().Format("Y-m-d")
	return from, to
}

func popTagEffectiveRange(days int) (from, to string) {
	from = gtime.Now().AddDate(0, 0, -(days - 1)).Format("Y-m-d")
	to = gtime.Now().Format("Y-m-d")
	return from, to
}

// popDateRangeFromParams extracts the effective date range from ExtractedParams.
// If AllDates is true, queries the database for the actual min/max date range.
// Otherwise uses DateFrom/DateTo if set, or falls back to popEffectiveRange(defaultDays).
func popDateRangeFromParams(ctx context.Context, p ExtractedParams, defaultDays int) (from, to string) {
	if p.AllDates {
		db := g.DB("master")
		// Try population_metric_daily first, then population_tag_daily
		for _, table := range []string{"population_metric_daily", "population_tag_daily"} {
			dateCol := "metric_date"
			if table == "population_tag_daily" {
				dateCol = "day"
			}
			record, err := db.Ctx(ctx).Raw(
				fmt.Sprintf("SELECT MIN(%s) AS min_d, MAX(%s) AS max_d FROM %s", dateCol, dateCol, table),
			).One()
			if err == nil && record != nil {
				minD := record["min_d"].String()
				maxD := record["max_d"].String()
				if minD != "" && maxD != "" {
					return minD, maxD + " 23:59:59"
				}
			}
		}
		return popEffectiveRange(defaultDays)
	}
	if p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
		if to == "" {
			to = gtime.Now().Format("Y-m-d") + " 23:59:59"
		} else if len(to) <= 10 {
			to = to[:10] + " 23:59:59"
		}
		return from, to
	}
	from, to = popEffectiveRange(defaultDays)
	return from, to + " 23:59:59"
}
