package service

import (
	"ai-chat-sql/internal/model"
	"context"
)

type (
	ITraffic interface {
		InitTables(ctx context.Context) error
		StartMqttSubscriber(ctx context.Context)
		HandleRawPayload(ctx context.Context, topic string, raw []byte) error
		RecognizePlate(ctx context.Context, plate string) model.TrafficPlateRecognition
		Aggregate(ctx context.Context, query model.TrafficAggregateQuery) (*model.TrafficAggregateResult, error)
		ListRecords(ctx context.Context, query model.TrafficRecordQuery) (*model.TrafficRecordListResult, error)
		ListDevices(ctx context.Context, query model.TrafficDeviceQuery) (*model.TrafficDeviceListResult, error)
		GetIngestStatus(ctx context.Context) (*model.TrafficIngestStatusOutput, error)
		SaveGateRecord(ctx context.Context, in model.TrafficGateRecordInput) (bool, error)
		UpsertGateDevice(ctx context.Context, in model.TrafficGateDeviceInput) error
		WriteIngestLog(ctx context.Context, in model.TrafficIngestLogInput) error
		UpdateIngestStatus(ctx context.Context, in model.TrafficIngestStatusInput) error
	}
)

var localTraffic ITraffic

func Traffic() ITraffic {
	if localTraffic == nil {
		panic("implement not found for interface ITraffic, forgot register?")
	}
	return localTraffic
}

func RegisterTraffic(i ITraffic) {
	localTraffic = i
}
