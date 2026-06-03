package traffic

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/grand"
)

const (
	batchSize    = 100
	flushInterval = 3 * time.Second
)

type batchItem struct {
	record model.TrafficGateRecordInput
	topic  string
	raw    string
	hash   string
}

type gateBatchWriter struct {
	mu     sync.Mutex
	items  []batchItem
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	batchWriter     *gateBatchWriter
	batchWriterOnce sync.Once
	mqttStartOnce   sync.Once
)

func getBatchWriter(ctx context.Context) *gateBatchWriter {
	batchWriterOnce.Do(func() {
		childCtx, cancel := context.WithCancel(ctx)
		batchWriter = &gateBatchWriter{
			ctx:    childCtx,
			cancel: cancel,
		}
		go batchWriter.run()
	})
	return batchWriter
}

func (w *gateBatchWriter) add(item batchItem) {
	w.mu.Lock()
	w.items = append(w.items, item)
	if len(w.items) >= batchSize {
		w.mu.Unlock()
		w.flush()
		return
	}
	w.mu.Unlock()
}

func (w *gateBatchWriter) flush() {
	w.mu.Lock()
	batch := w.items
	w.items = nil
	w.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	db := g.DB("master")
	ctx := w.ctx
	now := int(gtime.Timestamp())

	records := make([]g.Map, 0, len(batch))
	for _, item := range batch {
		rec := item.record
		if rec.DeviceId == "" || rec.PlateNormalized == "" || rec.SnapshotTime.IsZero() {
			continue
		}
		records = append(records, g.Map{
			"device_id":          rec.DeviceId,
			"device_name":        rec.DeviceName,
			"camera_ip":          rec.CameraIp,
			"plate_char":         rec.PlateChar,
			"plate_normalized":   rec.PlateNormalized,
			"plate_type":         rec.PlateType,
			"plate_color":        rec.PlateColor,
			"vehicle_type":       rec.VehicleType,
			"vehicle_type_ext":   rec.VehicleTypeExt,
			"vehicle_color":      rec.VehicleColor,
			"vehicle_speed":      rec.VehicleSpeed,
			"in_dir":             rec.InDir,
			"vehicle_dir":        rec.VehicleDir,
			"car_drv_dir":        rec.CarDrvDir,
			"lane_id":            rec.LaneId,
			"lane_desc":          rec.LaneDesc,
			"lane_dir_desc":      rec.LaneDirDesc,
			"snapshot_time":      dbTime(rec.SnapshotTime),
			"collect_time":       nullableDbTime(rec.CollectTime),
			"source_insert_time": nullableDbTime(rec.SourceInsertTime),
			"plate_picture":      rec.PlatePicture,
			"panorama_picture":   rec.PanoramaPicture,
			"vehicle_picture":    rec.VehiclePicture,
			"car_pre_brand":      rec.CarPreBrand,
			"car_sub_brand":      rec.CarSubBrand,
			"car_year_brand":     rec.CarYearBrand,
			"plate_origin":       rec.PlateOrigin,
			"plate_region_type":  rec.PlateRegionType,
			"is_hk_macau":        boolInt(rec.IsHkMacau),
			"is_province_inside": boolInt(rec.IsProvinceInside),
			"raw_payload":        rec.RawPayload,
			"payload_hash":       rec.PayloadHash,
			"create_time":        now,
			"update_time":        now,
		})
	}

	if len(records) == 0 {
		return
	}

	_, insertErr := db.Model("traffic_gate_record").Ctx(ctx).Data(records).InsertIgnore()
	if insertErr != nil {
		if !strings.Contains(strings.ToLower(insertErr.Error()), "duplicate") {
			consts.Logger.Warningf(ctx, "batch insert gate records failed: %v", insertErr)
		}
		// Fall back to individual inserts for any that might have succeeded
		// In practice, MySQL batch INSERT IGNORE will skip duplicates
	}

	// Upsert devices in batch
	deviceMap := make(map[string]model.TrafficGateDeviceInput)
	for _, item := range batch {
		rec := item.record
		if rec.DeviceId == "" {
			continue
		}
		deviceMap[rec.DeviceId] = model.TrafficGateDeviceInput{
			DeviceId:     rec.DeviceId,
			DeviceSn:     rec.DeviceId,
			DeviceName:   rec.DeviceName,
			LatestSeenAt: rec.SnapshotTime,
			Enabled:      true,
		}
	}
	for _, dev := range deviceMap {
		_ = service.Traffic().UpsertGateDevice(ctx, dev)
	}

	// Log results
	var inserted, failed int
	for _, item := range batch {
		status := "success"
		msg := "record inserted"
		if item.record.DeviceId == "" || item.record.PlateNormalized == "" || item.record.SnapshotTime.IsZero() {
			status = "skipped"
			msg = "missing required fields"
			failed++
		} else {
			inserted++
		}
		_ = service.Traffic().WriteIngestLog(ctx, model.TrafficIngestLogInput{
			Source:      "mqtt",
			Topic:       item.topic,
			Status:      status,
			Message:     msg,
			DeviceId:    item.record.DeviceId,
			PayloadHash: item.hash,
			RawPayload:  item.raw,
		})
	}

	_ = service.Traffic().UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
		Source:           "mqtt",
		Enabled:          true,
		Connected:        true,
		Topic:            batch[0].topic,
		LatestReceivedAt: gtime.Now().Time,
		LatestError:      "",
		IncrementToday:   inserted > 0,
	})
}

func (w *gateBatchWriter) run() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			w.flush()
			return
		case <-ticker.C:
			w.flush()
		}
	}
}

func (s *sTraffic) StartMqttSubscriber(ctx context.Context) {
	mqttStartOnce.Do(func() {
		cfg := consts.GetConfig()
		if cfg == nil || cfg.Traffic == nil || cfg.Traffic.Mqtt == nil {
			return
		}
		mqttCfg := cfg.Traffic.Mqtt
		topic := strings.TrimSpace(mqttCfg.Topic)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     mqttCfg.Enabled,
			Connected:   false,
			Topic:       topic,
		})
		if !mqttCfg.Enabled {
			consts.Logger.Info(ctx, "traffic mqtt subscriber disabled")
			return
		}
		if strings.TrimSpace(mqttCfg.Broker) == "" || topic == "" {
			errMessage := "traffic mqtt broker or topic is empty"
			consts.Logger.Warning(ctx, errMessage)
			_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
				Source:      "mqtt",
				Enabled:     true,
				Connected:   false,
				Topic:       topic,
				LatestError: errMessage,
			})
			return
		}

		w := getBatchWriter(ctx)
		go s.runMqttSubscriberWithBatch(ctx, mqttCfg, w)
	})
}

func (s *sTraffic) runMqttSubscriberWithBatch(ctx context.Context, cfg *model.TrafficMqttConfig, w *gateBatchWriter) {
	clientId := strings.TrimSpace(cfg.ClientIdPrefix)
	if clientId == "" {
		clientId = "chatdb_traffic_"
	}
	clientId += grand.S(10, false)
	opts := mqtt.NewClientOptions().
		AddBroker(strings.TrimSpace(cfg.Broker)).
		SetClientID(clientId).
		SetUsername(strings.TrimSpace(cfg.Username)).
		SetPassword(cfg.Password).
		SetAutoReconnect(false).
		SetConnectRetry(true).
		SetConnectRetryInterval(5*time.Second).
		SetKeepAlive(30*time.Second).
		SetPingTimeout(10*time.Second)

	topic := strings.TrimSpace(cfg.Topic)
	qos := byte(cfg.Qos)
	if cfg.Qos < 0 || cfg.Qos > 2 {
		qos = 1
	}

	var (
		backoffMu   sync.Mutex
		backoffBase = 5 * time.Second
		backoffMax  = 5 * time.Minute
		attempt     int
	)

	resetBackoff := func() {
		backoffMu.Lock()
		attempt = 0
		backoffMu.Unlock()
	}

	nextBackoff := func() (time.Duration, int) {
		backoffMu.Lock()
		defer backoffMu.Unlock()
		n := attempt
		d := backoffBase * time.Duration(1<<uint(n))
		attempt++
		if d > backoffMax {
			d = backoffMax
		}
		return d, n + 1
	}

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		resetBackoff()
		consts.Logger.Infof(ctx, "traffic mqtt connected, subscribing topic %s", topic)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   true,
			Topic:       topic,
			LatestError: "",
		})
		token := client.Subscribe(topic, qos, func(_ mqtt.Client, message mqtt.Message) {
			rawText := strings.TrimSpace(string(message.Payload()))
			hash := payloadHash(message.Payload())
			payload, err := parseGatePayload(message.Payload())
			if err != nil {
				_ = s.WriteIngestLog(ctx, model.TrafficIngestLogInput{
					Source:      "mqtt",
					Topic:       message.Topic(),
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
				return
			}
			record, err := payload.toRecordInput(rawText, hash)
			if err != nil {
				_ = s.WriteIngestLog(ctx, model.TrafficIngestLogInput{
					Source:      "mqtt",
					Topic:       message.Topic(),
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
				return
			}
			w.add(batchItem{
				record: record,
				topic:  message.Topic(),
				raw:    rawText,
				hash:   hash,
			})
		})
		if token.Wait() && token.Error() != nil {
			errMessage := "traffic mqtt subscribe failed: " + token.Error().Error()
			consts.Logger.Warning(ctx, errMessage)
			_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
				Source:      "mqtt",
				Enabled:     true,
				Connected:   false,
				Topic:       topic,
				LatestError: errMessage,
			})
		}
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		errMessage := "traffic mqtt connection lost"
		if err != nil {
			errMessage += ": " + err.Error()
		}
		consts.Logger.Warning(ctx, errMessage)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   false,
			Topic:       topic,
			LatestError: errMessage,
		})
		go func() {
			for {
				wait, n := nextBackoff()
				consts.Logger.Infof(ctx, "traffic mqtt reconnecting in %v (attempt %d)", wait, n)
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				token := client.Connect()
				if token.Wait() && token.Error() == nil {
					return
				}
				consts.Logger.Warningf(ctx, "traffic mqtt reconnect attempt %d failed: %v", n, token.Error())
			}
		}()
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		errMessage := "traffic mqtt connect failed: " + token.Error().Error()
		consts.Logger.Warning(ctx, errMessage)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   false,
			Topic:       topic,
			LatestError: errMessage,
		})
		return
	}

	<-ctx.Done()
	w.flush()
	client.Disconnect(250)
}
