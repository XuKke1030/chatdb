package mcp

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type sMcpHandler struct {
}

func NewMcpHandler() *sMcpHandler {
	return &sMcpHandler{}
}

func init() {
	service.RegisterMcpHandler(NewMcpHandler())
	service.RegisterMcpTool(NewMcpTool())
}

func (s *sMcpHandler) GetList() []model.McpReg {
	return []model.McpReg{
		{
			Name:        "RunSafeShellCommand",
			Description: "Execute a read-only terminal command safely with allowlist and timeout; no pipes/redirects/chaining",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("command",
					mcp.Required(),
					mcp.Description("The terminal command to execute (single command only, no pipes or redirects)"),
				),
				mcp.WithString("timeoutSeconds",
					mcp.Description("Timeout seconds (default 10, max 60)"),
				),
				mcp.WithString("cwd",
					mcp.Description("Optional working directory"),
				),
			},
			Fn: service.McpTool().RunSafeShellCommand,
		},
		{
			Name:        "Md5Encode",
			Description: "Calculate MD5 (hex lower-case) for a given text",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("text",
					mcp.Required(),
					mcp.Description("The text to hash"),
				),
			},
			Fn: service.McpTool().Md5Encode,
		},
		{
			Name:        "Base64Encode",
			Description: "Encode text to Base64",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("text",
					mcp.Required(),
					mcp.Description("Plain text to encode"),
				),
			},
			Fn: service.McpTool().Base64Encode,
		},
		{
			Name:        "Base64Decode",
			Description: "Decode Base64 string to text",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("data",
					mcp.Required(),
					mcp.Description("Base64-encoded data"),
				),
			},
			Fn: service.McpTool().Base64Decode,
		},
		{
			Name:        "JwtParse",
			Description: "Parse a JWT without verifying signature; returns header and payload",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("token",
					mcp.Required(),
					mcp.Description("The JWT token"),
				),
			},
			Fn: service.McpTool().JwtParse,
		},
		{
			Name:        "JsonEncode",
			Description: "Validate and compact a JSON string",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("raw",
					mcp.Required(),
					mcp.Description("Raw JSON string to validate and compact"),
				),
			},
			Fn: service.McpTool().JsonEncode,
		},
		{
			Name:        "SQL_Actuator",
			Description: "Convert the user's requirements into SQL statements, execute the SQL statements, and return the execution results",
			ToolOptions: []mcp.ToolOption{
				mcp.WithNumber(
					"databaseId",
					mcp.Required(),
					mcp.Description("The database ID to be used"),
				),
				mcp.WithString("sql",
					mcp.Required(),
					mcp.Description("The SQL statement to be executed"),
				),
			},
			Fn: service.McpTool().ExecSql,
		},
		{
			Name:        "NowTime",
			Description: "Obtain the current time information，Return the timestamp and date time in the specified time zone",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("timeZone",
					mcp.Required(),
					mcp.Description("The time zone to be used (e.g. 'Asia/Shanghai')"),
				),
			},
			Fn: service.McpTool().GetNowTime,
		},
		{
			Name:        "TimestampToDateTime",
			Description: "Convert a timestamp to a date and time",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("timestamp",
					mcp.Required(),
					mcp.Description("The timestamp to be converted"),
				),
			},
			Fn: service.McpTool().TimestampToDateTime,
		},
		{
			Name:        "GetCalendarDays",
			Description: "Get all days of a specified year and month",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("year",
					mcp.Required(),
					mcp.Description("The year (e.g. 2024)"),
				),
				mcp.WithString("month",
					mcp.Required(),
					mcp.Description("The month (1-12)"),
				),
			},
			Fn: service.McpTool().GetCalendarDays,
		},
		{
			Name:        "GetDatabaseInfo",
			Description: "Get database information including type, name, and connection details",
			ToolOptions: []mcp.ToolOption{
				mcp.WithNumber("databaseId",
					mcp.Description("ChatDB database configuration ID. Prefer this for business databases, e.g. 1"),
				),
				mcp.WithString("dbname",
					mcp.Description("Legacy GoFrame database group name. Numeric strings such as '1' are treated as databaseId."),
				),
			},
			Fn: service.McpTool().GetDatabaseInfo,
		},
		{
			Name:        "ExportToExcel",
			Description: "Export query results or data to an Excel file (.xlsx format) and return the download URL",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("data",
					mcp.Required(),
					mcp.Description("The data to export in JSON format (array of objects)"),
				),
				mcp.WithString("fileName",
					mcp.Description("The name of the exported file (without extension, defaults to timestamp-based name)"),
				),
			},
			Fn: service.McpTool().ExportToExcel,
		},
		{
			Name:        "ExportData",
			Description: "Export data to Excel or JSON format",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("data",
					mcp.Required(),
					mcp.Description("The data to export in JSON format (array of objects)"),
				),
				mcp.WithString("fileName",
					mcp.Description("The name of the exported file (without extension)"),
				),
				mcp.WithString("format",
					mcp.Description("Export format: 'xlsx' (default) or 'json'"),
				),
			},
			Fn: service.McpTool().ExportData,
		},
		{
			Name:        "RecognizeVehiclePlate",
			Description: "Recognize vehicle plate origin, including mainland China province/city, Yue-Z Hong Kong/Macau cross-border plates, and local Hong Kong/Macau plates",
			ToolOptions: []mcp.ToolOption{
				mcp.WithString("plateNumber",
					mcp.Required(),
					mcp.Description("Vehicle plate number, e.g. 粤C12345, 粤Z1234港, 粤Z1234澳, AB1234, MZ-12-34"),
				),
			},
			Fn: service.McpTool().RecognizeVehiclePlate,
		},
	}
}

func (s *sMcpHandler) GetMcpFn(item *model.McpReg) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				consts.Logger.Printf(ctx, "panic error %+v", r)
				result = nil
				err = fmt.Errorf("tool %s panic: %v", item.Name, r)
			}
		}()
		consts.Logger.Printf(ctx, "使用工具 %s 请求内容 %+v", item.Name, request.Params.Arguments)
		if item.Fn == nil {
			return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return mcp.NewToolResultText("处理函数未定义"), nil
			}(ctx, request)
		}
		return item.Fn(ctx, request)
	}
}

type sMcpTool struct{}

func NewMcpTool() *sMcpTool {
	return &sMcpTool{}
}
