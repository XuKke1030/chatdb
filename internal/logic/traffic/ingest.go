package traffic

import (
	"ai-chat-sql/internal/model"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type gatePayload struct {
	Cd              string `json:"cd"`
	Date            string `json:"date"`
	VehicleDir      string `json:"VehicleDir"`
	CameraIP        string `json:"CameraIP"`
	InDir           int    `json:"InDir"`
	DeviceID        string `json:"DeviceID"`
	VehicleType     string `json:"VehicleType"`
	VehicleColor    string `json:"VehicleColor"`
	PlateType       string `json:"PlateType"`
	PlateColor      string `json:"PlateColor"`
	SnapshotTime    string `json:"SnapshotTime"`
	VehicleSpeed    int    `json:"VehicleSpeed"`
	LaneDesc        string `json:"LaneDesc"`
	LaneDirDesc     string `json:"LaneDirDesc"`
	CarDrvDir       string `json:"CarDrvDir"`
	VehicleTypeExt  string `json:"VehicleTypeExt"`
	CarPreBrand     string `json:"CarPreBrand"`
	LaneID          int    `json:"LaneID"`
	PlateChar       string `json:"PlateChar"`
	PlatePicture    string `json:"PlatePicture"`
	PanoramaPicture string `json:"PanoramaPicture"`
	VehiclePicture  string `json:"VehiclePicture"`
	CarSubBrand     string `json:"CarSubBrand"`
	CarYearBrand    string `json:"CarYearBrand"`
	DeviceName      string `json:"DeviceName"`
}

func (s *sTraffic) HandleRawPayload(ctx context.Context, topic string, raw []byte) error {
	rawText := strings.TrimSpace(string(raw))
	hash := payloadHash(raw)
	payload, err := parseGatePayload(raw)
	if err != nil {
		_ = s.WriteIngestLog(ctx, model.TrafficIngestLogInput{
			Source:      "mqtt",
			Topic:       topic,
			Status:      "failed",
			Message:     err.Error(),
			PayloadHash: hash,
			RawPayload:  rawText,
		})
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   true,
			Topic:       topic,
			LatestError: err.Error(),
		})
		return err
	}

	record, err := payload.toRecordInput(rawText, hash)
	if err != nil {
		_ = s.WriteIngestLog(ctx, model.TrafficIngestLogInput{
			Source:      "mqtt",
			Topic:       topic,
			Status:      "failed",
			Message:     err.Error(),
			DeviceId:    payload.DeviceID,
			PayloadHash: hash,
			RawPayload:  rawText,
		})
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   true,
			Topic:       topic,
			LatestError: err.Error(),
		})
		return err
	}

	inserted, err := s.SaveGateRecord(ctx, record)
	if err != nil {
		_ = s.WriteIngestLog(ctx, model.TrafficIngestLogInput{
			Source:      "mqtt",
			Topic:       topic,
			Status:      "failed",
			Message:     err.Error(),
			DeviceId:    record.DeviceId,
			PayloadHash: hash,
			RawPayload:  rawText,
		})
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   true,
			Topic:       topic,
			LatestError: err.Error(),
		})
		return err
	}

	status := "success"
	message := "record inserted"
	if !inserted {
		status = "duplicate"
		message = "record already exists"
	}
	if err = s.WriteIngestLog(ctx, model.TrafficIngestLogInput{
		Source:      "mqtt",
		Topic:       topic,
		Status:      status,
		Message:     message,
		DeviceId:    record.DeviceId,
		PayloadHash: hash,
		RawPayload:  rawText,
	}); err != nil {
		return err
	}
	return s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
		Source:           "mqtt",
		Enabled:          true,
		Connected:        true,
		Topic:            topic,
		LatestReceivedAt: time.Now(),
		LatestError:      "",
		IncrementToday:   inserted,
	})
}

func parseGatePayload(raw []byte) (*gatePayload, error) {
	var payload gatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse payload failed: %w", err)
	}
	missing := make([]string, 0, 6)
	if strings.TrimSpace(payload.Cd) == "" {
		missing = append(missing, "cd")
	}
	if strings.TrimSpace(payload.Date) == "" {
		missing = append(missing, "date")
	}
	if strings.TrimSpace(payload.DeviceID) == "" {
		missing = append(missing, "DeviceID")
	}
	if strings.TrimSpace(payload.SnapshotTime) == "" {
		missing = append(missing, "SnapshotTime")
	}
	if strings.TrimSpace(payload.PlateChar) == "" {
		missing = append(missing, "PlateChar")
	}
	if strings.TrimSpace(payload.DeviceName) == "" {
		missing = append(missing, "DeviceName")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	return &payload, nil
}

func (p *gatePayload) toRecordInput(rawText string, hash string) (model.TrafficGateRecordInput, error) {
	snapshotTime, err := parseTrafficTime(p.SnapshotTime)
	if err != nil {
		return model.TrafficGateRecordInput{}, fmt.Errorf("invalid SnapshotTime: %w", err)
	}
	collectTime, err := parseOptionalTrafficTime(p.Date)
	if err != nil {
		return model.TrafficGateRecordInput{}, fmt.Errorf("invalid date: %w", err)
	}
	sourceInsertTime, err := parseOptionalTrafficTime(p.Cd)
	if err != nil {
		return model.TrafficGateRecordInput{}, fmt.Errorf("invalid cd: %w", err)
	}
	normalized := normalizePlate(p.PlateChar)
	recognition := recognizePlate(p.PlateChar)
	return model.TrafficGateRecordInput{
		DeviceId:         strings.TrimSpace(p.DeviceID),
		DeviceName:       strings.TrimSpace(p.DeviceName),
		CameraIp:         strings.TrimSpace(p.CameraIP),
		PlateChar:        strings.TrimSpace(p.PlateChar),
		PlateNormalized:  normalized,
		PlateType:        strings.TrimSpace(p.PlateType),
		PlateColor:       strings.TrimSpace(p.PlateColor),
		VehicleType:      strings.TrimSpace(p.VehicleType),
		VehicleTypeExt:   strings.TrimSpace(p.VehicleTypeExt),
		VehicleColor:     strings.TrimSpace(p.VehicleColor),
		VehicleSpeed:     p.VehicleSpeed,
		InDir:            p.InDir,
		VehicleDir:       strings.TrimSpace(p.VehicleDir),
		CarDrvDir:        strings.TrimSpace(p.CarDrvDir),
		LaneId:           p.LaneID,
		LaneDesc:         strings.TrimSpace(p.LaneDesc),
		LaneDirDesc:      strings.TrimSpace(p.LaneDirDesc),
		SnapshotTime:     snapshotTime,
		CollectTime:      collectTime,
		SourceInsertTime: sourceInsertTime,
		PlatePicture:     strings.TrimSpace(p.PlatePicture),
		PanoramaPicture:  strings.TrimSpace(p.PanoramaPicture),
		VehiclePicture:   strings.TrimSpace(p.VehiclePicture),
		CarPreBrand:      strings.TrimSpace(p.CarPreBrand),
		CarSubBrand:      strings.TrimSpace(p.CarSubBrand),
		CarYearBrand:     strings.TrimSpace(p.CarYearBrand),
		PlateOrigin:      recognition.Origin,
		PlateRegionType:  recognition.RegionType,
		IsHkMacau:        recognition.IsHongKongMacau,
		RawPayload:       rawText,
		PayloadHash:      hash,
	}, nil
}

func parseTrafficTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05Z07:00",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func parseOptionalTrafficTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return parseTrafficTime(value)
}

func payloadHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func normalizePlate(plate string) string {
	plate = strings.TrimSpace(plate)
	plate = strings.ReplaceAll(plate, " ", "")
	plate = strings.ReplaceAll(plate, "-", "")
	plate = strings.ReplaceAll(plate, "路", "")
	plate = strings.TrimRight(plate, "警学")
	return strings.ToUpper(plate)
}
