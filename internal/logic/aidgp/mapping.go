package aidgp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-chat-sql/internal/model"
)

func externalId(syncType string, item map[string]any, fallback int) string {
	for _, key := range []string{"externalId", "external_id", "id", "code", "documentId", "segmentId", "caseNumber", "recordId"} {
		if value := stringFieldAny(item, key); value != "" {
			return value
		}
	}
	return fmt.Sprintf("%s:%d", syncType, fallback)
}

func syncVersion(item map[string]any) string {
	for _, key := range []string{"syncVersion", "sync_version", "version", "updateTime", "updatedAt"} {
		if value := stringFieldAny(item, key); value != "" {
			return value
		}
	}
	return ""
}

func rawJSON(item map[string]any) string {
	b, err := json.Marshal(item)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func payloadHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func stringFieldAny(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			value := strings.TrimSpace(fmt.Sprint(v))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func intFieldAny(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch value := v.(type) {
			case int:
				return value
			case int64:
				return int(value)
			case float64:
				return int(value)
			case json.Number:
				i, _ := value.Int64()
				return int(i)
			case string:
				var out int
				_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &out)
				return out
			}
		}
	}
	return 0
}

func floatFieldAny(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch value := v.(type) {
			case float64:
				return value
			case int:
				return float64(value)
			case int64:
				return float64(value)
			case json.Number:
				f, _ := value.Float64()
				return f
			case string:
				var out float64
				_, _ = fmt.Sscanf(strings.TrimSpace(value), "%f", &out)
				return out
			}
		}
	}
	return 0
}

func boolFieldAny(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch value := v.(type) {
			case bool:
				return value
			case int:
				return value != 0
			case float64:
				return value != 0
			case string:
				switch strings.ToLower(strings.TrimSpace(value)) {
				case "1", "true", "yes", "enabled", "active":
					return true
				}
			}
		}
	}
	return false
}

func parseTimeField(m map[string]any, keys ...string) time.Time {
	value := stringFieldAny(m, keys...)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func mapTrafficRecord(item map[string]any) (model.TrafficGateRecordInput, bool) {
	snapshotTime := parseTimeField(item, "snapshotTime", "captureTime", "passTime", "metricTime", "time")
	plate := stringFieldAny(item, "plateNormalized", "plateNo", "plate", "plateChar")
	deviceId := stringFieldAny(item, "deviceId", "gateId", "cameraId")
	if snapshotTime.IsZero() || plate == "" || deviceId == "" {
		return model.TrafficGateRecordInput{}, false
	}
	raw := rawJSON(item)
	return model.TrafficGateRecordInput{
		DeviceId:         deviceId,
		DeviceName:       stringFieldAny(item, "deviceName", "gateName", "cameraName"),
		CameraIp:         stringFieldAny(item, "cameraIp", "ip"),
		PlateChar:        plate,
		PlateNormalized:  strings.ToUpper(plate),
		PlateType:        stringFieldAny(item, "plateType"),
		PlateColor:       stringFieldAny(item, "plateColor"),
		VehicleType:      stringFieldAny(item, "vehicleType"),
		VehicleTypeExt:   stringFieldAny(item, "vehicleTypeExt"),
		VehicleColor:     stringFieldAny(item, "vehicleColor"),
		VehicleSpeed:     intFieldAny(item, "vehicleSpeed", "speed"),
		InDir:            intFieldAny(item, "inDir", "direction", "directionCode"),
		VehicleDir:       stringFieldAny(item, "vehicleDir", "directionName"),
		CarDrvDir:        stringFieldAny(item, "carDrvDir"),
		LaneId:           intFieldAny(item, "laneId"),
		LaneDesc:         stringFieldAny(item, "laneDesc", "laneName"),
		LaneDirDesc:      stringFieldAny(item, "laneDirDesc"),
		SnapshotTime:     snapshotTime,
		CollectTime:      parseTimeField(item, "collectTime"),
		SourceInsertTime: parseTimeField(item, "sourceInsertTime", "updateTime", "updatedAt"),
		PlatePicture:     stringFieldAny(item, "platePicture", "plateImage"),
		PanoramaPicture:  stringFieldAny(item, "panoramaPicture", "sceneImage"),
		VehiclePicture:   stringFieldAny(item, "vehiclePicture", "vehicleImage"),
		PlateOrigin:      stringFieldAny(item, "plateOrigin", "originCity"),
		PlateRegionType:  stringFieldAny(item, "plateRegionType", "regionType"),
		IsHkMacau:        boolFieldAny(item, "isHkMacau", "hkMacau"),
		RawPayload:       raw,
		PayloadHash:      payloadHash(raw),
	}, true
}
