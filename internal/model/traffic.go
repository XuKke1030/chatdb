package model

import "time"

type TrafficGateRecordInput struct {
	DeviceId         string    `json:"deviceId"         dc:"设备ID"`
	DeviceName       string    `json:"deviceName"       dc:"设备名称"`
	CameraIp         string    `json:"cameraIp"         dc:"摄像头IP"`
	PlateChar        string    `json:"plateChar"        dc:"原始车牌"`
	PlateNormalized  string    `json:"plateNormalized"  dc:"标准化车牌"`
	PlateType        string    `json:"plateType"        dc:"车牌类型"`
	PlateColor       string    `json:"plateColor"       dc:"车牌颜色"`
	VehicleType      string    `json:"vehicleType"      dc:"车辆类型"`
	VehicleTypeExt   string    `json:"vehicleTypeExt"   dc:"车辆扩展类型"`
	VehicleColor     string    `json:"vehicleColor"     dc:"车身颜色"`
	VehicleSpeed     int       `json:"vehicleSpeed"     dc:"车速"`
	InDir            int       `json:"inDir"            dc:"进方向"`
	VehicleDir       string    `json:"vehicleDir"       dc:"车辆方向"`
	CarDrvDir        string    `json:"carDrvDir"        dc:"行驶方向"`
	LaneId           int       `json:"laneId"           dc:"车道ID"`
	LaneDesc         string    `json:"laneDesc"         dc:"车道描述"`
	LaneDirDesc      string    `json:"laneDirDesc"      dc:"车道方向描述"`
	SnapshotTime     time.Time `json:"snapshotTime"     dc:"抓拍时间"`
	CollectTime      time.Time `json:"collectTime"      dc:"采集时间"`
	SourceInsertTime time.Time `json:"sourceInsertTime" dc:"源入库时间"`
	PlatePicture     string    `json:"platePicture"     dc:"车牌图片"`
	PanoramaPicture  string    `json:"panoramaPicture"  dc:"全景图片"`
	VehiclePicture   string    `json:"vehiclePicture"   dc:"车辆图片"`
	CarPreBrand      string    `json:"carPreBrand"      dc:"品牌"`
	CarSubBrand      string    `json:"carSubBrand"      dc:"子品牌"`
	CarYearBrand     string    `json:"carYearBrand"     dc:"年款"`
	PlateOrigin      string    `json:"plateOrigin"      dc:"车牌来源地"`
	PlateRegionType  string    `json:"plateRegionType"  dc:"车牌区域类型"`
	IsHkMacau        bool      `json:"isHkMacau"        dc:"是否港澳车"`
	IsProvinceInside bool      `json:"isProvinceInside" dc:"是否省内车"`
	RawPayload       string    `json:"rawPayload"       dc:"原始载荷"`
	PayloadHash      string    `json:"payloadHash"      dc:"载荷哈希"`
}

type TrafficGateDeviceInput struct {
	DeviceId         string    `json:"deviceId"         dc:"设备ID"`
	DeviceSn         string    `json:"deviceSn"         dc:"设备序列号"`
	DeviceName       string    `json:"deviceName"       dc:"设备名称"`
	Category         string    `json:"category"         dc:"设备分类"`
	Model            string    `json:"model"             dc:"设备型号"`
	ConnectionStatus string    `json:"connectionStatus" dc:"连接状态"`
	WorkStatus       string    `json:"workStatus"       dc:"工作状态"`
	Region           string    `json:"region"            dc:"所属区域"`
	Address          string    `json:"address"           dc:"安装地址"`
	Longitude        float64   `json:"longitude"         dc:"经度"`
	Latitude         float64   `json:"latitude"          dc:"纬度"`
	OwnerName        string    `json:"ownerName"         dc:"归属人姓名"`
	OwnerPhone       string    `json:"ownerPhone"        dc:"归属人电话"`
	Enabled          bool      `json:"enabled"           dc:"是否启用"`
	LatestSeenAt     time.Time `json:"latestSeenAt"      dc:"最近在线时间"`
	LastReportedAt   time.Time `json:"lastReportedAt"    dc:"最后上报时间"`
	DeviceCreatedAt  time.Time `json:"deviceCreatedAt"  dc:"设备创建时间"`
	Remark           string    `json:"remark"            dc:"备注"`
}

type TrafficIngestLogInput struct {
	Source      string `json:"source"      dc:"来源"`
	Topic       string `json:"topic"       dc:"主题"`
	Status      string `json:"status"      dc:"状态"`
	Message     string `json:"message"     dc:"消息"`
	DeviceId    string `json:"deviceId"     dc:"设备ID"`
	PayloadHash string `json:"payloadHash" dc:"载荷哈希"`
	RawPayload  string `json:"rawPayload"  dc:"原始载荷"`
}

type TrafficIngestStatusInput struct {
	Source           string    `json:"source"          dc:"来源"`
	Enabled          bool      `json:"enabled"          dc:"是否启用"`
	Connected        bool      `json:"connected"        dc:"是否已连接"`
	Topic            string    `json:"topic"            dc:"订阅主题"`
	LatestReceivedAt time.Time `json:"latestReceivedAt" dc:"最近接收时间"`
	TodayReceived    int       `json:"todayReceived"    dc:"今日接收数"`
	LatestError      string    `json:"latestError"      dc:"最近错误"`
	IncrementToday   bool      `json:"incrementToday"   dc:"今日是否有增量"`
}

type TrafficPlateRecognition struct {
	Plate            string `json:"plate"`
	Normalized       string `json:"normalized"`
	Origin           string `json:"origin"`
	RegionType       string `json:"regionType"`
	IsHongKongMacau  bool   `json:"isHongKongMacau"`
	IsProvinceInside bool   `json:"isProvinceInside"`
	Province         string `json:"province,omitempty"`
	City             string `json:"city,omitempty"`
	PlateType        string `json:"plateType"`
	Confidence       string `json:"confidence"`
	Basis            string `json:"basis"`
}

type TrafficAggregateQuery struct {
	DateFrom    string `json:"dateFrom"    dc:"起始日期 Y-m-d"`
	DateTo      string `json:"dateTo"      dc:"截止日期 Y-m-d"`
	DeviceId    string `json:"deviceId"    dc:"设备ID"`
	Plate       string `json:"plate"       dc:"车牌号"`
	GroupBy     string `json:"groupBy"     dc:"分组方式：hour/day/device"`
	Holiday     string `json:"holiday"     dc:"节假日筛选：holiday/workday"`
	GateName    string `json:"gateName"    dc:"卡口名称过滤"`
	PlateRegion string `json:"plateRegion" dc:"区域名称过滤"`
}

type TrafficAggregateSummary struct {
	Total              int     `json:"total"`
	InCount            int     `json:"inCount"`
	OutCount           int     `json:"outCount"`
	HkMacauCount       int     `json:"hkMacauCount"`
	HkMacauRatio       float64 `json:"hkMacauRatio"`
	MainlandCount      int     `json:"mainlandCount"`
	UnknownDirCount    int     `json:"unknownDirCount"`
	ProvinceInsideCount  int   `json:"provinceInsideCount"`
	ProvinceOutsideCount int   `json:"provinceOutsideCount"`
	ProvinceInsideRatio  float64 `json:"provinceInsideRatio"`
}

type TrafficAggregateSeriesItem struct {
	Name                string `json:"name"`
	Total               int    `json:"total"`
	InCount             int    `json:"inCount"`
	OutCount            int    `json:"outCount"`
	HkMacauCount        int    `json:"hkMacauCount"`
	ProvinceInsideCount  int   `json:"provinceInsideCount"`
	ProvinceOutsideCount int   `json:"provinceOutsideCount"`
}

type TrafficAggregateResult struct {
	Summary TrafficAggregateSummary      `json:"summary"`
	Series  []TrafficAggregateSeriesItem `json:"series"`
	GroupBy string                       `json:"groupBy"`
}

type TrafficRecordQuery struct {
	DateFrom  string `json:"dateFrom"  dc:"起始日期 Y-m-d"`
	DateTo    string `json:"dateTo"    dc:"截止日期 Y-m-d"`
	DeviceId  string `json:"deviceId"  dc:"设备ID"`
	Plate     string `json:"plate"     dc:"车牌号"`
	IsHkMacau string `json:"isHkMacau" dc:"是否港澳车"`
	Page      int    `json:"page"      dc:"页码"`
	PageSize  int    `json:"pageSize"  dc:"每页条数"`
}

type TrafficRecordItem struct {
	Id               int64   `json:"id"`
	DeviceId         string  `json:"deviceId"`
	DeviceName       string  `json:"deviceName"`
	CameraIp         string  `json:"cameraIp"`
	PlateChar        string  `json:"plateChar"`
	PlateNormalized  string  `json:"plateNormalized"`
	PlateType        string  `json:"plateType"`
	PlateColor       string  `json:"plateColor"`
	VehicleType      string  `json:"vehicleType"`
	VehicleTypeExt   string  `json:"vehicleTypeExt"`
	VehicleColor     string  `json:"vehicleColor"`
	VehicleSpeed     int     `json:"vehicleSpeed"`
	InDir            int     `json:"inDir"`
	InDirName        string  `json:"inDirName"`
	VehicleDir       string  `json:"vehicleDir"`
	CarDrvDir        string  `json:"carDrvDir"`
	LaneId           int     `json:"laneId"`
	SnapshotTime     string  `json:"snapshotTime"`
	CollectTime      string  `json:"collectTime"`
	SourceInsertTime string  `json:"sourceInsertTime"`
	PlateOrigin      string  `json:"plateOrigin"`
	PlateRegionType  string  `json:"plateRegionType"`
	IsHkMacau        bool    `json:"isHkMacau"`
	PlatePicture     string  `json:"platePicture"`
	PanoramaPicture  string  `json:"panoramaPicture"`
	VehiclePicture   string  `json:"vehiclePicture"`
	Longitude        float64 `json:"longitude,omitempty"`
	Latitude         float64 `json:"latitude,omitempty"`
}

type TrafficRecordListResult struct {
	List     []TrafficRecordItem `json:"list"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

type TrafficDeviceQuery struct {
	Enabled string `json:"enabled" dc:"启用状态筛选"`
	Keyword string `json:"keyword" dc:"搜索关键词"`
}

type TrafficDeviceItem struct {
	Id               int64   `json:"id"`
	DeviceId         string  `json:"deviceId"`
	DeviceSn         string  `json:"deviceSn"`
	DeviceName       string  `json:"deviceName"`
	Category         string  `json:"category"`
	Model            string  `json:"model"`
	ConnectionStatus string  `json:"connectionStatus"`
	WorkStatus       string  `json:"workStatus"`
	Region           string  `json:"region"`
	Address          string  `json:"address"`
	Longitude        float64 `json:"longitude"`
	Latitude         float64 `json:"latitude"`
	OwnerName        string  `json:"ownerName"`
	OwnerPhone       string  `json:"ownerPhone"`
	Enabled          bool    `json:"enabled"`
	LatestSeenAt     string  `json:"latestSeenAt"`
	LastReportedAt   string  `json:"lastReportedAt"`
	DeviceCreatedAt  string  `json:"deviceCreatedAt"`
	Remark           string  `json:"remark"`
}

type TrafficDeviceListResult struct {
	List []TrafficDeviceItem `json:"list"`
}

type TrafficIngestStatusOutput struct {
	Enabled          bool   `json:"enabled"`
	Connected        bool   `json:"connected"`
	Topic            string `json:"topic"`
	LatestReceivedAt string `json:"latestReceivedAt"`
	TodayReceived    int    `json:"todayReceived"`
	LatestError      string `json:"latestError"`
	UpdatedAt        string `json:"updatedAt"`
}

type TrafficStayBucketItem struct {
	Bucket        string  `json:"bucket"`
	VehicleCount  int     `json:"vehicleCount"`
	AvgStayMinutes float64 `json:"avgStayMinutes"`
}

type TrafficOriginRankItem struct {
	Province      string `json:"province"`
	City          string `json:"city"`
	VehicleCount  int    `json:"vehicleCount"`
	IsHkMacau     bool   `json:"isHkMacau"`
}

type TrafficYoYCompareItem struct {
	Name       string `json:"name"`
	CurrentVal int    `json:"currentVal"`
	PriorVal   int    `json:"priorVal"`
	ChangePct  float64 `json:"changePct"`
}


type VerifyReport struct {
	Total         int                     `json:"total"`
	ByType        map[string]int          `json:"byType"`
	ByConfidence  map[string]int          `json:"byConfidence"`
	LowConfidence []VerifySampleItem      `json:"lowConfidence"`
}

type VerifySampleItem struct {
	PlateChar  string                    `json:"plateChar"`
	Normalized string                    `json:"normalized"`
	Recognized TrafficPlateRecognition   `json:"recognized"`
}