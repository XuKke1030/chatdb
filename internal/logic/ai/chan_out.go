package ai

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
)

// streamState 流式输出状态机
type streamState int

const (
	// stateBuffering 初始缓冲状态，等待判断是澄清还是正常回答
	stateBuffering streamState = iota
	// stateNormal 正常回答流式输出
	stateNormal
	// stateClarification 澄清块内容累积
	stateClarification
)

// clarifyPrefix 澄清块前缀标记
const clarifyPrefix = "```chatdb-clarify"

// clarifyPrefixLen 澄清块前缀长度，用于初始判定窗口
const clarifyDetectWindow = 30

// sendStreamError 向 SSE 通道发送错误事件
func sendStreamError(ctx context.Context, respChan chan any, err error) {
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "error",
		Data: g.Map{
			"message": err.Error(),
		},
	}, respChan)
}

func (s *sAiChat) AiChatStreamOut(ctx context.Context, respChan chan any, stream *schema.StreamReader[*schema.Message], cancel context.CancelFunc) {
	g.Go(ctx, func(ctx context.Context) {
		defer close(respChan)

		state := stateBuffering
		var buffer strings.Builder // 判定窗口缓冲
		var clarifyBuf strings.Builder // 澄清块 JSON 缓冲

		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// 根据最终状态处理剩余缓冲
				switch state {
				case stateBuffering:
					if buffer.Len() > 0 {
						_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
							Event:   "message",
							Content: buffer.String(),
							Role:    "assistant",
						}, respChan)
					}
				case stateClarification:
					emitClarification(ctx, respChan, clarifyBuf.String())
				}
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
				cancel()
				return
			}
			if err != nil {
				sendStreamError(ctx, respChan, err)
				consts.Logger.Errorf(ctx, "AiChatStreamOut 流读取错误: %v", err)
				cancel()
				return
			}

			content := chunk.Content
			if content == "" {
				continue
			}

			switch state {
			case stateBuffering:
				buffer.WriteString(content)
				buf := buffer.String()
				if strings.HasPrefix(buf, clarifyPrefix) {
					// 判定为澄清块，切换到澄清模式
					state = stateClarification
					clarifyBuf.WriteString(buf[len(clarifyPrefix):])
					buffer.Reset()
				} else if len(buf) >= clarifyDetectWindow {
					// 缓冲窗口已满且不是澄清块，切换到正常模式
					state = stateNormal
					_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
						Event:   "message",
						Content: buf,
						Role:    gconv.String(chunk.Role),
					}, respChan)
					buffer.Reset()
				}

			case stateNormal:
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
					Event:   "message",
					Content: content,
					Role:    gconv.String(chunk.Role),
				}, respChan)

			case stateClarification:
				clarifyBuf.WriteString(content)
				// 检测到结束的 ``` 标记，发射澄清事件并切换到正常模式
				buf := clarifyBuf.String()
				if idx := strings.Index(buf, "```"); idx >= 0 {
					emitClarification(ctx, respChan, buf)
					clarifyBuf.Reset()
					state = stateNormal
				}
			}
		}
	}, func(ctx context.Context, exception error) {
		cancel()
		consts.Logger.Errorf(ctx, "AiChatStreamOut 异常 %s", exception.Error())
	})
}

// emitClarification 从原始缓冲内容中解析澄清 JSON 并发送 clarification SSE 事件。
// 如果 ``` 结束标记后有额外文本，会作为 message 事件补发。
func emitClarification(ctx context.Context, respChan chan any, raw string) {
	// 找到第一个 ``` 结束标记的位置
	endIdx := strings.Index(raw, "```")
	jsonStr := raw
	trailing := ""
	if endIdx >= 0 {
		jsonStr = raw[:endIdx]
		trailing = strings.TrimSpace(raw[endIdx+3:])
	}
	jsonStr = strings.TrimSpace(jsonStr)

	var data model.ClarificationData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// JSON 解析失败，降级为普通消息
		consts.Logger.Errorf(ctx, "clarify JSON 解析失败: %v, raw: %s", err, raw)
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event:   "message",
			Content: raw,
			Role:    "assistant",
		}, respChan)
		return
	}

	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "clarification",
		Data:  data,
	}, respChan)

	// 如果结束标记后还有额外文本，补发为 message 事件
	if trailing != "" {
		_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
			Event:   "message",
			Content: trailing,
			Role:    "assistant",
		}, respChan)
	}
}

func (s *sAiChat) AiChatHeartbeat(ctx context.Context, respChan chan any) {
	g.Go(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(time.Millisecond * 1500)
		respChan <- "event: ping"
		for {
			select {
			case <-ticker.C:
				respChan <- "event: ping"
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}, func(ctx context.Context, exception error) {
		consts.Logger.Errorf(ctx, "AiChatHeartbeat 异常 %s", exception.Error())
	})
}
