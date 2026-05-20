package traffic

import (
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type sTraffic struct {
	ingestStatusReady atomic.Bool
}

func init() {
	service.RegisterTraffic(NewTraffic())
}

func NewTraffic() *sTraffic {
	return &sTraffic{}
}

func (s *sTraffic) InitTables(ctx context.Context) error {
	db := g.DB("master")
	for _, sql := range trafficTableSQL() {
		if _, err := db.Exec(ctx, sql); err != nil {
			return err
		}
	}
	go autoSyncHolidays(context.Background())
	return s.ensureIngestStatus(ctx)
}

func (s *sTraffic) SaveGateRecord(ctx context.Context, in model.TrafficGateRecordInput) (bool, error) {
	if in.DeviceId == "" || in.PlateNormalized == "" || in.SnapshotTime.IsZero() {
		return false, nil
	}
	db := g.DB("master")
	count, err := db.Model("traffic_gate_record").Ctx(ctx).
		Where("device_id = ? AND plate_normalized = ? AND snapshot_time = ?", in.DeviceId, in.PlateNormalized, dbTime(in.SnapshotTime)).
		Count()
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	now := int(gtime.Timestamp())
	_, err = db.Model("traffic_gate_record").Ctx(ctx).Data(g.Map{
		"device_id":          in.DeviceId,
		"device_name":        in.DeviceName,
		"camera_ip":          in.CameraIp,
		"plate_char":         in.PlateChar,
		"plate_normalized":   in.PlateNormalized,
		"plate_type":         in.PlateType,
		"plate_color":        in.PlateColor,
		"vehicle_type":       in.VehicleType,
		"vehicle_type_ext":   in.VehicleTypeExt,
		"vehicle_color":      in.VehicleColor,
		"vehicle_speed":      in.VehicleSpeed,
		"in_dir":             in.InDir,
		"vehicle_dir":        in.VehicleDir,
		"car_drv_dir":        in.CarDrvDir,
		"lane_id":            in.LaneId,
		"lane_desc":          in.LaneDesc,
		"lane_dir_desc":      in.LaneDirDesc,
		"snapshot_time":      dbTime(in.SnapshotTime),
		"collect_time":       nullableDbTime(in.CollectTime),
		"source_insert_time": nullableDbTime(in.SourceInsertTime),
		"plate_picture":      in.PlatePicture,
		"panorama_picture":   in.PanoramaPicture,
		"vehicle_picture":    in.VehiclePicture,
		"car_pre_brand":      in.CarPreBrand,
		"car_sub_brand":      in.CarSubBrand,
		"car_year_brand":     in.CarYearBrand,
		"plate_origin":       in.PlateOrigin,
		"plate_region_type":  in.PlateRegionType,
		"is_hk_macau":        boolInt(in.IsHkMacau),
		"is_province_inside": boolInt(in.IsProvinceInside),
		"raw_payload":        in.RawPayload,
		"payload_hash":       in.PayloadHash,
		"create_time":        now,
		"update_time":        now,
	}).Insert()
	if err != nil {
		return false, err
	}
	err = s.UpsertGateDevice(ctx, model.TrafficGateDeviceInput{
		DeviceId:     in.DeviceId,
		DeviceSn:     in.DeviceId,
		DeviceName:   in.DeviceName,
		LatestSeenAt: in.SnapshotTime,
		Enabled:      true,
	})
	return err == nil, err
}

func (s *sTraffic) UpsertGateDevice(ctx context.Context, in model.TrafficGateDeviceInput) error {
	if in.DeviceId == "" {
		return nil
	}
	db := g.DB("master")
	now := int(gtime.Timestamp())
	data := g.Map{
		"device_sn":         in.DeviceSn,
		"device_name":       in.DeviceName,
		"category":          in.Category,
		"model":             in.Model,
		"connection_status": in.ConnectionStatus,
		"work_status":       in.WorkStatus,
		"region":            in.Region,
		"address":           in.Address,
		"longitude":         in.Longitude,
		"latitude":          in.Latitude,
		"owner_name":        in.OwnerName,
		"owner_phone":       in.OwnerPhone,
		"enabled":           boolInt(in.Enabled),
		"latest_seen_at":    nullableDbTime(in.LatestSeenAt),
		"last_reported_at":  nullableDbTime(in.LastReportedAt),
		"device_created_at": nullableDbTime(in.DeviceCreatedAt),
		"remark":            in.Remark,
		"update_time":       now,
	}
	count, err := db.Model("traffic_gate_device").Ctx(ctx).Where("device_id = ?", in.DeviceId).Count()
	if err != nil {
		return err
	}
	if count == 0 {
		data["device_id"] = in.DeviceId
		data["create_time"] = now
		_, err = db.Model("traffic_gate_device").Ctx(ctx).Data(data).Insert()
		return err
	}
	_, err = db.Model("traffic_gate_device").Ctx(ctx).Where("device_id = ?", in.DeviceId).Data(compactUpdate(data)).Update()
	return err
}

func (s *sTraffic) ensureIngestStatus(ctx context.Context) error {
	if s.ingestStatusReady.Load() {
		return nil
	}
	db := g.DB("master")
	count, err := db.Model("traffic_ingest_status").Ctx(ctx).Where("source = ?", "mqtt").Count()
	if err != nil {
		return err
	}
	if count > 0 {
		s.ingestStatusReady.Store(true)
		return nil
	}
	now := int(gtime.Timestamp())
	_, err = db.Model("traffic_ingest_status").Ctx(ctx).Data(g.Map{
		"source":         "mqtt",
		"enabled":        0,
		"connected":      0,
		"topic":          "",
		"today_received": 0,
		"latest_error":   "",
		"create_time":    now,
		"update_time":    now,
	}).Insert()
	if err == nil {
		s.ingestStatusReady.Store(true)
	}
	return err
}

func (s *sTraffic) WriteIngestLog(ctx context.Context, in model.TrafficIngestLogInput) error {
	if in.Source == "" {
		in.Source = "mqtt"
	}
	now := int(gtime.Timestamp())
	_, err := g.DB("master").Model("traffic_ingest_log").Ctx(ctx).Data(g.Map{
		"source":       in.Source,
		"topic":        in.Topic,
		"status":       in.Status,
		"message":      in.Message,
		"device_id":    in.DeviceId,
		"payload_hash": in.PayloadHash,
		"raw_payload":  in.RawPayload,
		"create_time":  now,
	}).Insert()
	return err
}

func (s *sTraffic) UpdateIngestStatus(ctx context.Context, in model.TrafficIngestStatusInput) error {
	if in.Source == "" {
		in.Source = "mqtt"
	}
	if err := s.ensureIngestStatus(ctx); err != nil {
		return err
	}
	db := g.DB("master")
	record, err := db.Model("traffic_ingest_status").Ctx(ctx).Where("source = ?", in.Source).One()
	if err != nil {
		return err
	}
	todayReceived := in.TodayReceived
	if in.IncrementToday && record != nil {
		todayReceived = record["today_received"].Int() + 1
	}
	data := g.Map{
		"enabled":        boolInt(in.Enabled),
		"connected":      boolInt(in.Connected),
		"topic":          in.Topic,
		"today_received": todayReceived,
		"latest_error":   in.LatestError,
		"update_time":    int(gtime.Timestamp()),
	}
	if !in.LatestReceivedAt.IsZero() {
		data["latest_received_at"] = dbTime(in.LatestReceivedAt)
	}
	_, err = db.Model("traffic_ingest_status").Ctx(ctx).Where("source = ?", in.Source).Data(data).Update()
	return err
}

func (s *sTraffic) GetIngestStatus(ctx context.Context) (*model.TrafficIngestStatusOutput, error) {
	if err := s.ensureIngestStatus(ctx); err != nil {
		return nil, err
	}
	record, err := g.DB("master").Model("traffic_ingest_status").Ctx(ctx).Where("source = ?", "mqtt").One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &model.TrafficIngestStatusOutput{}, nil
	}
	return &model.TrafficIngestStatusOutput{
		Enabled:          record["enabled"].Int() == 1,
		Connected:        record["connected"].Int() == 1,
		Topic:            record["topic"].String(),
		LatestReceivedAt: record["latest_received_at"].String(),
		TodayReceived:    record["today_received"].Int(),
		LatestError:      record["latest_error"].String(),
		UpdatedAt:        record["update_time"].String(),
	}, nil
}

func trafficTableSQL() []string {
	return []string{mysqlRecordSQL, mysqlDeviceSQL, mysqlLogSQL, mysqlStatusSQL}
}

func dbTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func nullableDbTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return dbTime(t)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func compactUpdate(data g.Map) g.Map {
	out := g.Map{}
	for key, value := range data {
		switch v := value.(type) {
		case string:
			if v == "" {
				continue
			}
		case float64:
			if v == 0 {
				continue
			}
		case nil:
			continue
		}
		out[key] = value
	}
	return out
}

const mysqlRecordSQL = `CREATE TABLE IF NOT EXISTS traffic_gate_record (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
device_id VARCHAR(64) NOT NULL,
device_name VARCHAR(128),
camera_ip VARCHAR(64),
plate_char VARCHAR(32),
plate_normalized VARCHAR(32) NOT NULL,
plate_type VARCHAR(64),
plate_color VARCHAR(64),
vehicle_type VARCHAR(64),
vehicle_type_ext VARCHAR(64),
vehicle_color VARCHAR(64),
vehicle_speed INT DEFAULT 0,
in_dir INT DEFAULT -1,
vehicle_dir VARCHAR(64),
car_drv_dir VARCHAR(64),
lane_id INT DEFAULT 0,
lane_desc VARCHAR(64),
lane_dir_desc VARCHAR(64),
snapshot_time DATETIME NOT NULL,
collect_time DATETIME NULL,
source_insert_time DATETIME NULL,
plate_picture TEXT,
panorama_picture TEXT,
vehicle_picture TEXT,
car_pre_brand VARCHAR(64),
car_sub_brand VARCHAR(64),
car_year_brand VARCHAR(32),
plate_origin VARCHAR(64),
plate_region_type VARCHAR(32),
is_hk_macau TINYINT(1) NOT NULL DEFAULT 0,
	is_province_inside TINYINT(1) NOT NULL DEFAULT 0,
raw_payload LONGTEXT,
payload_hash VARCHAR(64),
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_snapshot_time (snapshot_time),
INDEX idx_device_time (device_id, snapshot_time),
INDEX idx_plate_time (plate_normalized, snapshot_time),
INDEX idx_hk_macau_time (is_hk_macau, snapshot_time),
	INDEX idx_province_inside_time (is_province_inside, snapshot_time),
UNIQUE KEY uk_record_dedupe (device_id, plate_normalized, snapshot_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

const mysqlDeviceSQL = `CREATE TABLE IF NOT EXISTS traffic_gate_device (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
device_id VARCHAR(64) NOT NULL,
device_sn VARCHAR(64),
device_name VARCHAR(128),
category VARCHAR(64),
model VARCHAR(128),
connection_status VARCHAR(32),
work_status VARCHAR(128),
region VARCHAR(128),
address VARCHAR(255),
longitude DECIMAL(10,6) DEFAULT 0,
latitude DECIMAL(10,6) DEFAULT 0,
owner_name VARCHAR(64),
owner_phone VARCHAR(32),
enabled TINYINT(1) NOT NULL DEFAULT 1,
latest_seen_at DATETIME NULL,
last_reported_at DATETIME NULL,
device_created_at DATETIME NULL,
remark VARCHAR(255),
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_device_id (device_id),
INDEX idx_latest_seen_at (latest_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

const mysqlLogSQL = `CREATE TABLE IF NOT EXISTS traffic_ingest_log (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
source VARCHAR(32) NOT NULL,
topic VARCHAR(128),
status VARCHAR(32) NOT NULL,
message VARCHAR(512),
device_id VARCHAR(64),
payload_hash VARCHAR(64),
raw_payload LONGTEXT,
create_time INT NOT NULL,
INDEX idx_source_time (source, create_time),
INDEX idx_status_time (status, create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

const mysqlStatusSQL = `CREATE TABLE IF NOT EXISTS traffic_ingest_status (
source VARCHAR(32) PRIMARY KEY,
enabled TINYINT(1) NOT NULL DEFAULT 0,
connected TINYINT(1) NOT NULL DEFAULT 0,
topic VARCHAR(128),
latest_received_at DATETIME NULL,
today_received INT NOT NULL DEFAULT 0,
latest_error VARCHAR(512),
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
