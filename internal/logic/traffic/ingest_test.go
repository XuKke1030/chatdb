package traffic

import (
	"strings"
	"testing"
)

func TestParseGatePayloadAndRecord(t *testing.T) {
	raw := []byte(`{
		"cd": "2024-05-31 08:51:03",
		"date": "2024-05-31 08:50:45",
		"VehicleDir": "向下",
		"CameraIP": "10.172.4.166",
		"InDir": 1,
		"DeviceID": "QC00070001090100000019",
		"VehicleType": "轿车",
		"VehicleColor": "白色",
		"PlateType": "新能源牌",
		"PlateColor": "渐变绿底黑字",
		"SnapshotTime": "2024-05-31 08:50:45",
		"VehicleSpeed": 60,
		"LaneID": 2,
		"PlateChar": "粤CD65***",
		"PlatePicture": "https://example.com/plate.jpg",
		"PanoramaPicture": "https://example.com/panorama.jpg",
		"VehiclePicture": "https://example.com/vehicle.jpg",
		"CarSubBrand": "AION S",
		"CarYearBrand": "2019",
		"DeviceName": "凤凰山隧道(出高新)"
	}`)

	payload, err := parseGatePayload(raw)
	if err != nil {
		t.Fatalf("parseGatePayload() error = %v", err)
	}
	record, err := payload.toRecordInput(string(raw), payloadHash(raw))
	if err != nil {
		t.Fatalf("toRecordInput() error = %v", err)
	}

	if record.DeviceId != "QC00070001090100000019" {
		t.Fatalf("DeviceId = %q", record.DeviceId)
	}
	if record.PlateNormalized != "粤CD65***" {
		t.Fatalf("PlateNormalized = %q", record.PlateNormalized)
	}
	if record.SnapshotTime.Format("2006-01-02 15:04:05") != "2024-05-31 08:50:45" {
		t.Fatalf("SnapshotTime = %s", record.SnapshotTime)
	}
	if record.InDir != 1 || record.VehicleSpeed != 60 || record.LaneId != 2 {
		t.Fatalf("numeric fields not mapped: %+v", record)
	}
	if record.PlateOrigin != "中国内地" || record.PlateRegionType != "mainland" || record.IsHkMacau {
		t.Fatalf("plate recognition fields not mapped: %+v", record)
	}
}

func TestParseGatePayloadMissingRequired(t *testing.T) {
	_, err := parseGatePayload([]byte(`{"DeviceID":"QC001"}`))
	if err == nil {
		t.Fatal("parseGatePayload() expected error")
	}
	if !strings.Contains(err.Error(), "PlateChar") {
		t.Fatalf("missing fields error = %v", err)
	}
}
