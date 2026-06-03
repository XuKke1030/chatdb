package consts

import (
	"ai-chat-sql/internal/model"
	"os"
	"strconv"
	"sync"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	mcpclient "github.com/mark3labs/mcp-go/client"
)

var (
	Ctx    = gctx.New()
	Logger = g.Log()
)

var (
	systemConfig     *gjson.Json
	systemConfigMu   sync.RWMutex
)

func SystemConfig() *gjson.Json {
	systemConfigMu.RLock()
	defer systemConfigMu.RUnlock()
	return systemConfig
}

func setSystemConfig(cfg *gjson.Json) {
	systemConfigMu.Lock()
	defer systemConfigMu.Unlock()
	systemConfig = cfg
}

var (
	config   *model.ConfigData
	configMu sync.RWMutex
)

func GetConfig() *model.ConfigData {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}

func setConfig(cfg *model.ConfigData) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
}

var (
	mcpClient   *mcpclient.Client
	mcpClientMu sync.RWMutex
)

func McpClientInstance() *mcpclient.Client {
	mcpClientMu.RLock()
	defer mcpClientMu.RUnlock()
	return mcpClient
}

func SetMcpClient(c *mcpclient.Client) {
	mcpClientMu.Lock()
	defer mcpClientMu.Unlock()
	mcpClient = c
}

var (
	privateKey   string = ""
	privateKeyMu sync.RWMutex
)

func PrivateKeyValue() string {
	privateKeyMu.RLock()
	defer privateKeyMu.RUnlock()
	return privateKey
}

func SetPrivateKey(key string) {
	privateKeyMu.Lock()
	defer privateKeyMu.Unlock()
	privateKey = key
}

const JwtSubjectUser = "ai-chat-user"
const JwtSubjectAdmin = "ai-chat-admin"

func init() {
	if err := initSystemConfig(); err != nil {
		panic(err)
	}
}

func initSystemConfig() (err error) {
	data, err := g.Cfg().Data(Ctx)
	if err != nil {
		return
	}
	setSystemConfig(gjson.New(data))
	var cfg model.ConfigData
	if err = SystemConfig().Scan(&cfg); err != nil {
		return
	}
	applyEnvOverrides(&cfg)
	setConfig(&cfg)
	return
}

func applyEnvOverrides(cfg *model.ConfigData) {
	if cfg == nil {
		cfg = &model.ConfigData{}
	}
	if cfg.Server == nil {
		cfg.Server = &model.ServerConfig{}
	}
	if cfg.AiConfig == nil {
		cfg.AiConfig = &model.AiConfig{}
	}
	if cfg.AiConfig.OpenAI == nil {
		cfg.AiConfig.OpenAI = &model.OpenAIConfig{}
	}
	if cfg.AiConfig.DeepSeek == nil {
		cfg.AiConfig.DeepSeek = &model.DeepSeekConfig{}
	}
	if cfg.AiConfig.Mcp == nil {
		cfg.AiConfig.Mcp = &model.McpConfig{}
	}
	if cfg.AiConfig.Asr == nil {
		cfg.AiConfig.Asr = &model.AsrConfig{}
	}
	if cfg.QaConfig == nil {
		cfg.QaConfig = &model.QaConfig{}
	}
	if cfg.QaConfig.WebSearch == nil {
		cfg.QaConfig.WebSearch = &model.QaWebSearchConfig{}
	}
	if cfg.QaConfig.Sync == nil {
		cfg.QaConfig.Sync = &model.QaSyncConfig{}
	}
	if cfg.QaConfig.Sync.Aidgp == nil {
		cfg.QaConfig.Sync.Aidgp = &model.QaAidgpConfig{}
	}
	if cfg.Uiap == nil {
		cfg.Uiap = &model.UiapConfig{}
	}
	if cfg.DbConfig == nil {
		cfg.DbConfig = &model.DbConfig{}
	}
	if cfg.Traffic == nil {
		cfg.Traffic = &model.TrafficConfig{}
	}
	if cfg.Traffic.Mqtt == nil {
		cfg.Traffic.Mqtt = &model.TrafficMqttConfig{}
	}
	if cfg.Traffic.Ingest == nil {
		cfg.Traffic.Ingest = &model.TrafficIngestConfig{}
	}
	if cfg.Sync == nil {
		cfg.Sync = &model.SyncConfig{}
	}

	setStringFromEnv(&cfg.Server.Address, "CHATDB_SERVER_ADDRESS")
	setStringFromEnv(&cfg.Server.Mode, "CHATDB_SERVER_MODE")
	setStringFromEnv(&cfg.AiConfig.OpenAI.BaseUrl, "CHATDB_OPENAI_BASE_URL")
	setStringFromEnv(&cfg.AiConfig.OpenAI.Key, "CHATDB_OPENAI_KEY")
	setStringFromEnv(&cfg.AiConfig.DeepSeek.BaseUrl, "CHATDB_DEEPSEEK_BASE_URL")
	setStringFromEnv(&cfg.AiConfig.DeepSeek.Key, "CHATDB_DEEPSEEK_KEY")
	setStringFromEnv(&cfg.AiConfig.Mcp.Address, "CHATDB_MCP_ADDRESS")
	setStringFromEnv(&cfg.AiConfig.Asr.BaseUrl, "CHATDB_ASR_BASE_URL")
	setStringFromEnv(&cfg.AiConfig.Asr.Key, "CHATDB_ASR_KEY")
	setStringFromEnv(&cfg.AiConfig.Asr.Model, "CHATDB_ASR_MODEL")
	setStringFromEnv(&cfg.QaConfig.WebSearch.Provider, "CHATDB_QA_WEB_SEARCH_PROVIDER")
	setStringFromEnv(&cfg.QaConfig.WebSearch.BaseUrl, "CHATDB_QA_WEB_SEARCH_BASE_URL")
	setStringFromEnv(&cfg.QaConfig.WebSearch.Key, "CHATDB_QA_WEB_SEARCH_KEY")
	setStringFromEnv(&cfg.QaConfig.Sync.Provider, "CHATDB_QA_SYNC_PROVIDER")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.BaseUrl, "CHATDB_QA_AIDGP_BASE_URL")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.AppKey, "CHATDB_QA_AIDGP_APP_KEY")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.AppSecret, "CHATDB_QA_AIDGP_APP_SECRET")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.TokenPath, "CHATDB_AIDGP_TOKEN_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.TrafficQueryPath, "CHATDB_AIDGP_TRAFFIC_QUERY_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.PopulationQueryPath, "CHATDB_AIDGP_POPULATION_QUERY_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.GridQueryPath, "CHATDB_AIDGP_GRID_QUERY_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.KnowledgeBasesPath, "CHATDB_AIDGP_KNOWLEDGE_BASES_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.DocumentsPath, "CHATDB_AIDGP_DOCUMENTS_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.DocumentSegmentsPath, "CHATDB_AIDGP_DOCUMENT_SEGMENTS_PATH")
	setStringFromEnv(&cfg.QaConfig.Sync.Aidgp.KnowledgePermissionsPath, "CHATDB_AIDGP_KNOWLEDGE_PERMISSIONS_PATH")
	setStringFromEnv(&cfg.Uiap.BaseUrl, "CHATDB_UIAP_BASE_URL")
	setStringFromEnv(&cfg.Uiap.ClientId, "CHATDB_UIAP_CLIENT_ID")
	setStringFromEnv(&cfg.Uiap.ClientSecret, "CHATDB_UIAP_CLIENT_SECRET")
	setStringFromEnv(&cfg.Uiap.TokenPath, "CHATDB_UIAP_TOKEN_PATH")
	setStringFromEnv(&cfg.Uiap.UserInfoPath, "CHATDB_UIAP_USER_INFO_PATH")
	setStringFromEnv(&cfg.Uiap.PermissionPath, "CHATDB_UIAP_PERMISSION_PATH")
	setStringFromEnv(&cfg.Uiap.BatchPermissionPath, "CHATDB_UIAP_BATCH_PERMISSION_PATH")
	setStringFromEnv(&cfg.Traffic.Mqtt.Broker, "CHATDB_TRAFFIC_MQTT_BROKER")
	setStringFromEnv(&cfg.Traffic.Mqtt.Topic, "CHATDB_TRAFFIC_MQTT_TOPIC")
	setStringFromEnv(&cfg.Traffic.Mqtt.ClientIdPrefix, "CHATDB_TRAFFIC_MQTT_CLIENT_ID_PREFIX")
	setStringFromEnv(&cfg.Traffic.Mqtt.Username, "CHATDB_TRAFFIC_MQTT_USERNAME")
	setStringFromEnv(&cfg.Traffic.Mqtt.Password, "CHATDB_TRAFFIC_MQTT_PASSWORD")

	if enabled := os.Getenv("CHATDB_QA_WEB_SEARCH_ENABLED"); enabled != "" {
		if v, err := strconv.ParseBool(enabled); err == nil {
			cfg.QaConfig.WebSearch.Enabled = v
		}
	}

	if readonly := os.Getenv("CHATDB_DB_READONLY"); readonly != "" {
		if v, err := strconv.ParseBool(readonly); err == nil {
			cfg.DbConfig.Readonly = v
		}
	}
	if enabled := os.Getenv("CHATDB_UIAP_ENABLED"); enabled != "" {
		if v, err := strconv.ParseBool(enabled); err == nil {
			cfg.Uiap.Enabled = v
		}
	}
	if seconds := os.Getenv("CHATDB_UIAP_TIMEOUT_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			cfg.Uiap.TimeoutSeconds = v
		}
	}
	if seconds := os.Getenv("CHATDB_UIAP_TOKEN_EXPIRE_SKEW_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			cfg.Uiap.TokenExpireSkewSeconds = v
		}
	}
	if seconds := os.Getenv("CHATDB_UIAP_PERMISSION_POLL_INTERVAL_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			cfg.Uiap.PermissionPollIntervalSeconds = v
		}
	}
	if size := os.Getenv("CHATDB_UIAP_PERMISSION_POLL_PAGE_SIZE"); size != "" {
		if v, err := strconv.Atoi(size); err == nil {
			cfg.Uiap.PermissionPollPageSize = v
		}
	}
	if seconds := os.Getenv("CHATDB_AIDGP_TIMEOUT_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			cfg.QaConfig.Sync.Aidgp.TimeoutSeconds = v
		}
	}
	if retries := os.Getenv("CHATDB_AIDGP_RETRY_TIMES"); retries != "" {
		if v, err := strconv.Atoi(retries); err == nil {
			cfg.QaConfig.Sync.Aidgp.RetryTimes = v
		}
	}
	if seconds := os.Getenv("CHATDB_AIDGP_TOKEN_EXPIRE_SKEW_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			cfg.QaConfig.Sync.Aidgp.TokenExpireSkewSeconds = v
		}
	}
	if enabled := os.Getenv("CHATDB_TRAFFIC_MQTT_ENABLED"); enabled != "" {
		if v, err := strconv.ParseBool(enabled); err == nil {
			cfg.Traffic.Mqtt.Enabled = v
		}
	}
	if qos := os.Getenv("CHATDB_TRAFFIC_MQTT_QOS"); qos != "" {
		if v, err := strconv.Atoi(qos); err == nil {
			cfg.Traffic.Mqtt.Qos = v
		}
	}
	if days := os.Getenv("CHATDB_TRAFFIC_DEDUPE_WINDOW_DAYS"); days != "" {
		if v, err := strconv.Atoi(days); err == nil {
			cfg.Traffic.Ingest.DedupeWindowDays = v
		}
	}
	if days := os.Getenv("CHATDB_TRAFFIC_RAW_PAYLOAD_RETAIN_DAYS"); days != "" {
		if v, err := strconv.Atoi(days); err == nil {
			cfg.Traffic.Ingest.RawPayloadRetainDays = v
		}
	}
	if minutes := os.Getenv("CHATDB_TRAFFIC_NO_DATA_WARN_MINUTES"); minutes != "" {
		if v, err := strconv.Atoi(minutes); err == nil {
			cfg.Traffic.Ingest.NoDataWarnMinutes = v
		}
	}

	if seconds := os.Getenv("CHATDB_SYNC_INTERVAL_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil && v > 0 {
			cfg.Sync.IntervalSeconds = v
		}
	}

	jwtSecret := os.Getenv("CHATDB_JWT_SECRET")
	jwtExpire := os.Getenv("CHATDB_JWT_EXPIRE_HOURS")
	jwtIssuer := os.Getenv("CHATDB_JWT_ISSUER")
	if jwtSecret != "" || jwtExpire != "" || jwtIssuer != "" {
		ensureJwtOption(cfg)
		for _, option := range cfg.Jwt {
			if option == nil || option.Subject != JwtSubjectUser {
				continue
			}
			if jwtSecret != "" {
				option.Secret = jwtSecret
			}
			if jwtExpire != "" {
				if v, err := strconv.Atoi(jwtExpire); err == nil && v > 0 {
					option.Expire = v
				}
			}
			if jwtIssuer != "" {
				option.Issuer = jwtIssuer
			}
		}
	}

	adminJwtSecret := os.Getenv("CHATDB_ADMIN_JWT_SECRET")
	if adminJwtSecret != "" {
		ensureJwtOption(cfg)
		for _, option := range cfg.Jwt {
			if option == nil || option.Subject != JwtSubjectAdmin {
				continue
			}
			option.Secret = adminJwtSecret
		}
	}

	for _, opt := range cfg.Jwt {
		if opt == nil {
			continue
		}
		if opt.Secret == "" {
			panic("JWT secret for subject " + opt.Subject + " must not be empty; set CHATDB_JWT_SECRET / CHATDB_ADMIN_JWT_SECRET or configure in config.yaml")
		}
	}
}

func ensureJwtOption(cfg *model.ConfigData) {
	hasUser, hasAdmin := false, false
	for _, option := range cfg.Jwt {
		if option == nil {
			continue
		}
		if option.Subject == JwtSubjectUser {
			hasUser = true
		}
		if option.Subject == JwtSubjectAdmin {
			hasAdmin = true
		}
	}
	if !hasUser {
		cfg.Jwt = append(cfg.Jwt, &model.JwtOption{
			Subject: JwtSubjectUser,
			Expire:  24,
			Issuer:  "chatdb",
		})
	}
	if !hasAdmin {
		cfg.Jwt = append(cfg.Jwt, &model.JwtOption{
			Subject: JwtSubjectAdmin,
			Expire:  8,
			Issuer:  "chatdb",
		})
	}
}

func setStringFromEnv(target *string, key string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}
