package traffic

import (
	"ai-chat-sql/internal/logic/plate"
	"ai-chat-sql/internal/model"
	"context"
)

func (s *sTraffic) RecognizePlate(ctx context.Context, plateStr string) model.TrafficPlateRecognition {
	r := plate.RecognizePlateFull(plateStr)
	return model.TrafficPlateRecognition{
		Plate:            r.Plate,
		Normalized:       r.Normalized,
		Origin:           r.Origin,
		RegionType:       r.RegionType,
		IsHongKongMacau:  r.IsHongKongMacau,
		IsProvinceInside: r.IsProvinceInside,
		Province:         r.Province,
		City:             r.City,
		PlateType:        r.PlateType,
		Confidence:       r.Confidence,
		Basis:            r.Basis,
	}
}
