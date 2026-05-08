package traffic

import (
	"context"

	v1 "ai-chat-sql/api/traffic/v1"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
)

func (c *ControllerV1) Aggregate(ctx context.Context, req *v1.AggregateReq) (res *v1.AggregateRes, err error) {
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: req.DateFrom,
		DateTo:   req.DateTo,
		DeviceId: req.DeviceId,
		Plate:    req.Plate,
		GroupBy:  req.GroupBy,
		Holiday:  req.Holiday,
	})
	if err != nil {
		return nil, err
	}
	return &v1.AggregateRes{TrafficAggregateResult: *result}, nil
}

func (c *ControllerV1) Records(ctx context.Context, req *v1.RecordsReq) (res *v1.RecordsRes, err error) {
	result, err := service.Traffic().ListRecords(ctx, model.TrafficRecordQuery{
		DateFrom:  req.DateFrom,
		DateTo:    req.DateTo,
		DeviceId:  req.DeviceId,
		Plate:     req.Plate,
		IsHkMacau: req.IsHkMacau,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RecordsRes{TrafficRecordListResult: *result}, nil
}

func (c *ControllerV1) Devices(ctx context.Context, req *v1.DevicesReq) (res *v1.DevicesRes, err error) {
	result, err := service.Traffic().ListDevices(ctx, model.TrafficDeviceQuery{
		Enabled: req.Enabled,
		Keyword: req.Keyword,
	})
	if err != nil {
		return nil, err
	}
	return &v1.DevicesRes{TrafficDeviceListResult: *result}, nil
}

func (c *ControllerV1) IngestStatus(ctx context.Context, req *v1.IngestStatusReq) (res *v1.IngestStatusRes, err error) {
	result, err := service.Traffic().GetIngestStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.IngestStatusRes{TrafficIngestStatusOutput: *result}, nil
}

func (c *ControllerV1) PlateRecognize(ctx context.Context, req *v1.PlateRecognizeReq) (res *v1.PlateRecognizeRes, err error) {
	result := service.Traffic().RecognizePlate(ctx, req.Plate)
	return &v1.PlateRecognizeRes{TrafficPlateRecognition: result}, nil
}
