package traffic

import (
	"context"

	"ai-chat-sql/api/traffic/v1"
)

type ITrafficV1 interface {
	Aggregate(ctx context.Context, req *v1.AggregateReq) (res *v1.AggregateRes, err error)
	Records(ctx context.Context, req *v1.RecordsReq) (res *v1.RecordsRes, err error)
	Devices(ctx context.Context, req *v1.DevicesReq) (res *v1.DevicesRes, err error)
	IngestStatus(ctx context.Context, req *v1.IngestStatusReq) (res *v1.IngestStatusRes, err error)
	PlateRecognize(ctx context.Context, req *v1.PlateRecognizeReq) (res *v1.PlateRecognizeRes, err error)
}
