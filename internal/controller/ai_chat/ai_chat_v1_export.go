package ai_chat

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	v1 "ai-chat-sql/api/ai_chat/v1"
)

// ExportDownload 处理文件下载（实现 IAiChatV1 接口）
func (c *ControllerV1) ExportDownload(ctx context.Context, req *v1.ExportDownloadReq) (res *v1.ExportDownloadRes, err error) {
	// 验证文件名，防止路径遍历攻击
	fileName := req.File
	if fileName == "" {
		g.Log().Error(ctx, "file parameter is required")
		return nil, fmt.Errorf("file parameter is required")
	}
	if fileName != filepath.Base(fileName) || strings.Contains(fileName, "..") {
		g.Log().Warning(ctx, "invalid export file name: "+fileName)
		return nil, fmt.Errorf("invalid file name")
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != ".xlsx" && ext != ".json" {
		return nil, fmt.Errorf("unsupported export file type")
	}

	// 构建文件路径
	exportDir, err := filepath.Abs("exports")
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(exportDir, fileName)

	// 检查文件是否存在
	if _, err = os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			g.Log().Error(ctx, "file not found: "+filePath)
			return nil, fmt.Errorf("file not found")
		}
		g.Log().Error(ctx, "stat file error: "+err.Error())
		return nil, err
	}

	// 获取 HTTP 响应对象
	r := g.RequestFromCtx(ctx)

	// 设置响应头
	r.Response.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	if ext == ".json" {
		r.Response.Header().Set("Content-Type", "application/json")
	} else {
		r.Response.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	}
	r.Response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	r.Response.Header().Set("Pragma", "no-cache")
	r.Response.Header().Set("Expires", "0")

	// 发送文件
	http.ServeFile(r.Response.Writer, r.Request, filePath)

	return
}
