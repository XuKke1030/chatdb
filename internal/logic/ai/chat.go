package ai

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/dao"
	"ai-chat-sql/internal/logic/mcp"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"ai-chat-sql/utility"
	"context"
	"fmt"
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
	totalStart := time.Now()
	stageStart := totalStart
	logStage := func(stage string) {
		consts.Logger.Infof(ctx, "perf ask_number_ai stage=%s topic=%s databaseId=%d sessionId=%s costMs=%d totalMs=%d", stage, in.Topic, in.DatabaseId, in.SessionId, time.Since(stageStart).Milliseconds(), time.Since(totalStart).Milliseconds())
		stageStart = time.Now()
	}

	logStage("start")
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "start",
		Data: g.Map{
			"sessionId": in.SessionId,
		},
	}, respChan)

	logStage("send_start")

	// 表名累加器，在MCP工具handler闭包中收集
	var tablesSlice []string
	tablesPtr := &tablesSlice

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
		consts.Logger.Infof(ctx, "perf mcp_handler ENTER name=%s contentLen=%d", name, len(result.Content))
		dataMap := g.Map{
			"name":   name,
			"output": result.Content[0],
		}
		model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event: "tool_call",
			Data:  dataMap,
		}, respChan)
		// 从SQL工具返回结果中提取表名
		if name == "SQL_Actuator" {
			var resultText string
			if text, ok := result.Content[0].(gMcp.TextContent); ok {
				resultText = text.Text
			} else {
				resultText = fmt.Sprintf("%v", result.Content[0])
			}
			for _, t := range mcp.ExtractTableNames(resultText) {
				mcp.AddTable(tablesPtr, t)
				consts.Logger.Infof(ctx, "perf mcp_handler added_table=%s total=%d", t, len(*tablesPtr))
			}
		}
		out = result
		return
	})
	if err != nil {
		cancel()
		return
	}
	logStage("mcp_tools")

	aiAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: llm,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: mcpTools},
		MaxStep:          35,
	})
	if err != nil {
		cancel()
		return
	}

	// 获取数据库类型
	dbTypeT, dbTypeErr := dao.DatabaseConf.Ctx(ctx).Cache(gdb.CacheOption{
		Duration: 30 * time.Minute,
		Name:     "db_type:" + gconv.String(in.DatabaseId),
	}).Where("database_id = ?", in.DatabaseId).Value("db_type")
	if dbTypeErr != nil {
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
		cancel()
		if timeoutCtx.Err() == context.DeadlineExceeded {
			_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
				Event: "error",
				Data:  g.Map{"message": "查询超时，请尝试简化问题或换一种问法"},
			}, respChan)
			_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
		}
		return
	}
	logStage("agent_generate")

	// 发送表名
	if len(*tablesPtr) > 0 {
		tablesStr := strings.Join(*tablesPtr, ",")
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
	cancel()
}
