package ai

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/service"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/gogf/gf/v2/net/gclient"
)

type sAI struct {
	mu         sync.RWMutex
	modelCache map[string]einoModel.ToolCallingChatModel
}

func init() {
	service.RegisterAI(NewAI())
}

func NewAI() *sAI {
	return &sAI{
		modelCache: make(map[string]einoModel.ToolCallingChatModel),
	}
}

// GetChatModel 获取聊天模型（带缓存）
func (s *sAI) GetChatModel(ai, model string) (chatModel einoModel.ToolCallingChatModel, err error) {
	if model == "" {
		switch ai {
		case "openai":
			model = "gpt-4o-mini"
		case "deepseek":
			model = "deepseek-chat"
		}
	}
	key := ai + ":" + model

	s.mu.RLock()
	if cached, ok := s.modelCache[key]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	chatModel, err = s.createChatModel(ai, model)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.modelCache[key] = chatModel
	s.mu.Unlock()
	return
}

func (s *sAI) createChatModel(ai, model string) (chatModel einoModel.ToolCallingChatModel, err error) {
	switch ai {
	case "openai":
		if err = validateAIProviderConfig("openai", consts.GetConfig().AiConfig.OpenAI.BaseUrl, consts.GetConfig().AiConfig.OpenAI.Key); err != nil {
			return
		}
		chatModel, err = openai.NewChatModel(consts.Ctx, &openai.ChatModelConfig{
			BaseURL: consts.GetConfig().AiConfig.OpenAI.BaseUrl,
			Model:   model,
			APIKey:  consts.GetConfig().AiConfig.OpenAI.Key,
		})
		return
	case "deepseek":
		if err = validateAIProviderConfig("deepseek", consts.GetConfig().AiConfig.DeepSeek.BaseUrl, consts.GetConfig().AiConfig.DeepSeek.Key); err != nil {
			return
		}
		chatModel, err = deepseek.NewChatModel(consts.Ctx, &deepseek.ChatModelConfig{
			BaseURL: consts.GetConfig().AiConfig.DeepSeek.BaseUrl,
			Model:   model,
			APIKey:  consts.GetConfig().AiConfig.DeepSeek.Key,
		})
		return
	}
	err = fmt.Errorf("不支持的 AI 类型: %s", ai)
	return
}

// GetChatModeListJson 获取聊天模型列表
func (s *sAI) GetChatModeListJson(ctx context.Context, ai string) (json string, err error) {
	var (
		url string
		key string
	)
	switch ai {
	case "openai":
		url = consts.GetConfig().AiConfig.OpenAI.BaseUrl
		key = consts.GetConfig().AiConfig.OpenAI.Key
	case "deepseek":
		url = consts.GetConfig().AiConfig.DeepSeek.BaseUrl
		key = consts.GetConfig().AiConfig.DeepSeek.Key
	default:
		err = errors.New("未知的操作")
		return
	}
	if err = validateAIProviderConfig(ai, url, key); err != nil {
		return
	}
	resp, err := gclient.New().Discovery(nil).SetHeader("authorization", "Bearer "+key).Get(ctx, fmt.Sprintf("%s/models", url))
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = errors.New("获取模型列表失败")
		return
	}
	json = resp.ReadAllString()
	return
}

func validateAIProviderConfig(provider string, baseURL string, key string) error {
	if strings.TrimSpace(baseURL) == "" {
		return fmt.Errorf("%s baseUrl 未配置", provider)
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("%s API key 未配置，请设置环境变量或配置文件", provider)
	}
	if strings.Contains(strings.ToUpper(key), "CHANGE_ME") {
		return fmt.Errorf("%s API key 仍是占位值，请替换为真实密钥", provider)
	}
	return nil
}
