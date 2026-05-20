package service

import (
	"ai-chat-sql/internal/model"
	"context"
)

type IMetric interface {
	List(ctx context.Context, topic string, activeOnly bool) ([]model.MetricCatalogItem, error)
	GetByID(ctx context.Context, id int64) (*model.MetricCatalogItem, error)
	Create(ctx context.Context, in model.MetricCatalogCreate) (int64, error)
	Update(ctx context.Context, id int64, in model.MetricCatalogUpdate) error
	Delete(ctx context.Context, id int64) error
	ToggleActive(ctx context.Context, id int64) (bool, error)
	GetThreshold(ctx context.Context, topic, metricName string) (float64, string, bool)
	Bootstrap(ctx context.Context) error
}

var localMetric IMetric

func Metric() IMetric {
	return localMetric
}

func RegisterMetric(i IMetric) {
	localMetric = i
}
