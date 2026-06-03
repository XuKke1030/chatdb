package admin

import (
	"context"

	v1 "ai-chat-sql/api/admin/v1"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
)

func (c *ControllerV1) AdminMetrics(ctx context.Context, req *v1.AdminMetricsReq) (res *v1.AdminMetricsRes, err error) {
	items, err := service.Metric().List(ctx, req.Topic, false)
	if err != nil {
		return
	}
	total := len(items)
	offset := (req.Page - 1) * req.PageSize
	end := offset + req.PageSize
	if offset > total {
		offset = total
	}
	if end > total {
		end = total
	}
	page := items[offset:end]
	list := make([]v1.AdminMetricItem, 0, len(page))
	for _, it := range page {
		list = append(list, v1.AdminMetricItem{
			ID:                 it.ID,
			Topic:              it.Topic,
			MetricName:         it.MetricName,
			DisplayName:        it.DisplayName,
			Description:        it.Description,
			Unit:               it.Unit,
			Dimensions:         it.Dimensions,
			DefaultThreshold:   it.DefaultThreshold,
			ThresholdDirection:  it.ThresholdDirection,
			RelatedFastPath:    it.RelatedFastPath,
			ChartTypeHint:      it.ChartTypeHint,
			IsActive:           it.IsActive,
			CreateTime:         it.CreateTime,
			UpdateTime:         it.UpdateTime,
		})
	}
	return &v1.AdminMetricsRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (c *ControllerV1) AdminMetricCreate(ctx context.Context, req *v1.AdminMetricCreateReq) (res *v1.AdminMetricCreateRes, err error) {
	id, err := service.Metric().Create(ctx, model.MetricCatalogCreate{
		Topic:              req.Topic,
		MetricName:         req.MetricName,
		DisplayName:        req.DisplayName,
		Description:        req.Description,
		Unit:               req.Unit,
		Dimensions:         req.Dimensions,
		DefaultThreshold:   req.DefaultThreshold,
		ThresholdDirection: req.ThresholdDirection,
		RelatedFastPath:    req.RelatedFastPath,
		ChartTypeHint:      req.ChartTypeHint,
	})
	if err != nil {
		return
	}
	return &v1.AdminMetricCreateRes{Id: id}, nil
}

func (c *ControllerV1) AdminMetricUpdate(ctx context.Context, req *v1.AdminMetricUpdateReq) (res *v1.AdminMetricUpdateRes, err error) {
	err = service.Metric().Update(ctx, req.Id, model.MetricCatalogUpdate{
		DisplayName:         req.DisplayName,
		Description:         req.Description,
		Unit:               req.Unit,
		Dimensions:         req.Dimensions,
		DefaultThreshold:   req.DefaultThreshold,
		ThresholdDirection: req.ThresholdDirection,
		RelatedFastPath:    req.RelatedFastPath,
		ChartTypeHint:      req.ChartTypeHint,
	})
	return
}

func (c *ControllerV1) AdminMetricDelete(ctx context.Context, req *v1.AdminMetricDeleteReq) (res *v1.AdminMetricDeleteRes, err error) {
	err = service.Metric().Delete(ctx, req.Id)
	return
}

func (c *ControllerV1) AdminMetricToggle(ctx context.Context, req *v1.AdminMetricToggleReq) (res *v1.AdminMetricToggleRes, err error) {
	isActive, err := service.Metric().ToggleActive(ctx, req.Id)
	if err != nil {
		return
	}
	return &v1.AdminMetricToggleRes{IsActive: isActive}, nil
}
