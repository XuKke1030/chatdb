package model

import "time"

type TrafficGateRecordInput struct {
	DeviceId         string
	DeviceName       string
	CameraIp         string
	PlateChar        string
	PlateNormalized  string
	PlateType        string
	PlateColor       string
	VehicleType      string
	VehicleTypeExt   string
	VehicleColor     string
	VehicleSpeed     int
	InDir            int
	VehicleDir       string
	CarDrvDir        string
	LaneId           int
	LaneDesc         string
	LaneDirDesc      string
	SnapshotTime     time.Time
	CollectTime      time.Time
	SourceInsertTime time.Time
	PlatePicture     string
	PanoramaPicture  string
	VehiclePicture   string
	CarPreBrand      string
	CarSubBrand      string
	CarYearBrand     string
	PlateOrigin      string
	PlateRegionType  string
	IsHkMacau        bool
	IsProvinceInside bool
	RawPayload       string
	PayloadHash      string
}

type TrafficGateDeviceInput struct {
	DeviceId         string
	DeviceSn         string
	DeviceName       string
	Category         string
	Model            string
	ConnectionStatus string
	WorkStatus       string
	Region           string
	Address          string
	Longitude        float64
	Latitude         float64
	OwnerName        string
	OwnerPhone       string
	Enabled          bool
	LatestSeenAt     time.Time
	LastReportedAt   time.Time
	DeviceCreatedAt  time.Time
	Remark           string
}

type TrafficIngestLogInput struct {
	Source      string
	Topic       string
	Status      string
	Message     string
	DeviceId    string
	PayloadHash string
	RawPayload  string
}

type TrafficIngestStatusInput struct {
	Source           string
	Enabled          bool
	Connected        bool
	Topic            string
	LatestReceivedAt time.Time
	TodayReceived    int
	LatestError      string
	IncrementToday   bool
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
	DateFrom    string
	DateTo      string
	DeviceId    string
	Plate       string
	GroupBy     string
	Holiday     string
	GateName    string // 卡口名称过滤
	PlateRegion string // 区域名称过滤
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
	DateFrom  string
	DateTo    string
	DeviceId  string
	Plate     string
	IsHkMacau string
	Page      int
	PageSize  int
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
	Enabled string
	Keyword string
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