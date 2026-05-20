package population

import (
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

type sPopulation struct{}

func init() {
	service.RegisterPopulation(NewPopulation())
}

func NewPopulation() *sPopulation {
	return &sPopulation{}
}

func (s *sPopulation) InitTables(ctx context.Context) error {
	db := g.DB("master")
	statements := []string{
		`CREATE TABLE IF NOT EXISTS population_flow_record (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		external_id VARCHAR(128) NOT NULL,
		sync_version VARCHAR(64),
		metric_time DATETIME NOT NULL,
		region VARCHAR(128),
		grid_name VARCHAR(128),
		in_count INT NOT NULL DEFAULT 0,
		out_count INT NOT NULL DEFAULT 0,
		net_in_count INT NOT NULL DEFAULT 0,
		floating_population_count INT NOT NULL DEFAULT 0,
		source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
		raw_payload LONGTEXT,
		create_time INT NOT NULL,
		update_time INT NOT NULL,
		UNIQUE KEY uk_population_external (external_id),
		INDEX idx_population_time_region (metric_time, region)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_metric_hourly (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		metric_hour DATETIME NOT NULL,
		region VARCHAR(128),
		grid_name VARCHAR(128),
		in_count INT NOT NULL DEFAULT 0,
		out_count INT NOT NULL DEFAULT 0,
		net_in_count INT NOT NULL DEFAULT 0,
		floating_population_count INT NOT NULL DEFAULT 0,
		source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
		update_time INT NOT NULL,
		UNIQUE KEY uk_pop_hour (metric_hour, region, grid_name),
		INDEX idx_pop_hour_region (metric_hour, region)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_metric_daily (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		metric_date DATE NOT NULL,
		region VARCHAR(128),
		grid_name VARCHAR(128),
		in_count INT NOT NULL DEFAULT 0,
		out_count INT NOT NULL DEFAULT 0,
		net_in_count INT NOT NULL DEFAULT 0,
		floating_population_count INT NOT NULL DEFAULT 0,
		source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
		update_time INT NOT NULL,
		UNIQUE KEY uk_pop_daily (metric_date, region, grid_name),
		INDEX idx_pop_region_date (region, metric_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_floating_daily (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		metric_date DATE NOT NULL,
		region VARCHAR(128),
		grid_name VARCHAR(128),
		floating_population_count INT NOT NULL DEFAULT 0,
		source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
		update_time INT NOT NULL,
		UNIQUE KEY uk_pop_float_daily (metric_date, region, grid_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *sPopulation) Aggregate(ctx context.Context, query model.PopulationAggregateQuery) (*model.PopulationAggregateResult, error) {
	db := g.DB("master")
	groupBy := normalizePopGroupBy(query.GroupBy)

	var table string
	switch groupBy {
	case "hour":
		table = "population_metric_hourly"
	default:
		table = "population_metric_daily"
	}

	conditions := []string{}
	args := []any{}
	from := query.DateFrom
	to := query.DateTo
	if from == "" {
		from = time.Now().AddDate(0, 0, -7).Format("2006-01-02 00:00:00")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02 23:59:59")
	}

	if groupBy == "hour" {
		conditions = append(conditions, "metric_hour >= ? AND metric_hour <= ?")
		args = append(args, from, to)
	} else {
		conditions = append(conditions, "metric_date >= DATE(?) AND metric_date <= DATE(?)")
		args = append(args, from, to)
	}

	if query.Region != "" {
		conditions = append(conditions, "region = ?")
		args = append(args, query.Region)
	}
	if query.GridName != "" {
		conditions = append(conditions, "grid_name = ?")
		args = append(args, query.GridName)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			where += " AND " + c
		}
	}

	groupExpr := popGroupExpr(groupBy, table)
	records, err := db.Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			%s AS name,
			SUM(in_count) AS in_count,
			SUM(out_count) AS out_count,
			SUM(net_in_count) AS net_in_count,
			SUM(floating_population_count) AS floating_population_count
		FROM %s%s
		GROUP BY name
		ORDER BY name`, groupExpr, table, where), args...).All()
	if err != nil {
		return nil, err
	}

	summary := model.PopulationAggregateSummary{}
	series := make([]model.PopulationAggregateSeriesItem, 0, len(records))
	for _, r := range records {
		item := model.PopulationAggregateSeriesItem{
			Name:               r["name"].String(),
			InCount:            r["in_count"].Int(),
			OutCount:           r["out_count"].Int(),
			NetInCount:         r["net_in_count"].Int(),
			FloatingPopulation: r["floating_population_count"].Int(),
		}
		summary.TotalInCount += item.InCount
		summary.TotalOutCount += item.OutCount
		summary.TotalNetInCount += item.NetInCount
		summary.TotalFloatingPopulation += item.FloatingPopulation
		series = append(series, item)
	}

	return &model.PopulationAggregateResult{
		Summary: summary,
		Series:  series,
		GroupBy: groupBy,
	}, nil
}

func (s *sPopulation) YoYCompare(ctx context.Context, dateFrom, dateTo, groupBy string) ([]model.PopulationYoYItem, error) {
	db := g.DB("master")
	from := dateFrom
	to := dateTo
	if from == "" {
		from = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	currentRecords, err := db.Ctx(ctx).Raw(`
		SELECT metric_date AS name, SUM(in_count + out_count) AS total
		FROM population_metric_daily
		WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)
		GROUP BY metric_date ORDER BY metric_date`, from, to).All()
	if err != nil || len(currentRecords) == 0 {
		return nil, nil
	}

	parsedFrom, _ := time.Parse("2006-01-02", from[:10])
	parsedTo, _ := time.Parse("2006-01-02", to[:10])
	priorFrom := parsedFrom.AddDate(-1, 0, 0).Format("2006-01-02")
	priorTo := parsedTo.AddDate(-1, 0, 0).Format("2006-01-02")

	priorRecords, _ := db.Ctx(ctx).Raw(`
		SELECT metric_date AS name, SUM(in_count + out_count) AS total
		FROM population_metric_daily
		WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)
		GROUP BY metric_date ORDER BY metric_date`, priorFrom, priorTo).All()
	priorMap := make(map[string]int)
	for _, r := range priorRecords {
		priorMap[r["name"].String()] = r["total"].Int()
	}

	items := make([]model.PopulationYoYItem, 0, len(currentRecords))
	for _, r := range currentRecords {
		name := r["name"].String()
		curVal := r["total"].Int()
		priorVal := priorMap[name]
		var pct float64
		if priorVal > 0 {
			pct = float64(curVal-priorVal) / float64(priorVal) * 100
		}
		items = append(items, model.PopulationYoYItem{
			Name:       name,
			CurrentVal: curVal,
			PriorVal:   priorVal,
			ChangePct:  pct,
		})
	}
	return items, nil
}

func normalizePopGroupBy(groupBy string) string {
	switch groupBy {
	case "hour":
		return "hour"
	case "region":
		return "region"
	default:
		return "day"
	}
}

func popGroupExpr(groupBy string, table string) string {
	switch groupBy {
	case "hour":
		return "DATE_FORMAT(metric_hour, '%Y-%m-%d %H:00')"
	case "region":
		return "COALESCE(region, '')"
	default:
		return fmt.Sprintf("DATE(%s)", popDateCol(table))
	}
}

func popDateCol(table string) string {
	switch table {
	case "population_metric_hourly":
		return "metric_hour"
	default:
		return "metric_date"
	}
}
