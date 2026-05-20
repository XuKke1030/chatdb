package metric

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type seedRow struct {
	Topic              string
	MetricName         string
	DisplayName        string
	Description        string
	Unit               string
	DefaultThreshold   *float64
	ThresholdDirection string
	RelatedFastPath    *int
	ChartTypeHint      string
}

func intPtr(v int) *int       { return &v }
func floatPtr(v float64) *float64 { return &v }

var seeds = []seedRow{
	{
		Topic: "traffic", MetricName: "traffic_daily_total",
		DisplayName: "车流日总量", Description: "当天卡口通行记录总数",
		Unit: "辆", DefaultThreshold: floatPtr(50000),
		ThresholdDirection: "above", RelatedFastPath: intPtr(0), ChartTypeHint: "line",
	},
	{
		Topic: "traffic", MetricName: "hk_macau_ratio_pct",
		DisplayName: "港澳车占比", Description: "港澳车牌车辆占总车流的比例",
		Unit: "%", DefaultThreshold: floatPtr(15.0),
		ThresholdDirection: "above", RelatedFastPath: intPtr(3), ChartTypeHint: "pie",
	},
	{
		Topic: "traffic", MetricName: "province_outside_pct",
		DisplayName: "省外车占比", Description: "非广东省牌照车辆占内地车的比例",
		Unit: "%", DefaultThreshold: floatPtr(40.0),
		ThresholdDirection: "above", RelatedFastPath: intPtr(10), ChartTypeHint: "bar",
	},
	{
		Topic: "traffic", MetricName: "yoy_change_pct",
		DisplayName: "同比变化率", Description: "当前周期与去年同期车流变化百分比",
		Unit: "%", DefaultThreshold: floatPtr(20.0),
		ThresholdDirection: "above", RelatedFastPath: intPtr(11), ChartTypeHint: "bar",
	},
	{
		Topic: "traffic", MetricName: "mom_change_pct",
		DisplayName: "环比变化率", Description: "当前周期与上月同期车流变化百分比",
		Unit: "%", DefaultThreshold: floatPtr(15.0),
		ThresholdDirection: "above", RelatedFastPath: intPtr(12), ChartTypeHint: "bar",
	},
	{
		Topic: "grid", MetricName: "close_rate_pct",
		DisplayName: "结案率", Description: "已结案件数占总案件数的比例",
		Unit: "%", DefaultThreshold: floatPtr(85.0),
		ThresholdDirection: "below", RelatedFastPath: intPtr(14), ChartTypeHint: "bar",
	},
	{
		Topic: "traffic", MetricName: "stay_long_bucket",
		DisplayName: "长停留占比", Description: "停留超过4小时的车辆占比",
		Unit: "", DefaultThreshold: nil,
		ThresholdDirection: "above", RelatedFastPath: intPtr(8), ChartTypeHint: "pie",
	},
}

// Bootstrap inserts seed rows if the metric_catalog table is empty.
func (s *sMetric) Bootstrap(ctx context.Context) error {
	count, err := g.DB("master").Model("metric_catalog").Ctx(ctx).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := int(gtime.Timestamp())
	for _, row := range seeds {
		_, err := g.DB("master").Model("metric_catalog").Ctx(ctx).Data(g.Map{
			"topic":               row.Topic,
			"metric_name":         row.MetricName,
			"display_name":        row.DisplayName,
			"description":         row.Description,
			"unit":                row.Unit,
			"dimensions":          "[]",
			"default_threshold":    row.DefaultThreshold,
			"threshold_direction": row.ThresholdDirection,
			"related_fast_path":   row.RelatedFastPath,
			"chart_type_hint":     row.ChartTypeHint,
			"is_active":           1,
			"create_time":         now,
			"update_time":         now,
		}).Insert()
		if err != nil {
			return err
		}
	}
	return nil
}
