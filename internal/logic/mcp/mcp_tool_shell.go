package mcp

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
)

var allowedCommands = map[string]bool{
	"ls": true, "dir": true, "cat": true, "type": true,
	"head": true, "tail": true, "grep": true, "find": true,
	"wc": true, "du": true, "df": true, "pwd": true,
	"echo": true, "date": true, "uname": true, "whoami": true,
	"stat": true, "file": true, "tree": true, "diff": true,
	"sort": true, "uniq": true, "cut": true, "tr": true,
	"awk": true, "sed": true, "xargs": true, "tee": true,
	"curl": true, "wget": true, "ping": true, "nslookup": true,
	"top": true, "ps": true, "free": true, "uptime": true,
	"ifconfig": true, "ip": true, "netstat": true, "ss": true,
	"hostname": true, "id": true, "env": true, "printenv": true,
	"which": true, "where": true, "whereis": true,
}

// RunSafeShellCommand 执行安全受限的终端命令（白名单模式）
func (s *sMcpTool) RunSafeShellCommand(ctx context.Context, request mcp.CallToolRequest) (out *mcp.CallToolResult, err error) {
	command := request.GetString("command", "")
	if command == "" {
		err = errors.New("command is required")
		return
	}

	timeoutSeconds := gconv.Int(request.GetString("timeoutSeconds", "10"))
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	if timeoutSeconds > 60 {
		timeoutSeconds = 60
	}

	cwd := request.GetString("cwd", "")

	if err = validateSafeCommand(command); err != nil {
		out = mcp.NewToolResultText(err.Error())
		err = nil
		return
	}

	if cwd != "" {
		if err = validateCwd(cwd); err != nil {
			out = mcp.NewToolResultText(err.Error())
			err = nil
			return
		}
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctxTimeout, "cmd", "/c", command)
	default:
		cmd = exec.CommandContext(ctxTimeout, "/bin/sh", "-c", command)
	}
	if cwd != "" {
		cmd.Dir = cwd
	}

	stdoutBytes := &strings.Builder{}
	stderrBytes := &strings.Builder{}
	cmd.Stdout = stdoutBytes
	cmd.Stderr = stderrBytes

	start := time.Now()
	runErr := cmd.Run()
	durationMs := time.Since(start).Milliseconds()

	killedByTimeout := ctxTimeout.Err() == context.DeadlineExceeded

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	stdout := trimLong(stdoutBytes.String(), 64*1024)
	stderr := trimLong(stderrBytes.String(), 32*1024)

	result := g.Map{
		"stdout":           stdout,
		"stderr":           stderr,
		"exitCode":         exitCode,
		"durationMs":       durationMs,
		"killedByTimeout":  killedByTimeout,
		"timeoutSeconds":   timeoutSeconds,
		"workingDirectory": cmd.Dir,
		"command":          command,
	}

	out = mcp.NewToolResultText(gjson.MustEncodeString(result))
	return
}

// validateSafeCommand 白名单校验：只允许预定义安全命令，禁止管道/重定向/分号等复合执行
func validateSafeCommand(command string) error {
	normalized := strings.TrimSpace(command)

	// 禁止复合执行操作符
	if strings.ContainsAny(normalized, "|;&`$") {
		return errors.New("命令不允许包含管道、逻辑运算、重定向、命令替换或后台执行符号")
	}
	if strings.Contains(normalized, ">\n") || strings.Contains(normalized, ">>") {
		return errors.New("命令不允许包含重定向")
	}
	if strings.Contains(normalized, "<") {
		return errors.New("命令不允许包含输入重定向")
	}

	// 提取首命令 token 并校验白名单
	firstToken := firstTokenOf(strings.ToLower(normalized))
	if firstToken == "" {
		return errors.New("空命令")
	}
	base := filepath.Base(firstToken)
	if !allowedCommands[base] {
		return errors.New("命令不在允许列表中: " + base + "。仅允许只读查看类命令")
	}

	return nil
}

func validateCwd(cwd string) error {
	if strings.Contains(cwd, "..") {
		return errors.New("工作目录不允许包含 .. 路径穿越")
	}
	bannedPrefixes := []string{
		"/etc", "/root", "/boot", "/sys", "/proc", "/dev",
		"C:\\Windows\\System32", "C:\\Windows\\SysWOW64",
	}
	normalized := strings.ReplaceAll(strings.ToLower(cwd), "\\", "/")
	for _, prefix := range bannedPrefixes {
		p := strings.ReplaceAll(strings.ToLower(prefix), "\\", "/")
		if strings.HasPrefix(normalized, p) {
			return errors.New("工作目录不允许在系统敏感目录下: " + cwd)
		}
	}
	return nil
}

func firstTokenOf(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func trimLong(s string, max int) string {
	if len(s) <= max {
		return s
	}
	suffix := "\n...[truncated]"
	if max > len(suffix) {
		return s[:max-len(suffix)] + suffix
	}
	return s[:max]
}
