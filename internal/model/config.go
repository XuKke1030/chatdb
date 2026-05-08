package model

type ConfigData struct {
	Server   *ServerConfig  `json:"server"`
	AiConfig *AiConfig      `json:"ai"`
	QaConfig *QaConfig      `json:"qa"`
	Jwt      []*JwtOption   `json:"jwt" dc:"JWT配置"`
	DbConfig *DbConfig      `json:"db" dc:"数据库配置"`
	Traffic  *TrafficConfig `json:"traffic"`
}

// ServerConfig 服务配置
type ServerConfig struct {
	Address     string `json:"address" sm:"地址" dc:"服务器监听地址，格式如: ':8000'"`
	Mode        string `json:"mode" sm:"运行模式" dc:"运行模式，可选值：debug, release"`
	OpenapiPath string `json:"openapiPath" sm:"OpenAPI路径" dc:"OpenAPI文档的访问路径，例：'/api.json'"`
	SwaggerPath string `json:"swaggerPath" sm:"Swagger路径" dc:"Swagger UI的访问路径，例：'/swagger'"`
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
	BaseUrl   string `json:"baseUrl"`
	AppKey    string `json:"appKey"`
	AppSecret string `json:"appSecret"`
}

// DbConfig 数据库配置
type DbConfig struct {
	Readonly bool `json:"readonly" dc:"是否启用只读模式，启用后只允许执行查询操作（SELECT语句）"`
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
