package ai

import (
	"ai-chat-sql/internal/consts"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mcpTool "github.com/cloudwego/eino-ext/components/tool/mcp"
	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	gMcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/gogf/gf/v2/os/gtime"
)

const mcpToolCacheTTL = 30 * time.Minute

var cachedMCPTools = struct {
	mu        sync.RWMutex
	expiresAt time.Time
	infos     []*schema.ToolInfo
}{}

type cachedMCPTool struct {
	info   *schema.ToolInfo
	handle func(ctx context.Context, name string, result *gMcp.CallToolResult) (*gMcp.CallToolResult, error)
}

func (t *cachedMCPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *cachedMCPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	// Validate arguments JSON before passing to MCP client
	if !json.Valid([]byte(argumentsInJSON)) {
		consts.Logger.Errorf(ctx, "MCP tool %s received invalid JSON arguments: %q", t.info.Name, argumentsInJSON)
		return "", fmt.Errorf("invalid JSON arguments for tool %s: %s", t.info.Name, argumentsInJSON)
	}
	result, err := consts.McpClient.CallTool(ctx, gMcp.CallToolRequest{
		Request: gMcp.Request{
			Method: "tools/call",
		},
		Params: gMcp.CallToolParams{
			Name:      t.info.Name,
			Arguments: json.RawMessage(argumentsInJSON),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to call mcp tool: %w", err)
	}
	if t.handle != nil {
		result, err = t.handle(ctx, t.info.Name, result)
		if err != nil {
			return "", fmt.Errorf("failed to execute mcp tool call result handler: %w", err)
		}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal mcp tool result: %w", err)
	}
	if result.IsError {
		return "", fmt.Errorf("failed to call mcp tool, mcp server return error: %s", string(data))
	}
	return string(data), nil
}

func getCachedMCPTools(ctx context.Context, handler func(ctx context.Context, name string, result *gMcp.CallToolResult) (*gMcp.CallToolResult, error)) ([]einoTool.BaseTool, error) {
	now := gtime.Now().Time
	cachedMCPTools.mu.RLock()
	if len(cachedMCPTools.infos) > 0 && now.Before(cachedMCPTools.expiresAt) {
		infos := append([]*schema.ToolInfo(nil), cachedMCPTools.infos...)
		cachedMCPTools.mu.RUnlock()
		return wrapMCPToolInfos(infos, handler), nil
	}
	cachedMCPTools.mu.RUnlock()

	cachedMCPTools.mu.Lock()
	defer cachedMCPTools.mu.Unlock()
	if len(cachedMCPTools.infos) > 0 && now.Before(cachedMCPTools.expiresAt) {
		return wrapMCPToolInfos(cachedMCPTools.infos, handler), nil
	}

	tools, err := mcpTool.GetTools(ctx, &mcpTool.Config{Cli: consts.McpClient})
	if err != nil {
		return nil, err
	}
	infos := make([]*schema.ToolInfo, 0, len(tools))
	for _, toolItem := range tools {
		info, infoErr := toolItem.Info(ctx)
		if infoErr != nil {
			return nil, infoErr
		}
		infos = append(infos, info)
	}
	cachedMCPTools.infos = infos
	cachedMCPTools.expiresAt = now.Add(mcpToolCacheTTL)
	return wrapMCPToolInfos(infos, handler), nil
}

func wrapMCPToolInfos(infos []*schema.ToolInfo, handler func(ctx context.Context, name string, result *gMcp.CallToolResult) (*gMcp.CallToolResult, error)) []einoTool.BaseTool {
	tools := make([]einoTool.BaseTool, 0, len(infos))
	for _, info := range infos {
		tools = append(tools, &cachedMCPTool{info: info, handle: handler})
	}
	return tools
}
