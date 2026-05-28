package mcp

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
)

// RunSafeShellCommand 执行安全受限的终端命令
func (s *sMcpTool) RunSafeShellCommand(ctx context.Context, request mcp.CallToolRequest) (out *mcp.CallToolResult, err error) {
	command := request.GetString("command", "")
	if command == "" {
		err = errors.New("command is required")
		return
	}

	// 超时（秒），默认 10 秒，最大 60 秒
	timeoutSeconds := gconv.Int(request.GetString("timeoutSeconds", "10"))
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	if timeoutSeconds > 60 {
		timeoutSeconds = 60
	}

	// 可选工作目录
	cwd := request.GetString("cwd", "")

	// 风险校验
	if err = validateSafeCommand(command); err != nil {
		out = mcp.NewToolResultText(err.Error())
		err = nil
		return
	}

	// 校验 cwd
	if cwd != "" {
		if err = validateCwd(cwd); err != nil {
			out = mcp.NewToolResultText(err.Error())
			err = nil
			return
		}
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// 不使用登录 shell（去掉 -l），避免 source 用户配置文件导致绕过
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

	// 退出码
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// 限制输出大小，防止过大返回
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

// validateSafeCommand 黑名单规则与操作符禁用
func validateSafeCommand(command string) error {
	normalized := strings.ToLower(strings.TrimSpace(command))

	// 禁用危险操作符与特性（避免复合执行、重定向、替换等）。放开 | 管道符。
	bannedOperators := []string{
		"||", "&&", ";", ">", ">>", "<", "<<", "`", "$(", "&", "2>", "2>>",
	}
	for _, op := range bannedOperators {
		if strings.Contains(normalized, op) {
			return errors.New("命令包含被禁用的操作符: " + op)
		}
	}

	// 允许使用 |，对每个分段分别做首 token 校验
	segments := strings.Split(normalized, "|")
	if len(segments) > 1 {
		if len(segments)-1 > 3 {
			return errors.New("管道分段过多：最多允许 3 个管道")
		}
		for _, seg := range segments {
			segTrim := strings.TrimSpace(seg)
			if segTrim == "" {
				return errors.New("无效的空管道分段")
			}
			if err := validateFirstToken(segTrim); err != nil {
				return err
			}
		}
	} else {
		if err := validateFirstToken(normalized); err != nil {
			return err
		}
	}

	// 禁用高危命令片段（含 shell 元编程/逃逸手段）
	bannedFragments := []string{
		"rm -rf", ":(){:|:&};:", "mkfs.", "/dev/", "/etc/passwd",
		"eval ", "exec ", "source ", ". ",
		"export ", "alias ", "function ", "typeset ", "declare ",
		"bash -i", "sh -i", "nc -", "ncat ", "/dev/tcp", "/dev/udp",
	}
	for _, frag := range bannedFragments {
		if strings.Contains(normalized, frag) {
			return errors.New("命令包含危险片段: " + strings.TrimSpace(frag))
		}
	}

	return nil
}

func validateFirstToken(cmd string) error {
	// 禁用高危命令（匹配首 token）
	bannedCommands := []string{
		// 文件系统破坏
		"rm", "rmdir", "mkfs", "dd", "chmod", "chown", "mv",
		// 系统控制
		"shutdown", "reboot", "halt", "poweroff", "init", "service", "systemctl",
		"mount", "umount",
		// 进程管理
		"kill", "pkill", "killall",
		// 定时任务
		"crontab", "at", "batch",
		// 用户/权限管理
		"useradd", "userdel", "usermod", "groupadd", "groupdel", "visudo", "sudo", "su",
		// Shell 元编程/逃逸
		"eval", "exec", "source", "export", "alias", "unalias",
		"function", "typeset", "declare", "unset",
		"bash", "sh", "zsh", "csh", "tcsh", "fish", "dash", "ksh",
		// 网络工具（常用于反弹 shell）
		"nc", "ncat", "socat", "telnet",
		// 包管理（避免安装任意软件）
		"apt", "yum", "dnf", "pip", "npm", "gem", "cargo",
	}

	firstToken := firstTokenOf(cmd)
	for _, b := range bannedCommands {
		if firstToken == b {
			return errors.New("命令被禁用: " + b)
		}
	}
	return nil
}

// validateCwd 校验工作目录，只允许在白名单目录下执行
func validateCwd(cwd string) error {
	// 禁止路径穿越
	if strings.Contains(cwd, "..") {
		return errors.New("工作目录不允许包含 .. 路径穿越")
	}
	// 禁止敏感系统目录
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
	// 截断并标记
	suffix := "\n...[truncated]"
	if max > len(suffix) {
		return s[:max-len(suffix)] + suffix
	}
	return s[:max]
}
