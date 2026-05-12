package ai

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/dao"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"io"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	gMcp "github.com/mark3labs/mcp-go/mcp"
)

type sAiChat struct{}

func init() {
	service.RegisterAiChat(NewAiChat())
}

func NewAiChat() *sAiChat {
	return &sAiChat{}
}

// Chat 聊天
func (s *sAiChat) Chat(ctx context.Context, in model.ChatInput, respChan chan any) {
	var err error
	totalStart := time.Now()
	stageStart := totalStart
	logStage := func(stage string) {
		consts.Logger.Infof(ctx, "perf ask_number_ai stage=%s topic=%s databaseId=%d sessionId=%s costMs=%d totalMs=%d", stage, in.Topic, in.DatabaseId, in.SessionId, time.Since(stageStart).Milliseconds(), time.Since(totalStart).Milliseconds())
		stageStart = time.Now()
	}
	defer func() {
		consts.Logger.Infof(ctx, "perf ask_number_ai stage=total topic=%s databaseId=%d sessionId=%s costMs=%d", in.Topic, in.DatabaseId, in.SessionId, time.Since(totalStart).Milliseconds())
	}()
	defer func() {
		if err != nil {
			respChan <- err
			close(respChan)
		}
	}()
	// 发送开始包
	if err = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "start",
		Data: g.Map{
			"sessionId": in.SessionId,
		},
	}, respChan); err != nil {
		return
	}
	logStage("send_start")
	// 创建响应通道
	HeartbeatCtx, cancel := context.WithCancel(ctx)
	s.AiChatHeartbeat(HeartbeatCtx, respChan)

	llm, err := service.AI().GetChatModel(in.Ai, in.Model)
	if err != nil {
		cancel()
		return
	}
	logStage("model")
	// 获取MCP工具定义（带缓存），每次请求重新包装 handler，避免复用 respChan 闭包。
	mcpTools, err := getCachedMCPTools(ctx, func(ctx context.Context, name string, result *gMcp.CallToolResult) (out *gMcp.CallToolResult, err error) {
		dataMap := g.Map{
			"name":   name,
			"output": result.Content[0],
		}
		model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event: "tool_call",
			Data:  dataMap,
		}, respChan)
		out = result
		return
	})
	if err != nil {
		cancel()
		return
	}
	logStage("mcp_tools")
	// 创建React智能体
	aiAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: llm,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: mcpTools},
		MaxStep:          8,
		// 自定义 StreamToolCallChecker：DeepSeek 等模型会先输出文本再输出 tool calls
		// 默认实现只检查第一个 chunk，会导致 tool calls 被忽略
		StreamToolCallChecker: func(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
			defer sr.Close()
			for {
				msg, err := sr.Recv()
				if err == io.EOF {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				if len(msg.ToolCalls) > 0 {
					return true, nil
				}
			}
		},
	})
	if err != nil {
		cancel()
		return
	}
	logStage("agent")
	// 获取需要操作的数据库信息
	dbTypeT, err := dao.DatabaseConf.Ctx(ctx).Cache(gdb.CacheOption{
		Duration: 30 * time.Minute,
		Name:     "db_type:" + gconv.String(in.DatabaseId),
	}).Where("database_id = ?", in.DatabaseId).Value("db_type")
	if err != nil {
		cancel()
		return
	}
	logStage("database_type")

	prompt, err := service.Prompt().GetPrompt(ctx, consts.PromptMain)
	if err != nil {
		cancel()
		return
	}
	if g.IsEmpty(in.Prompt) {
		in.Prompt = "-"
	}

	// 构建消息列表
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: prompt.GetContent(in.DatabaseId, dbTypeT.String()),
		},
		{
			Role:    schema.System,
			Content: in.Prompt,
		},
	}

	// 如果指定了主题，加载对应的主题 prompt 作为补充系统提示
	if in.Topic != "" {
		topicPrompt, promptErr := service.Prompt().GetPrompt(ctx, in.Topic)
		if promptErr == nil && topicPrompt != nil {
			messages = append(messages, &schema.Message{
				Role:    schema.System,
				Content: topicPrompt.Content,
			})
		}
	}
	logStage("prompt")

	for _, item := range in.History {
		if item.Content == "" {
			continue
		}
		role := schema.User
		if item.Role == "assistant" {
			role = schema.Assistant
		}
		messages = append(messages, &schema.Message{
			Role:    role,
			Content: item.Content,
		})
	}

	// 添加用户消息
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: in.Message,
	})

	out, err := aiAgent.Stream(ctx, messages)
	if err != nil {
		cancel()
		return
	}
	logStage("agent_stream")

	// AI输出流
	s.AiChatStreamOut(ctx, respChan, out, cancel)
}
