package v1

import (
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

type AggregateReq struct {
	g.Meta   `path:"/traffic/aggregate" method:"get" tags:"V1/车流" sm:"车流聚合统计" dc:"按时间、卡口、车牌、进出方向、港澳车维度统计车流"`
	DateFrom string `json:"dateFrom" in:"query" dc:"开始日期或时间"`
	DateTo   string `json:"dateTo" in:"query" dc:"结束日期或时间"`
	DeviceId string `json:"deviceId" in:"query" dc:"设备编码"`
	Plate    string `json:"plate" in:"query" dc:"车牌"`
	GroupBy  string `json:"groupBy" in:"query" dc:"hour/day/gate/plateRegion/inDir"`
	Holiday  string `json:"holiday" in:"query" dc:"是否节假日，预留"`
}

type AggregateRes struct {
	model.TrafficAggregateResult
}

type RecordsReq struct {
	g.Meta    `path:"/traffic/records" method:"get" tags:"V1/车流" sm:"车流明细查询" dc:"查询卡口过车明细"`
	DateFrom  string `json:"dateFrom" in:"query" dc:"开始日期或时间"`
	DateTo    string `json:"dateTo" in:"query" dc:"结束日期或时间"`
	DeviceId  string `json:"deviceId" in:"query" dc:"设备编码"`
	Plate     string `json:"plate" in:"query" dc:"车牌"`
	IsHkMacau string `json:"isHkMacau" in:"query" dc:"是否港澳车 true/false/1/0"`
	Page      int    `json:"page" in:"query" dc:"页码"`
	PageSize  int    `json:"pageSize" in:"query" dc:"每页数量"`
}

type RecordsRes struct {
	model.TrafficRecordListResult
}

type DevicesReq struct {
	g.Meta  `path:"/traffic/devices" method:"get" tags:"V1/车流" sm:"卡口设备列表" dc:"查询车流卡口设备档案"`
	Enabled string `json:"enabled" in:"query" dc:"是否启用 true/false/1/0"`
	Keyword string `json:"keyword" in:"query" dc:"设备名称、编码、地址关键字"`
}

type DevicesRes struct {
	model.TrafficDeviceListResult
}

type IngestStatusReq struct {
	g.Meta `path:"/traffic/ingest/status" method:"get" tags:"V1/车流" sm:"车流接入状态" dc:"查询 MQTT 数据接入状态"`
}

type IngestStatusRes struct {
	model.TrafficIngestStatusOutput
}

type PlateRecognizeReq struct {
	g.Meta `path:"/traffic/plate/recognize" method:"get" tags:"V1/车流" sm:"车牌归属识别" dc:"识别车牌归属地、类型和是否港澳车"`
	Plate  string `json:"plate" in:"query" v:"required#车牌不能为空" dc:"车牌号"`
}

type PlateRecognizeRes struct {
	model.TrafficPlateRecognition
}
