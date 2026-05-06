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
	if Config.DbConfig == nil {
		Config.DbConfig = &model.DbConfig{}
	}

	setStringFromEnv(&Config.Server.Address, "CHATDB_SERVER_ADDRESS")
	setStringFromEnv(&Config.Server.Mode, "CHATDB_SERVER_MODE")
	setStringFromEnv(&Config.AiConfig.OpenAI.BaseUrl, "CHATDB_OPENAI_BASE_URL")
	setStringFromEnv(&Config.AiConfig.OpenAI.Key, "CHATDB_OPENAI_KEY")
	setStringFromEnv(&Config.AiConfig.DeepSeek.BaseUrl, "CHATDB_DEEPSEEK_BASE_URL")
	setStringFromEnv(&Config.AiConfig.DeepSeek.Key, "CHATDB_DEEPSEEK_KEY")
	setStringFromEnv(&Config.AiConfig.Mcp.Address, "CHATDB_MCP_ADDRESS")

	if readonly := os.Getenv("CHATDB_DB_READONLY"); readonly != "" {
		if v, err := strconv.ParseBool(readonly); err == nil {
			Config.DbConfig.Readonly = v
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
