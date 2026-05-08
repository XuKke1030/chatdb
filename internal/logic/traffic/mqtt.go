package traffic

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gogf/gf/v2/util/grand"
)

var mqttStartOnce sync.Once

func (s *sTraffic) StartMqttSubscriber(ctx context.Context) {
	mqttStartOnce.Do(func() {
		cfg := consts.Config
		if cfg == nil || cfg.Traffic == nil || cfg.Traffic.Mqtt == nil {
			return
		}
		mqttCfg := cfg.Traffic.Mqtt
		topic := strings.TrimSpace(mqttCfg.Topic)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:    "mqtt",
			Enabled:   mqttCfg.Enabled,
			Connected: false,
			Topic:     topic,
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

		go s.runMqttSubscriber(ctx, mqttCfg)
	})
}

func (s *sTraffic) runMqttSubscriber(ctx context.Context, cfg *model.TrafficMqttConfig) {
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
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(10 * time.Second)

	topic := strings.TrimSpace(cfg.Topic)
	qos := byte(cfg.Qos)
	if cfg.Qos < 0 || cfg.Qos > 2 {
		qos = 1
	}

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		consts.Logger.Infof(ctx, "traffic mqtt connected, subscribing topic %s", topic)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   true,
			Topic:       topic,
			LatestError: "",
		})
		token := client.Subscribe(topic, qos, func(_ mqtt.Client, message mqtt.Message) {
			if err := s.HandleRawPayload(ctx, message.Topic(), message.Payload()); err != nil {
				consts.Logger.Warningf(ctx, "traffic mqtt payload failed: %s", err.Error())
			}
		})
		if token.Wait() && token.Error() != nil {
			errMessage := fmt.Sprintf("traffic mqtt subscribe failed: %s", token.Error().Error())
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
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		errMessage := "traffic mqtt connection lost"
		if err != nil {
			errMessage = fmt.Sprintf("%s: %s", errMessage, err.Error())
		}
		consts.Logger.Warning(ctx, errMessage)
		_ = s.UpdateIngestStatus(ctx, model.TrafficIngestStatusInput{
			Source:      "mqtt",
			Enabled:     true,
			Connected:   false,
			Topic:       topic,
			LatestError: errMessage,
		})
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		errMessage := fmt.Sprintf("traffic mqtt connect failed: %s", token.Error().Error())
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
	client.Disconnect(250)
}
