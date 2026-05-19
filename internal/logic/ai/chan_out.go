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

type chunkResult struct {
	chunk *schema.Message
	err   error
}

func (s *sAiChat) AiChatStreamOut(ctx context.Context, respChan chan any, stream *schema.StreamReader[*schema.Message], cancel context.CancelFunc) {
	g.Go(ctx, func(ctx context.Context) {
		defer close(respChan)

		state := stateBuffering
		var buffer strings.Builder
		var clarifyBuf strings.Builder

		// 首个 token 30s 超时，后续每个 chunk 15s 超时
		timeoutCh := time.After(30 * time.Second)

		for {
			chCh := make(chan chunkResult, 1)
			go func() {
				ch, e := stream.Recv()
				chCh <- chunkResult{ch, e}
			}()

			select {
			case <-timeoutCh:
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
					Event: "error",
					Data:  g.Map{"message": "查询超时，请尝试简化问题或换一种问法"},
				}, respChan)
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
				cancel()
				return

			case res := <-chCh:
				timeoutCh = time.After(15 * time.Second)

				if errors.Is(res.err, io.EOF) {
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

				if res.err != nil {
					sendStreamError(ctx, respChan, res.err)
					consts.Logger.Errorf(ctx, "AiChatStreamOut 流读取错误: %v", res.err)
					cancel()
					return
				}

				content := res.chunk.Content
				if content == "" {
					continue
				}

				switch state {
				case stateBuffering:
					buffer.WriteString(content)
					buf := buffer.String()
					if strings.HasPrefix(buf, clarifyPrefix) {
						state = stateClarification
						clarifyBuf.WriteString(buf[len(clarifyPrefix):])
						buffer.Reset()
					} else if len(buf) >= clarifyDetectWindow {
						state = stateNormal
						_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
							Event:   "message",
							Content: buf,
							Role:    gconv.String(res.chunk.Role),
						}, respChan)
						buffer.Reset()
					}

				case stateNormal:
					_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
						Event:   "message",
						Content: content,
						Role:    gconv.String(res.chunk.Role),
					}, respChan)

				case stateClarification:
					clarifyBuf.WriteString(content)
					buf := clarifyBuf.String()
					if idx := strings.Index(buf, "```"); idx >= 0 {
						emitClarification(ctx, respChan, buf)
						clarifyBuf.Reset()
						state = stateNormal
					}
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
