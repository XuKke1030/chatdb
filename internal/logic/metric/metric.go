package metric

import (
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func init() {
	service.RegisterMetric(&sMetric{})
}

type sMetric struct{}

func (s *sMetric) List(ctx context.Context, topic string, activeOnly bool) ([]model.MetricCatalogItem, error) {
	db := g.DB("master")
	m := db.Model("metric_catalog").Ctx(ctx)
	if topic != "" {
		m = m.Where("topic = ?", topic)
	}
	if activeOnly {
		m = m.Where("is_active = 1")
	}
	records, err := m.OrderAsc("topic").OrderAsc("id").All()
	if err != nil {
		return nil, err
	}
	items := make([]model.MetricCatalogItem, 0, len(records))
	for _, r := range records {
		items = append(items, recordToItem(r))
	}
	return items, nil
}

func (s *sMetric) GetByID(ctx context.Context, id int64) (*model.MetricCatalogItem, error) {
	record, err := g.DB("master").Model("metric_catalog").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("metric not found: %d", id)
	}
	item := recordToItem(record)
	return &item, nil
}

func (s *sMetric) Create(ctx context.Context, in model.MetricCatalogCreate) (int64, error) {
	dimJSON, _ := json.Marshal(in.Dimensions)
	now := int(gtime.Timestamp())
	result, err := g.DB("master").Model("metric_catalog").Ctx(ctx).Data(g.Map{
		"topic":               in.Topic,
		"metric_name":         in.MetricName,
		"display_name":        in.DisplayName,
		"description":         in.Description,
		"unit":                in.Unit,
		"dimensions":          string(dimJSON),
		"default_threshold":    in.DefaultThreshold,
		"threshold_direction": in.ThresholdDirection,
		"related_fast_path":   in.RelatedFastPath,
		"chart_type_hint":     in.ChartTypeHint,
		"is_active":           1,
		"create_time":         now,
		"update_time":         now,
	}).Insert()
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func (s *sMetric) Update(ctx context.Context, id int64, in model.MetricCatalogUpdate) error {
	data := g.Map{"update_time": int(gtime.Timestamp())}
	if in.DisplayName != nil {
		data["display_name"] = *in.DisplayName
	}
	if in.Description != nil {
		data["description"] = *in.Description
	}
	if in.Unit != nil {
		data["unit"] = *in.Unit
	}
	if in.Dimensions != nil {
		dimJSON, _ := json.Marshal(*in.Dimensions)
		data["dimensions"] = string(dimJSON)
	}
	if in.DefaultThreshold != nil {
		if *in.DefaultThreshold == nil {
			data["default_threshold"] = nil
		} else {
			data["default_threshold"] = **in.DefaultThreshold
		}
	}
	if in.ThresholdDirection != nil {
		data["threshold_direction"] = *in.ThresholdDirection
	}
	if in.RelatedFastPath != nil {
		if *in.RelatedFastPath == nil {
			data["related_fast_path"] = nil
		} else {
			data["related_fast_path"] = **in.RelatedFastPath
		}
	}
	if in.ChartTypeHint != nil {
		data["chart_type_hint"] = *in.ChartTypeHint
	}
	_, err := g.DB("master").Model("metric_catalog").Ctx(ctx).
		Where("id = ?", id).Data(data).Update()
	return err
}

func (s *sMetric) Delete(ctx context.Context, id int64) error {
	_, err := g.DB("master").Model("metric_catalog").Ctx(ctx).Where("id = ?", id).Delete()
	return err
}

func (s *sMetric) ToggleActive(ctx context.Context, id int64) (bool, error) {
	record, err := g.DB("master").Model("metric_catalog").Ctx(ctx).
		Fields("is_active").Where("id = ?", id).One()
	if err != nil {
		return false, err
	}
	if record == nil {
		return false, fmt.Errorf("metric not found: %d", id)
	}
	newActive := record["is_active"].Int() == 0
	_, err = g.DB("master").Model("metric_catalog").Ctx(ctx).
		Where("id = ?", id).Data(g.Map{
		"is_active":   newActive,
		"update_time":  int(gtime.Timestamp()),
	}).Update()
	return newActive, err
}

// GetThreshold returns (thresholdValue, direction, found).
// If not found in DB, returns (0, "", false) — caller should fall back to Go constants.
func (s *sMetric) GetThreshold(ctx context.Context, topic, metricName string) (float64, string, bool) {
	record, err := g.DB("master").Model("metric_catalog").Ctx(ctx).
		Fields("default_threshold, threshold_direction").
		Where("topic = ? AND metric_name = ? AND is_active = 1", topic, metricName).
		One()
	if err != nil || record == nil {
		return 0, "", false
	}
	threshold := record["default_threshold"]
	if threshold == nil || threshold.IsNil() {
		return 0, "", false
	}
	return threshold.Float64(), record["threshold_direction"].String(), true
}

func recordToItem(r gdb.Record) model.MetricCatalogItem {
	return model.MetricCatalogItem{
		ID:                r["id"].Int64(),
		Topic:             r["topic"].String(),
		MetricName:        r["metric_name"].String(),
		DisplayName:       r["display_name"].String(),
		Description:       r["description"].String(),
		Unit:              r["unit"].String(),
		Dimensions:        model.MetricDimensionsFromDB(r["dimensions"]),
		DefaultThreshold:  model.MetricThresholdFromDB(r["default_threshold"]),
		ThresholdDirection: r["threshold_direction"].String(),
		RelatedFastPath:   func() *int { v := r["related_fast_path"].Int(); if r["related_fast_path"].IsNil() { return nil }; return &v }(),
		ChartTypeHint:     r["chart_type_hint"].String(),
		IsActive:          r["is_active"].Int() == 1,
		CreateTime:        r["create_time"].Int(),
		UpdateTime:        r["update_time"].Int(),
	}
}
