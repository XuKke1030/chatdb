package traffic

import (
	"ai-chat-sql/internal/model"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

func (s *sTraffic) BatchVerify(ctx context.Context, sampleSize int) (*model.VerifyReport, error) {
	if sampleSize <= 0 {
		sampleSize = 500
	}
	if sampleSize > 5000 {
		sampleSize = 5000
	}

	db := g.DB("master")
	records, err := db.Ctx(ctx).Raw(`
		SELECT plate_char, plate_normalized
		FROM traffic_gate_record
		WHERE plate_char <> '' AND plate_normalized <> ''
		ORDER BY RAND()
		LIMIT ?`, sampleSize).All()
	if err != nil {
		return nil, err
	}

	report := &model.VerifyReport{
		Total:        len(records),
		ByType:       make(map[string]int),
		ByConfidence: make(map[string]int),
	}

	lowCount := 0
	for _, r := range records {
		plateChar := r["plate_char"].String()
		rec := recognizePlate(plateChar)

		report.ByType[rec.PlateType]++
		report.ByConfidence[rec.Confidence]++

		if rec.Confidence == "low" || rec.Confidence == "medium" {
			report.LowConfidence = append(report.LowConfidence, model.VerifySampleItem{
				PlateChar:  plateChar,
				Normalized: rec.Normalized,
				Recognized: rec,
			})
			lowCount++
			if lowCount >= 200 {
				continue
			}
		}
	}

	return report, nil
}

func (s *sTraffic) ExportLowConfidencePlates(ctx context.Context, sampleSize int) ([][]string, error) {
	rpt, err := s.BatchVerify(ctx, sampleSize)
	if err != nil {
		return nil, err
	}
	header := []string{"原始车牌", "标准化车牌", "类型", "置信度", "识别依据", "省份", "城市", "是否港澳", "是否省内"}
	rows := [][]string{header}
	for _, item := range rpt.LowConfidence {
		rec := item.Recognized
		rows = append(rows, []string{
			item.PlateChar,
			rec.Normalized,
			rec.PlateType,
			rec.Confidence,
			rec.Basis,
			rec.Province,
			rec.City,
			fmt.Sprintf("%v", rec.IsHongKongMacau),
			fmt.Sprintf("%v", rec.IsProvinceInside),
		})
	}
	return rows, nil
}
