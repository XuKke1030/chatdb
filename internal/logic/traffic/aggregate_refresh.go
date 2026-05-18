package traffic

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

func (s *sTraffic) RefreshAggregates(ctx context.Context, dateFrom string, dateTo string) error {
	from, to := normalizeAggregateWindow(dateFrom, dateTo)
	if err := s.refreshTrafficHourly(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshTrafficDaily(ctx, from, to); err != nil {
		return err
	}
	return s.refreshTrafficStayDaily(ctx, from, to)
}

func normalizeAggregateWindow(dateFrom string, dateTo string) (string, string) {
	from := normalizeQueryTime(dateFrom, false)
	to := normalizeQueryTime(dateTo, true)
	if from == "" {
		from = time.Now().AddDate(0, 0, -7).Format("2006-01-02 00:00:00")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02 23:59:59")
	}
	return from, to
}

func (s *sTraffic) refreshTrafficHourly(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM traffic_metric_hourly WHERE metric_hour >= ? AND metric_hour <= ?", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
INSERT INTO traffic_metric_hourly (
metric_hour, region, device_id, device_name, total, in_count, out_count,
hk_macau_count, mainland_count, foreign_count, source_provider, update_time
)
SELECT
DATE_FORMAT(r.snapshot_time, '%Y-%m-%d %H:00:00') AS metric_hour,
COALESCE(NULLIF(d.region, ''), NULLIF(r.plate_region_type, ''), '') AS region,
r.device_id,
COALESCE(NULLIF(r.device_name, ''), r.device_id) AS device_name,
COUNT(*) AS total,
SUM(CASE WHEN r.in_dir = 0 THEN 1 ELSE 0 END) AS in_count,
SUM(CASE WHEN r.in_dir = 1 THEN 1 ELSE 0 END) AS out_count,
SUM(CASE WHEN r.is_hk_macau = 1 THEN 1 ELSE 0 END) AS hk_macau_count,
SUM(CASE WHEN r.is_hk_macau = 0 THEN 1 ELSE 0 END) AS mainland_count,
SUM(CASE WHEN r.plate_region_type <> '' AND r.plate_region_type NOT LIKE '%本地%' AND r.is_hk_macau = 0 THEN 1 ELSE 0 END) AS foreign_count,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM traffic_gate_record r
LEFT JOIN traffic_gate_device d ON d.device_id = r.device_id
WHERE r.snapshot_time >= ? AND r.snapshot_time <= ?
GROUP BY metric_hour, region, r.device_id, device_name`, from, to)
	return err
}

func (s *sTraffic) refreshTrafficDaily(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM traffic_metric_daily WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
INSERT INTO traffic_metric_daily (
metric_date, region, device_id, device_name, total, in_count, out_count,
hk_macau_count, mainland_count, foreign_count, source_provider, update_time
)
SELECT
DATE(r.snapshot_time) AS metric_date,
COALESCE(NULLIF(d.region, ''), NULLIF(r.plate_region_type, ''), '') AS region,
r.device_id,
COALESCE(NULLIF(r.device_name, ''), r.device_id) AS device_name,
COUNT(*) AS total,
SUM(CASE WHEN r.in_dir = 0 THEN 1 ELSE 0 END) AS in_count,
SUM(CASE WHEN r.in_dir = 1 THEN 1 ELSE 0 END) AS out_count,
SUM(CASE WHEN r.is_hk_macau = 1 THEN 1 ELSE 0 END) AS hk_macau_count,
SUM(CASE WHEN r.is_hk_macau = 0 THEN 1 ELSE 0 END) AS mainland_count,
SUM(CASE WHEN r.plate_region_type <> '' AND r.plate_region_type NOT LIKE '%本地%' AND r.is_hk_macau = 0 THEN 1 ELSE 0 END) AS foreign_count,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM traffic_gate_record r
LEFT JOIN traffic_gate_device d ON d.device_id = r.device_id
WHERE r.snapshot_time >= ? AND r.snapshot_time <= ?
GROUP BY metric_date, region, r.device_id, device_name`, from, to)
	return err
}

func (s *sTraffic) refreshTrafficStayDaily(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM traffic_vehicle_stay_daily WHERE metric_date >= DATE(?) AND metric_date <= DATE(?)", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
INSERT INTO traffic_vehicle_stay_daily (
metric_date, plate_normalized, region, device_count, stay_minutes,
first_seen_at, last_seen_at, source_provider, update_time
)
SELECT
DATE(r.snapshot_time) AS metric_date,
r.plate_normalized,
COALESCE(NULLIF(r.plate_region_type, ''), '') AS region,
COUNT(DISTINCT r.device_id) AS device_count,
TIMESTAMPDIFF(MINUTE, MIN(r.snapshot_time), MAX(r.snapshot_time)) AS stay_minutes,
MIN(r.snapshot_time) AS first_seen_at,
MAX(r.snapshot_time) AS last_seen_at,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM traffic_gate_record r
WHERE r.snapshot_time >= ? AND r.snapshot_time <= ?
GROUP BY metric_date, r.plate_normalized, region
HAVING device_count >= 2`, from, to)
	return err
}
