package population

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/dlock"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func (s *sPopulation) RefreshAggregates(ctx context.Context, dateFrom string, dateTo string) error {
	lockKey := "lock:aggregate:population"
	acquired, lockErr := dlock.TryAcquire(ctx, lockKey, 15)
	if lockErr != nil {
		consts.Logger.Warningf(ctx, "tryAcquire %s failed: %v", lockKey, lockErr)
		return fmt.Errorf("acquire aggregate lock: %w", lockErr)
	}
	if !acquired {
		consts.Logger.Infof(ctx, "skip RefreshAggregates: lock %s held by another instance", lockKey)
		return nil
	}
	defer dlock.Release(ctx, lockKey)

	from, to := normalizePopWindow(dateFrom, dateTo)
	if err := s.refreshPopulationHourly(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshPopulationDaily(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshPopulationFloatingDaily(ctx, from, to); err != nil {
		return err
	}
	return s.refreshPopulationTagDaily(ctx, from, to)
}

const defaultRefreshDays = 7

func normalizePopWindow(dateFrom string, dateTo string) (string, string) {
	from := dateFrom
	to := dateTo
	if from == "" {
		from = gtime.Now().AddDate(0, 0, -defaultRefreshDays).Format("Y-m-d H:i:s")
	}
	if to == "" {
		to = gtime.Now().Format("Y-m-d") + " 23:59:59"
	}
	return from, to
}

func (s *sPopulation) refreshPopulationHourly(ctx context.Context, from string, to string) error {
	return g.DB("master").Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("DELETE FROM population_metric_hourly WHERE metric_hour >= ? AND metric_hour <= ?", from, to); err != nil {
			return err
		}
		_, err := tx.Exec(`
INSERT INTO population_metric_hourly (
	metric_hour, region, grid_name,
	in_count, out_count, net_in_count, floating_population_count,
	source_provider, update_time
)
SELECT
	DATE_FORMAT(metric_time, '%Y-%m-%d %H:00:00') AS metric_hour,
	COALESCE(region, '') AS region,
	COALESCE(grid_name, '') AS grid_name,
	SUM(in_count) AS in_count,
	SUM(out_count) AS out_count,
	SUM(net_in_count) AS net_in_count,
	SUM(floating_population_count) AS floating_population_count,
	'aidgp' AS source_provider,
	UNIX_TIMESTAMP() AS update_time
FROM population_flow_record
WHERE metric_time >= ? AND metric_time <= ?
GROUP BY metric_hour, region, grid_name`, from, to)
		return err
	})
}

func (s *sPopulation) refreshPopulationDaily(ctx context.Context, from string, to string) error {
	return g.DB("master").Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("DELETE FROM population_metric_daily WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)", from, to); err != nil {
			return err
		}
		_, err := tx.Exec(`
INSERT INTO population_metric_daily (
	metric_date, region, grid_name,
	in_count, out_count, net_in_count, floating_population_count,
	source_provider, update_time
)
SELECT
	DATE(metric_time) AS metric_date,
	COALESCE(region, '') AS region,
	COALESCE(grid_name, '') AS grid_name,
	SUM(in_count) AS in_count,
	SUM(out_count) AS out_count,
	SUM(net_in_count) AS net_in_count,
	SUM(floating_population_count) AS floating_population_count,
	'aidgp' AS source_provider,
	UNIX_TIMESTAMP() AS update_time
FROM population_flow_record
WHERE metric_time >= ? AND metric_time <= ?
GROUP BY metric_date, region, grid_name`, from, to)
		return err
	})
}

func (s *sPopulation) refreshPopulationFloatingDaily(ctx context.Context, from string, to string) error {
	return g.DB("master").Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("DELETE FROM population_floating_daily WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)", from, to); err != nil {
			return err
		}
		_, err := tx.Exec(`
INSERT INTO population_floating_daily (
	metric_date, region, grid_name,
	floating_population_count,
	source_provider, update_time
)
SELECT
	DATE(metric_time) AS metric_date,
	COALESCE(region, '') AS region,
	COALESCE(grid_name, '') AS grid_name,
	SUM(floating_population_count) AS floating_population_count,
	'aidgp' AS source_provider,
	UNIX_TIMESTAMP() AS update_time
FROM population_flow_record
WHERE metric_time >= ? AND metric_time <= ?
GROUP BY metric_date, region, grid_name`, from, to)
		return err
	})
}

func (s *sPopulation) refreshPopulationTagDaily(ctx context.Context, from, to string) error {
	return g.DB("master").Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("DELETE FROM population_tag_daily WHERE day >= DATE(?) AND day <= DATE(?)", from, to); err != nil {
			return err
		}
		_, err := tx.Exec(`
INSERT INTO population_tag_daily (day, area, tag, label, type, label_cnt, update_time)
SELECT day, COALESCE(area,''), tag, label, type, SUM(label_cnt), UNIX_TIMESTAMP()
FROM mobile_day_flow_tag
WHERE day >= DATE(?) AND day <= DATE(?)
GROUP BY day, area, tag, label, type`, from, to)
		return err
	})
}
