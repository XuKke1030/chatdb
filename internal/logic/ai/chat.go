package ai

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/dao"
	"ai-chat-sql/internal/logic/mcp"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"ai-chat-sql/utility"
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	gMcp "github.com/mark3labs/mcp-go/mcp"
)

type sAiChat struct{}

func init() {
	service.RegisterAiChat(&sAiChat{})
}

func (s *sAiChat) Chat(ctx context.Context, in model.ChatInput, respChan chan any) {
	ctx = mcp.ContextWithSessionID(ctx, in.SessionId)
	ctx, _ = mcp.WithTablesAccumulator(ctx)
	totalStart := time.Now()
	stageStart := totalStart
	logStage := func(stage string) {
		consts.Logger.Infof(ctx, "perf ask_number_ai stage=%s topic=%s databaseId=%d sessionId=%s costMs=%d totalMs=%d", stage, in.Topic, in.DatabaseId, in.SessionId, time.Since(stageStart).Milliseconds(), time.Since(totalStart).Milliseconds())
		stageStart = time.Now()
	}

	// 统一保障：任何退出路径都会关闭 respChan 并 cancel heartbeat
	closer := newChanCloser(respChan)
	defer closer.Close()

	heartbeatCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	logStage("start")
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "start",
		Data: g.Map{
			"sessionId": in.SessionId,
		},
	}, respChan)

	logStage("send_start")

	s.AiChatHeartbeat(heartbeatCtx, respChan)

	llm, err := service.AI().GetChatModel(in.Ai, in.Model)
	if err != nil {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		return
	}
	logStage("model")

	// 获取MCP工具定义（带缓存），每次请求重新包装 handler，避免复用 respChan 闭包。
	mcpTools, err := getCachedMCPTools(ctx, func(ctx context.Context, name string, result *gMcp.CallToolResult) (out *gMcp.CallToolResult, err error) {
		consts.Logger.Infof(ctx, "perf mcp_handler ENTER name=%s contentLen=%d", name, len(result.Content))
		dataMap := g.Map{
			"name":   name,
			"output": result.Content[0],
		}
		model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event: "tool_call",
			Data:  dataMap,
		}, respChan)
		// MCP server 的 tool handler 收到的是独立 context，无法传播我们设置的 context value，
		// 所以在 Eino agent 的回调（仍持有原始 context）中从结果提取表名并写入累加器。
		if name == "SQL_Actuator" {
			var resultText string
			if text, ok := result.Content[0].(gMcp.TextContent); ok {
				resultText = text.Text
			} else {
				resultText = gconv.String(result.Content[0])
			}
			for _, t := range mcp.ExtractTableNames(resultText) {
				mcp.AddTableToContext(ctx, t)
			}
		}
		out = result
		return
	})
	if err != nil {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		return
	}
	logStage("mcp_tools")

	aiAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: llm,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: mcpTools},
		MaxStep:          35,
	})
	if err != nil {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		return
	}

	// 获取数据库类型
	dbTypeT, dbTypeErr := dao.DatabaseConf.Ctx(ctx).Cache(gdb.CacheOption{
		Duration: 30 * time.Minute,
		Name:     "db_type:" + gconv.String(in.DatabaseId),
	}).Where("database_id = ?", in.DatabaseId).Value("db_type")
	if dbTypeErr != nil {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		return
	}
	logStage("database_type")

	prompt, err := service.Prompt().GetPrompt(ctx, consts.PromptMain)
	if err != nil {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		return
	}
	if g.IsEmpty(in.Prompt) {
		in.Prompt = "-"
	}

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: prompt.GetContent(gtime.Now().Format("Y-m-d"), in.DatabaseId, dbTypeT.String()),
		},
		{
			Role:    schema.System,
			Content: in.Prompt,
		},
	}

	// 注入主题提示词
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

	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: in.Message,
	})

	// 整体超时保护
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 60*time.Second)
	defer timeoutCancel()

	msg, err := aiAgent.Generate(timeoutCtx, messages)
	if err != nil {
		consts.Logger.Errorf(ctx, "aiAgent.Generate error: %v", err)
		if timeoutCtx.Err() == context.DeadlineExceeded {
			_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
				Event: "error",
				Data:  g.Map{"message": "查询超时，请尝试简化问题或换一种问法"},
			}, respChan)
		} else {
			_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
				Event: "error",
				Data:  g.Map{"message": utility.SafeUserErr(err)},
			}, respChan)
		}
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		return
	}
	logStage("agent_generate")

	// 发送表名
	if t := mcp.TablesFromContext(ctx); t != nil && len(*t) > 0 {
		tablesStr := strings.Join(*t, ",")
		consts.Logger.Infof(ctx, "perf ask_number_sse send_tables=%s", tablesStr)
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event:   "tables",
			Content: tablesStr,
		}, respChan)
	}

	// 发送完整回复
	content := utility.SanitizeOutput(msg.Content)
	if content != "" {
		consts.Logger.Infof(ctx, "perf ask_number_sse send_message len=%d", len(content))
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event:   "message",
			Content: content,
			Role:    "assistant",
		}, respChan)
	}

	consts.Logger.Infof(ctx, "perf ask_number_sse send_end")
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
}
