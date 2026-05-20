package population

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

func (s *sPopulation) RefreshAggregates(ctx context.Context, dateFrom string, dateTo string) error {
	from, to := normalizePopWindow(dateFrom, dateTo)
	if err := s.refreshPopulationHourly(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshPopulationDaily(ctx, from, to); err != nil {
		return err
	}
	return s.refreshPopulationFloatingDaily(ctx, from, to)
}

func normalizePopWindow(dateFrom string, dateTo string) (string, string) {
	from := dateFrom
	to := dateTo
	if from == "" {
		from = time.Now().AddDate(0, 0, -7).Format("2006-01-02 00:00:00")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02 23:59:59")
	}
	return from, to
}

func (s *sPopulation) refreshPopulationHourly(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM population_metric_hourly WHERE metric_hour >= ? AND metric_hour <= ?", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
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
}

func (s *sPopulation) refreshPopulationDaily(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM population_metric_daily WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
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
}

func (s *sPopulation) refreshPopulationFloatingDaily(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM population_floating_daily WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
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
}
