package traffic

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func (s *sTraffic) RefreshAggregates(ctx context.Context, dateFrom string, dateTo string) error {
	from, to := normalizeAggregateWindow(dateFrom, dateTo)
	if err := s.refreshTrafficHourly(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshTrafficDaily(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshTrafficStayDaily(ctx, from, to); err != nil {
		return err
	}
	if err := s.refreshStayDistributionDaily(ctx, from, to); err != nil {
		return err
	}
	return s.refreshOriginDaily(ctx, from, to)
}

const defaultRefreshDays = 7

func normalizeAggregateWindow(dateFrom string, dateTo string) (string, string) {
	from := normalizeQueryTime(dateFrom, false)
	to := normalizeQueryTime(dateTo, true)
	if from == "" {
		from = gtime.Now().AddDate(0, 0, -defaultRefreshDays).Format("Y-m-d H:i:s")
	}
	if to == "" {
		to = gtime.Now().Format("Y-m-d") + " 23:59:59"
	}
	return from, to
}

func normalizeQueryDateOnly(value string) string {
	value = normalizeQueryTime(value, false)
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}

func (s *sTraffic) refreshTrafficHourly(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM traffic_metric_hourly WHERE metric_hour >= ? AND metric_hour <= ?", from, to); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
INSERT INTO traffic_metric_hourly (
metric_hour, region, device_id, device_name, total, in_count, out_count,
hk_macau_count, mainland_count, foreign_count,
province_inside_count, province_outside_count,
source_provider, update_time
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
SUM(CASE WHEN r.is_province_inside = 1 THEN 1 ELSE 0 END) AS province_inside_count,
SUM(CASE WHEN r.is_hk_macau = 0 AND r.is_province_inside = 0 THEN 1 ELSE 0 END) AS province_outside_count,
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
hk_macau_count, mainland_count, foreign_count,
province_inside_count, province_outside_count,
is_holiday, holiday_name,
source_provider, update_time
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
SUM(CASE WHEN r.is_province_inside = 1 THEN 1 ELSE 0 END) AS province_inside_count,
SUM(CASE WHEN r.is_hk_macau = 0 AND r.is_province_inside = 0 THEN 1 ELSE 0 END) AS province_outside_count,
COALESCE(h.is_holiday, 0) AS is_holiday,
COALESCE(h.holiday_name, '') AS holiday_name,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM traffic_gate_record r
LEFT JOIN traffic_gate_device d ON d.device_id = r.device_id
LEFT JOIN traffic_holiday h ON h.holiday_date = DATE(r.snapshot_time)
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
is_hk_macau, first_seen_at, last_seen_at, source_provider, update_time
)
SELECT
DATE(r.snapshot_time) AS metric_date,
r.plate_normalized,
COALESCE(NULLIF(r.plate_region_type, ''), '') AS region,
COUNT(DISTINCT r.device_id) AS device_count,
TIMESTAMPDIFF(MINUTE, MIN(r.snapshot_time), MAX(r.snapshot_time)) AS stay_minutes,
MAX(r.is_hk_macau) AS is_hk_macau,
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

func (s *sTraffic) refreshStayDistributionDaily(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	dateFrom := normalizeQueryDateOnly(from)
	dateTo := normalizeQueryDateOnly(to)
	if _, err := db.Exec(ctx, "DELETE FROM traffic_stay_distribution_daily WHERE metric_date >= ? AND metric_date <= ?", dateFrom, dateTo); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
INSERT INTO traffic_stay_distribution_daily (
metric_date, region, bucket, is_hk_macau, vehicle_count, avg_stay_minutes,
source_provider, update_time
)
SELECT
s.metric_date,
s.region,
CASE
	WHEN s.stay_minutes < 30 THEN '0-30min'
	WHEN s.stay_minutes < 60 THEN '30-60min'
	WHEN s.stay_minutes < 120 THEN '1-2h'
	WHEN s.stay_minutes < 240 THEN '2-4h'
	ELSE '4h+'
END AS bucket,
s.is_hk_macau,
COUNT(*) AS vehicle_count,
AVG(s.stay_minutes) AS avg_stay_minutes,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM traffic_vehicle_stay_daily s
WHERE s.metric_date >= ? AND s.metric_date <= ?
GROUP BY s.metric_date, s.region, bucket, s.is_hk_macau`, dateFrom, dateTo)
	return err
}

func (s *sTraffic) refreshOriginDaily(ctx context.Context, from string, to string) error {
	db := g.DB("master")
	dateFrom := normalizeQueryDateOnly(from)
	dateTo := normalizeQueryDateOnly(to)
	if _, err := db.Exec(ctx, "DELETE FROM traffic_origin_daily WHERE metric_date >= ? AND metric_date <= ?", dateFrom, dateTo); err != nil {
		return err
	}

	// Group by province derived from first character of plate_normalized + is_hk_macau flag
	// For Guangdong plates, further group by city derived from second character
	_, err := db.Exec(ctx, `
INSERT INTO traffic_origin_daily (
metric_date, province, city, vehicle_count, is_hk_macau,
source_provider, update_time
)
SELECT
DATE(r.snapshot_time) AS metric_date,
CASE
	WHEN r.is_hk_macau = 1 AND LEFT(r.plate_normalized, 2) = '粤Z' THEN '广东'
	WHEN r.is_hk_macau = 1 THEN '港澳'
	WHEN LEFT(r.plate_normalized, 1) IN ('京','津','沪','渝','冀','豫','云','辽','黑','湘','皖','鲁','新','苏','浙','赣','鄂','桂','甘','晋','蒙','陕','吉','闽','贵','粤','青','藏','川','宁','琼') THEN
		CASE LEFT(r.plate_normalized, 1)
			WHEN '京' THEN '北京' WHEN '津' THEN '天津' WHEN '沪' THEN '上海' WHEN '渝' THEN '重庆'
			WHEN '冀' THEN '河北' WHEN '豫' THEN '河南' WHEN '云' THEN '云南' WHEN '辽' THEN '辽宁'
			WHEN '黑' THEN '黑龙江' WHEN '湘' THEN '湖南' WHEN '皖' THEN '安徽' WHEN '鲁' THEN '山东'
			WHEN '新' THEN '新疆' WHEN '苏' THEN '江苏' WHEN '浙' THEN '浙江' WHEN '赣' THEN '江西'
			WHEN '鄂' THEN '湖北' WHEN '桂' THEN '广西' WHEN '甘' THEN '甘肃' WHEN '晋' THEN '山西'
			WHEN '蒙' THEN '内蒙古' WHEN '陕' THEN '陕西' WHEN '吉' THEN '吉林' WHEN '闽' THEN '福建'
			WHEN '贵' THEN '贵州' WHEN '粤' THEN '广东' WHEN '青' THEN '青海' WHEN '藏' THEN '西藏'
			WHEN '川' THEN '四川' WHEN '宁' THEN '宁夏' WHEN '琼' THEN '海南'
		END
	ELSE '未知'
END AS province,
CASE
	WHEN r.is_hk_macau = 1 AND LEFT(r.plate_normalized, 2) = '粤Z' THEN
		CASE SUBSTRING(r.plate_normalized, 2, 1)
			WHEN 'A' THEN '广州' WHEN 'B' THEN '深圳' WHEN 'C' THEN '珠海' WHEN 'D' THEN '汕头'
			WHEN 'E' THEN '佛山' WHEN 'F' THEN '韶关' WHEN 'G' THEN '湛江' WHEN 'H' THEN '肇庆'
			WHEN 'J' THEN '江门' WHEN 'K' THEN '茂名' WHEN 'L' THEN '惠州' WHEN 'M' THEN '梅州'
			WHEN 'N' THEN '汕尾' WHEN 'P' THEN '河源' WHEN 'Q' THEN '阳江' WHEN 'R' THEN '清远'
			WHEN 'S' THEN '东莞' WHEN 'T' THEN '中山' WHEN 'U' THEN '潮州' WHEN 'V' THEN '揭阳'
			WHEN 'W' THEN '云浮' WHEN 'X' THEN '顺德' WHEN 'Y' THEN '南海' WHEN 'Z' THEN '港澳跨境'
			ELSE ''
		END
	WHEN LEFT(r.plate_normalized, 1) = '粤' THEN
		CASE SUBSTRING(r.plate_normalized, 2, 1)
			WHEN 'A' THEN '广州' WHEN 'B' THEN '深圳' WHEN 'C' THEN '珠海' WHEN 'D' THEN '汕头'
			WHEN 'E' THEN '佛山' WHEN 'F' THEN '韶关' WHEN 'G' THEN '湛江' WHEN 'H' THEN '肇庆'
			WHEN 'J' THEN '江门' WHEN 'K' THEN '茂名' WHEN 'L' THEN '惠州' WHEN 'M' THEN '梅州'
			WHEN 'N' THEN '汕尾' WHEN 'P' THEN '河源' WHEN 'Q' THEN '阳江' WHEN 'R' THEN '清远'
			WHEN 'S' THEN '东莞' WHEN 'T' THEN '中山' WHEN 'U' THEN '潮州' WHEN 'V' THEN '揭阳'
			WHEN 'W' THEN '云浮' WHEN 'X' THEN '顺德' WHEN 'Y' THEN '南海' WHEN 'Z' THEN '港澳跨境'
			ELSE ''
		END
	ELSE ''
END AS city,
COUNT(*) AS vehicle_count,
MAX(r.is_hk_macau) AS is_hk_macau,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM traffic_gate_record r
WHERE r.snapshot_time >= ? AND r.snapshot_time <= ?
GROUP BY metric_date, province, city`, from, to)
	return err
}

// SyncHolidaysFromCode syncs hardcoded 2026 holiday data into traffic_holiday table.
func (s *sTraffic) SyncHolidaysFromCode(ctx context.Context) error {
	db := g.DB("master")
	now := int(gtime.Timestamp())

	for dateStr, name := range chinaHolidayDates2025 {
		_, _ = db.Exec(ctx, `
INSERT INTO traffic_holiday (holiday_date, holiday_name, holiday_type)
VALUES (?, ?, 'holiday')
ON DUPLICATE KEY UPDATE holiday_name = VALUES(holiday_name), holiday_type = VALUES(holiday_type)`,
			dateStr, name)
	}
	for dateStr, name := range chinaAdjustedWorkdays2025 {
		_, _ = db.Exec(ctx, `
INSERT INTO traffic_holiday (holiday_date, holiday_name, holiday_type)
VALUES (?, ?, 'workday')
ON DUPLICATE KEY UPDATE holiday_name = VALUES(holiday_name), holiday_type = VALUES(holiday_type)`,
			dateStr, name)
	}

	for dateStr, name := range chinaHolidayDates2026 {
		_, _ = db.Exec(ctx, `
INSERT INTO traffic_holiday (holiday_date, holiday_name, holiday_type)
VALUES (?, ?, 'holiday')
ON DUPLICATE KEY UPDATE holiday_name = VALUES(holiday_name), holiday_type = VALUES(holiday_type)`,
			dateStr, name)
	}
	for dateStr, name := range chinaAdjustedWorkdays2026 {
		_, _ = db.Exec(ctx, `
INSERT INTO traffic_holiday (holiday_date, holiday_name, holiday_type)
VALUES (?, ?, 'workday')
ON DUPLICATE KEY UPDATE holiday_name = VALUES(holiday_name), holiday_type = VALUES(holiday_type)`,
			dateStr, name)
	}
	_ = now
	return nil
}

var chinaHolidayDates2025 = map[string]string{
	"2025-01-01": "元旦",
	"2025-01-28": "春节",
	"2025-01-29": "春节",
	"2025-01-30": "春节",
	"2025-01-31": "春节",
	"2025-02-01": "春节",
	"2025-02-02": "春节",
	"2025-02-03": "春节",
	"2025-02-04": "春节",
	"2025-04-04": "清明节",
	"2025-04-05": "清明节",
	"2025-04-06": "清明节",
	"2025-05-01": "劳动节",
	"2025-05-02": "劳动节",
	"2025-05-03": "劳动节",
	"2025-05-04": "劳动节",
	"2025-05-05": "劳动节",
	"2025-05-31": "端午节",
	"2025-06-01": "端午节",
	"2025-06-02": "端午节",
	"2025-10-01": "国庆节",
	"2025-10-02": "国庆节",
	"2025-10-03": "国庆节",
	"2025-10-04": "国庆节",
	"2025-10-05": "国庆节",
	"2025-10-06": "中秋节",
	"2025-10-07": "国庆节",
	"2025-10-08": "国庆节",
}

var chinaAdjustedWorkdays2025 = map[string]string{
	"2025-01-26": "春节调休工作日",
	"2025-02-08": "春节调休工作日",
	"2025-04-27": "劳动节调休工作日",
	"2025-09-28": "国庆节调休工作日",
	"2025-10-11": "国庆节调休工作日",
}

var chinaHolidayDates2026 = map[string]string{
	"2026-01-01": "元旦",
	"2026-01-02": "元旦",
	"2026-01-03": "元旦",
	"2026-02-15": "春节",
	"2026-02-16": "春节",
	"2026-02-17": "春节",
	"2026-02-18": "春节",
	"2026-02-19": "春节",
	"2026-02-20": "春节",
	"2026-02-21": "春节",
	"2026-02-22": "春节",
	"2026-02-23": "春节",
	"2026-04-04": "清明节",
	"2026-04-05": "清明节",
	"2026-04-06": "清明节",
	"2026-05-01": "劳动节",
	"2026-05-02": "劳动节",
	"2026-05-03": "劳动节",
	"2026-05-04": "劳动节",
	"2026-05-05": "劳动节",
	"2026-06-19": "端午节",
	"2026-06-20": "端午节",
	"2026-06-21": "端午节",
	"2026-09-25": "中秋节",
	"2026-09-26": "中秋节",
	"2026-09-27": "中秋节",
	"2026-10-01": "国庆节",
	"2026-10-02": "国庆节",
	"2026-10-03": "国庆节",
	"2026-10-04": "国庆节",
	"2026-10-05": "国庆节",
	"2026-10-06": "国庆节",
	"2026-10-07": "国庆节",
}

var chinaAdjustedWorkdays2026 = map[string]string{
	"2026-01-04": "元旦调休工作日",
	"2026-02-14": "春节调休工作日",
	"2026-02-28": "春节调休工作日",
	"2026-05-09": "劳动节调休工作日",
	"2026-09-20": "国庆节调休工作日",
	"2026-10-10": "国庆节调休工作日",
}

// IsDateHoliday checks whether a given date is a Chinese holiday by looking up traffic_holiday.
func IsDateHoliday(ctx context.Context, dateStr string) (bool, string) {
	record, err := g.DB("master").Model("traffic_holiday").Ctx(ctx).
		Where("holiday_date = ? AND holiday_type = 'holiday'", dateStr).One()
	if err != nil || record == nil {
		return false, ""
	}
	return true, record["holiday_name"].String()
}

// BuildDateCondition returns a WHERE condition fragment for is_holiday filtering.
func BuildDateCondition(dateField string, isHoliday bool) string {
	if isHoliday {
		return fmt.Sprintf("%s IN (SELECT holiday_date FROM traffic_holiday WHERE holiday_type = 'holiday')", dateField)
	}
	return fmt.Sprintf("(%s NOT IN (SELECT holiday_date FROM traffic_holiday WHERE holiday_type = 'holiday') OR %s IS NULL)", dateField, dateField)
}
