package model

type ConfigData struct {
	Server   *ServerConfig  `json:"server"`
	AiConfig *AiConfig      `json:"ai"`
	QaConfig *QaConfig      `json:"qa"`
	Uiap     *UiapConfig    `json:"uiap"`
	Jwt      []*JwtOption   `json:"jwt" dc:"JWT config"`
	DbConfig *DbConfig      `json:"db" dc:"database config"`
	Traffic  *TrafficConfig `json:"traffic"`
	Sync     *SyncConfig    `json:"sync" dc:"Sync scheduler config"`
}

type ServerConfig struct {
	Address     string `json:"address" sm:"address"`
	Mode        string `json:"mode" sm:"mode"`
	OpenapiPath string `json:"openapiPath" sm:"OpenAPI path"`
	SwaggerPath string `json:"swaggerPath" sm:"Swagger path"`
}

func (t *ConfigData) IsDebug() (ok bool) {
	if t == nil || t.Server == nil {
		return false
	}
	return t.Server.Mode == "debug"
}

type AiConfig struct {
	OpenAI   *OpenAIConfig   `json:"openai"`
	DeepSeek *DeepSeekConfig `json:"deepseek"`
	Mcp      *McpConfig      `json:"mcp"`
	Asr      *AsrConfig      `json:"asr"`
}

type OpenAIConfig struct {
	BaseUrl string `json:"baseUrl"`
	Key     string `json:"key"`
}

type DeepSeekConfig struct {
	BaseUrl string `json:"baseUrl"`
	Key     string `json:"key"`
}

type McpConfig struct {
	Address string `json:"address"`
}

type AsrConfig struct {
	BaseUrl string `json:"baseUrl"`
	Key     string `json:"key"`
	Model   string `json:"model"`
}

type QaConfig struct {
	WebSearch *QaWebSearchConfig `json:"webSearch"`
	Sync      *QaSyncConfig      `json:"sync"`
}

type QaWebSearchConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseUrl  string `json:"baseUrl"`
	Key      string `json:"key"`
}

type QaSyncConfig struct {
	Provider string         `json:"provider"`
	Aidgp    *QaAidgpConfig `json:"aidgp"`
}

type QaAidgpConfig struct {
	BaseUrl                  string `json:"baseUrl"`
	AppKey                   string `json:"appKey"`
	AppSecret                string `json:"appSecret"`
	TokenPath                string `json:"tokenPath"`
	TrafficQueryPath         string `json:"trafficQueryPath"`
	PopulationQueryPath      string `json:"populationQueryPath"`
	GridQueryPath            string `json:"gridQueryPath"`
	KnowledgeBasesPath       string `json:"knowledgeBasesPath"`
	DocumentsPath            string `json:"documentsPath"`
	DocumentSegmentsPath     string `json:"documentSegmentsPath"`
	KnowledgePermissionsPath string `json:"knowledgePermissionsPath"`
	TimeoutSeconds           int    `json:"timeoutSeconds"`
	RetryTimes               int    `json:"retryTimes"`
	TokenExpireSkewSeconds   int    `json:"tokenExpireSkewSeconds"`
}

type UiapConfig struct {
	Enabled                       bool   `json:"enabled"`
	BaseUrl                       string `json:"baseUrl"`
	ClientId                      string `json:"clientId"`
	ClientSecret                  string `json:"clientSecret"`
	TokenPath                     string `json:"tokenPath"`
	UserInfoPath                  string `json:"userInfoPath"`
	PermissionPath                string `json:"permissionPath"`
	BatchPermissionPath           string `json:"batchPermissionPath"`
	TimeoutSeconds                int    `json:"timeoutSeconds"`
	TokenExpireSkewSeconds        int    `json:"tokenExpireSkewSeconds"`
	PermissionPollIntervalSeconds int    `json:"permissionPollIntervalSeconds"`
	PermissionPollPageSize        int    `json:"permissionPollPageSize"`
}

type DbConfig struct {
	Readonly bool `json:"readonly" dc:"read-only mode"`
}

type TrafficConfig struct {
	Mqtt   *TrafficMqttConfig   `json:"mqtt"`
	Ingest *TrafficIngestConfig `json:"ingest"`
}

type TrafficMqttConfig struct {
	Enabled        bool   `json:"enabled"`
	Broker         string `json:"broker"`
	Topic          string `json:"topic"`
	ClientIdPrefix string `json:"clientIdPrefix"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Qos            int    `json:"qos"`
}

type TrafficIngestConfig struct {
	DedupeWindowDays     int `json:"dedupeWindowDays"`
	RawPayloadRetainDays int `json:"rawPayloadRetainDays"`
	NoDataWarnMinutes    int `json:"noDataWarnMinutes"`
}


type SyncConfig struct {
	IntervalSeconds int `json:"intervalSeconds" dc:"sync interval in seconds, default 1800"`
}
