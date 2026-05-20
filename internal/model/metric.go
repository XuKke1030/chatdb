package model

import "github.com/gogf/gf/v2/util/gconv"

type MetricCatalogItem struct {
	ID                 int64     `json:"id"`
	Topic              string    `json:"topic"`
	MetricName         string    `json:"metricName"`
	DisplayName        string    `json:"displayName"`
	Description        string    `json:"description"`
	Unit               string    `json:"unit"`
	Dimensions         []string  `json:"dimensions"`
	DefaultThreshold   *float64  `json:"defaultThreshold"`
	ThresholdDirection  string   `json:"thresholdDirection"`
	RelatedFastPath    *int      `json:"relatedFastPath"`
	ChartTypeHint      string    `json:"chartTypeHint"`
	IsActive          bool      `json:"isActive"`
	CreateTime        int       `json:"createTime"`
	UpdateTime        int       `json:"updateTime"`
}

type MetricCatalogCreate struct {
	Topic              string   `json:"topic" v:"required"`
	MetricName         string   `json:"metricName" v:"required"`
	DisplayName        string   `json:"displayName" v:"required"`
	Description        string   `json:"description"`
	Unit               string   `json:"unit"`
	Dimensions         []string `json:"dimensions"`
	DefaultThreshold   *float64 `json:"defaultThreshold"`
	ThresholdDirection string   `json:"thresholdDirection" d:"above"`
	RelatedFastPath    *int     `json:"relatedFastPath"`
	ChartTypeHint      string   `json:"chartTypeHint" d:"line"`
}

type MetricCatalogUpdate struct {
	DisplayName        *string   `json:"displayName"`
	Description         *string   `json:"description"`
	Unit               *string   `json:"unit"`
	Dimensions         *[]string `json:"dimensions"`
	DefaultThreshold   **float64 `json:"defaultThreshold"`
	ThresholdDirection *string   `json:"thresholdDirection"`
	RelatedFastPath    **int     `json:"relatedFastPath"`
	ChartTypeHint      *string   `json:"chartTypeHint"`
}

func MetricDimensionsFromDB(v any) []string {
	if v == nil {
		return nil
	}
	s := gconv.String(v)
	if s == "" || s == "null" {
		return nil
	}
	return gconv.Strings(v)
}

func MetricThresholdFromDB(v any) *float64 {
	if v == nil {
		return nil
	}
	f := gconv.Float64(v)
	return &f
}
