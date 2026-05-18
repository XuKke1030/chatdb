package consts

import (
	"ai-chat-sql/internal/model"
	"os"
	"strconv"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	mcpclient "github.com/mark3labs/mcp-go/client"
)

var (
	Ctx          = gctx.New()
	Logger       = g.Log()
	SystemConfig *gjson.Json
	Config       *model.ConfigData
)

var (
	McpClient *mcpclient.Client
)

var (
	PrivateKey string = ""
)

const JwtSubjectUser = "ai-chat-user"

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
	SystemConfig = gjson.New(data)
	if err = SystemConfig.Scan(&Config); err != nil {
		return
	}
	applyEnvOverrides()
	return
}

func applyEnvOverrides() {
	if Config == nil {
		Config = &model.ConfigData{}
	}
	if Config.Server == nil {
		Config.Server = &model.ServerConfig{}
	}
	if Config.AiConfig == nil {
		Config.AiConfig = &model.AiConfig{}
	}
	if Config.AiConfig.OpenAI == nil {
		Config.AiConfig.OpenAI = &model.OpenAIConfig{}
	}
	if Config.AiConfig.DeepSeek == nil {
		Config.AiConfig.DeepSeek = &model.DeepSeekConfig{}
	}
	if Config.AiConfig.Mcp == nil {
		Config.AiConfig.Mcp = &model.McpConfig{}
	}
	if Config.AiConfig.Asr == nil {
		Config.AiConfig.Asr = &model.AsrConfig{}
	}
	if Config.QaConfig == nil {
		Config.QaConfig = &model.QaConfig{}
	}
	if Config.QaConfig.WebSearch == nil {
		Config.QaConfig.WebSearch = &model.QaWebSearchConfig{}
	}
	if Config.QaConfig.Sync == nil {
		Config.QaConfig.Sync = &model.QaSyncConfig{}
	}
	if Config.QaConfig.Sync.Aidgp == nil {
		Config.QaConfig.Sync.Aidgp = &model.QaAidgpConfig{}
	}
	if Config.Uiap == nil {
		Config.Uiap = &model.UiapConfig{}
	}
	if Config.DbConfig == nil {
		Config.DbConfig = &model.DbConfig{}
	}
	if Config.Traffic == nil {
		Config.Traffic = &model.TrafficConfig{}
	}
	if Config.Traffic.Mqtt == nil {
		Config.Traffic.Mqtt = &model.TrafficMqttConfig{}
	}
	if Config.Traffic.Ingest == nil {
		Config.Traffic.Ingest = &model.TrafficIngestConfig{}
	}
	if Config.Redis == nil {
		Config.Redis = &model.RedisConfig{}
	}

	setStringFromEnv(&Config.Server.Address, "CHATDB_SERVER_ADDRESS")
	setStringFromEnv(&Config.Server.Mode, "CHATDB_SERVER_MODE")
	setStringFromEnv(&Config.AiConfig.OpenAI.BaseUrl, "CHATDB_OPENAI_BASE_URL")
	setStringFromEnv(&Config.AiConfig.OpenAI.Key, "CHATDB_OPENAI_KEY")
	setStringFromEnv(&Config.AiConfig.DeepSeek.BaseUrl, "CHATDB_DEEPSEEK_BASE_URL")
	setStringFromEnv(&Config.AiConfig.DeepSeek.Key, "CHATDB_DEEPSEEK_KEY")
	setStringFromEnv(&Config.AiConfig.Mcp.Address, "CHATDB_MCP_ADDRESS")
	setStringFromEnv(&Config.AiConfig.Asr.BaseUrl, "CHATDB_ASR_BASE_URL")
	setStringFromEnv(&Config.AiConfig.Asr.Key, "CHATDB_ASR_KEY")
	setStringFromEnv(&Config.AiConfig.Asr.Model, "CHATDB_ASR_MODEL")
	setStringFromEnv(&Config.QaConfig.WebSearch.Provider, "CHATDB_QA_WEB_SEARCH_PROVIDER")
	setStringFromEnv(&Config.QaConfig.WebSearch.BaseUrl, "CHATDB_QA_WEB_SEARCH_BASE_URL")
	setStringFromEnv(&Config.QaConfig.WebSearch.Key, "CHATDB_QA_WEB_SEARCH_KEY")
	setStringFromEnv(&Config.QaConfig.Sync.Provider, "CHATDB_QA_SYNC_PROVIDER")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.BaseUrl, "CHATDB_QA_AIDGP_BASE_URL")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.AppKey, "CHATDB_QA_AIDGP_APP_KEY")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.AppSecret, "CHATDB_QA_AIDGP_APP_SECRET")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.TokenPath, "CHATDB_AIDGP_TOKEN_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.TrafficQueryPath, "CHATDB_AIDGP_TRAFFIC_QUERY_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.PopulationQueryPath, "CHATDB_AIDGP_POPULATION_QUERY_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.GridQueryPath, "CHATDB_AIDGP_GRID_QUERY_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.KnowledgeBasesPath, "CHATDB_AIDGP_KNOWLEDGE_BASES_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.DocumentsPath, "CHATDB_AIDGP_DOCUMENTS_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.DocumentSegmentsPath, "CHATDB_AIDGP_DOCUMENT_SEGMENTS_PATH")
	setStringFromEnv(&Config.QaConfig.Sync.Aidgp.KnowledgePermissionsPath, "CHATDB_AIDGP_KNOWLEDGE_PERMISSIONS_PATH")
	setStringFromEnv(&Config.Uiap.BaseUrl, "CHATDB_UIAP_BASE_URL")
	setStringFromEnv(&Config.Uiap.ClientId, "CHATDB_UIAP_CLIENT_ID")
	setStringFromEnv(&Config.Uiap.ClientSecret, "CHATDB_UIAP_CLIENT_SECRET")
	setStringFromEnv(&Config.Uiap.TokenPath, "CHATDB_UIAP_TOKEN_PATH")
	setStringFromEnv(&Config.Uiap.UserInfoPath, "CHATDB_UIAP_USER_INFO_PATH")
	setStringFromEnv(&Config.Uiap.PermissionPath, "CHATDB_UIAP_PERMISSION_PATH")
	setStringFromEnv(&Config.Uiap.BatchPermissionPath, "CHATDB_UIAP_BATCH_PERMISSION_PATH")
	setStringFromEnv(&Config.Traffic.Mqtt.Broker, "CHATDB_TRAFFIC_MQTT_BROKER")
	setStringFromEnv(&Config.Traffic.Mqtt.Topic, "CHATDB_TRAFFIC_MQTT_TOPIC")
	setStringFromEnv(&Config.Traffic.Mqtt.ClientIdPrefix, "CHATDB_TRAFFIC_MQTT_CLIENT_ID_PREFIX")
	setStringFromEnv(&Config.Traffic.Mqtt.Username, "CHATDB_TRAFFIC_MQTT_USERNAME")
	setStringFromEnv(&Config.Traffic.Mqtt.Password, "CHATDB_TRAFFIC_MQTT_PASSWORD")
	setStringFromEnv(&Config.Redis.Address, "CHATDB_REDIS_ADDRESS")
	setStringFromEnv(&Config.Redis.Password, "CHATDB_REDIS_PASSWORD")

	if enabled := os.Getenv("CHATDB_QA_WEB_SEARCH_ENABLED"); enabled != "" {
		if v, err := strconv.ParseBool(enabled); err == nil {
			Config.QaConfig.WebSearch.Enabled = v
		}
	}

	if readonly := os.Getenv("CHATDB_DB_READONLY"); readonly != "" {
		if v, err := strconv.ParseBool(readonly); err == nil {
			Config.DbConfig.Readonly = v
		}
	}
	if enabled := os.Getenv("CHATDB_UIAP_ENABLED"); enabled != "" {
		if v, err := strconv.ParseBool(enabled); err == nil {
			Config.Uiap.Enabled = v
		}
	}
	if seconds := os.Getenv("CHATDB_UIAP_TIMEOUT_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			Config.Uiap.TimeoutSeconds = v
		}
	}
	if seconds := os.Getenv("CHATDB_UIAP_TOKEN_EXPIRE_SKEW_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			Config.Uiap.TokenExpireSkewSeconds = v
		}
	}
	if seconds := os.Getenv("CHATDB_UIAP_PERMISSION_POLL_INTERVAL_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			Config.Uiap.PermissionPollIntervalSeconds = v
		}
	}
	if size := os.Getenv("CHATDB_UIAP_PERMISSION_POLL_PAGE_SIZE"); size != "" {
		if v, err := strconv.Atoi(size); err == nil {
			Config.Uiap.PermissionPollPageSize = v
		}
	}
	if seconds := os.Getenv("CHATDB_AIDGP_TIMEOUT_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			Config.QaConfig.Sync.Aidgp.TimeoutSeconds = v
		}
	}
	if retries := os.Getenv("CHATDB_AIDGP_RETRY_TIMES"); retries != "" {
		if v, err := strconv.Atoi(retries); err == nil {
			Config.QaConfig.Sync.Aidgp.RetryTimes = v
		}
	}
	if seconds := os.Getenv("CHATDB_AIDGP_TOKEN_EXPIRE_SKEW_SECONDS"); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			Config.QaConfig.Sync.Aidgp.TokenExpireSkewSeconds = v
		}
	}
	if enabled := os.Getenv("CHATDB_TRAFFIC_MQTT_ENABLED"); enabled != "" {
		if v, err := strconv.ParseBool(enabled); err == nil {
			Config.Traffic.Mqtt.Enabled = v
		}
	}
	if qos := os.Getenv("CHATDB_TRAFFIC_MQTT_QOS"); qos != "" {
		if v, err := strconv.Atoi(qos); err == nil {
			Config.Traffic.Mqtt.Qos = v
		}
	}
	if days := os.Getenv("CHATDB_TRAFFIC_DEDUPE_WINDOW_DAYS"); days != "" {
		if v, err := strconv.Atoi(days); err == nil {
			Config.Traffic.Ingest.DedupeWindowDays = v
		}
	}
	if days := os.Getenv("CHATDB_TRAFFIC_RAW_PAYLOAD_RETAIN_DAYS"); days != "" {
		if v, err := strconv.Atoi(days); err == nil {
			Config.Traffic.Ingest.RawPayloadRetainDays = v
		}
	}
	if minutes := os.Getenv("CHATDB_TRAFFIC_NO_DATA_WARN_MINUTES"); minutes != "" {
		if v, err := strconv.Atoi(minutes); err == nil {
			Config.Traffic.Ingest.NoDataWarnMinutes = v
		}
	}
	if db := os.Getenv("CHATDB_REDIS_DB"); db != "" {
		if v, err := strconv.Atoi(db); err == nil {
			Config.Redis.Db = v
		}
	}

	jwtSecret := os.Getenv("CHATDB_JWT_SECRET")
	jwtExpire := os.Getenv("CHATDB_JWT_EXPIRE_HOURS")
	jwtIssuer := os.Getenv("CHATDB_JWT_ISSUER")
	if jwtSecret != "" || jwtExpire != "" || jwtIssuer != "" {
		ensureJwtOption()
		for _, option := range Config.Jwt {
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
}

func ensureJwtOption() {
	for _, option := range Config.Jwt {
		if option != nil && option.Subject == JwtSubjectUser {
			return
		}
	}
	Config.Jwt = append(Config.Jwt, &model.JwtOption{
		Subject: JwtSubjectUser,
		Expire:  24,
		Issuer:  "chatdb",
	})
}

func setStringFromEnv(target *string, key string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}
