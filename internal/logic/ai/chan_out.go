package ai

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/mcp"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/utility"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
)

var (
	withTablesAccumulator = mcp.WithTablesAccumulator
	tablesFromContext      = mcp.TablesFromContext
)

// chanCloser 安全地关闭 channel，防止重复 close 导致 panic
type chanCloser struct {
	ch   chan any
	once sync.Once
}

func newChanCloser(ch chan any) *chanCloser {
	return &chanCloser{ch: ch}
}

func (c *chanCloser) Close() {
	c.once.Do(func() {
		close(c.ch)
	})
}

// streamState 流式输出状态机
type streamState int

const (
	// stateBuffering 初始缓冲状态，等待判断是澄清还是正常回答
	stateBuffering streamState = iota
	// stateSkippingReasoning 跳过模型推理过程，等待正式回答结构
	stateSkippingReasoning
	// stateNormal 正常回答流式输出
	stateNormal
	// stateClarification 澄清块内容累积
	stateClarification
)

// clarifyPrefix 澄清块前缀标记
const clarifyPrefix = "```chatdb-clarify"

// clarifyPrefixLen 澄清块前缀长度，用于初始判定窗口
const clarifyDetectWindow = 30

// sendStreamError 向 SSE 通道发送错误事件，对用户隐藏内部错误细节
func sendStreamError(ctx context.Context, respChan chan any, err error) {
	consts.Logger.Errorf(ctx, "流错误: %v", err)
	_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
		Event: "error",
		Data: g.Map{
			"message": utility.SafeUserErr(err),
		},
	}, respChan)
}

type chunkResult struct {
	chunk *schema.Message
	err   error
}

// answerStartMarkers 用于检测正式回答开始，跳过模型推理文本
var answerStartMarkers = []string{"## 精", "## 特", "## 洞", "## 可"}

// isAnswerStart 判断缓冲内容是否包含正式回答结构标记
func isAnswerStart(s string) bool {
	for _, m := range answerStartMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// extractFromMarker 从缓冲内容中提取从第一个回答标记开始的部分
func extractFromMarker(s string) string {
	for _, m := range answerStartMarkers {
		if idx := strings.Index(s, m); idx >= 0 {
			return s[idx:]
		}
	}
	return s
}

func (s *sAiChat) AiChatStreamOut(ctx context.Context, closer *chanCloser, stream *schema.StreamReader[*schema.Message], cancel context.CancelFunc) {
	respChan := closer.ch
	g.Go(ctx, func(ctx context.Context) {
		defer closer.Close()

		state := stateBuffering
		var buffer strings.Builder
		var clarifyBuf strings.Builder
		var reasonBuf strings.Builder

		// 首个 token 30s 超时，后续每个 chunk 15s 超时
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()

		for {
			chCh := make(chan chunkResult, 1)
			go func() {
				ch, e := stream.Recv()
				chCh <- chunkResult{ch, e}
			}()

			select {
			case <-ctx.Done():
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
				cancel()
				return

			case <-timer.C:
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
					Event: "error",
					Data:  g.Map{"message": "查询超时，请尝试简化问题或换一种问法"},
				}, respChan)
				_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{Event: "end"}, respChan)
				cancel()
				return

			case res := <-chCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(15 * time.Second)

				if errors.Is(res.err, io.EOF) {
					consts.Logger.Infof(ctx, "perf ask_number_stream EOF state=%d bufLen=%d reasonLen=%d clarifyLen=%d", state, buffer.Len(), reasonBuf.Len(), clarifyBuf.Len())
					// 从context中读取ExecSql累积的表名
					if t := tablesFromContext(ctx); t != nil && len(*t) > 0 {
						_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
							Event:   "tables",
							Content: strings.Join(*t, ","),
						}, respChan)
					}

					switch state {
					case stateBuffering:
						if buffer.Len() > 0 {
							_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
								Event:   "message",
								Content: utility.SanitizeOutput(buffer.String()),
								Role:    "assistant",
							}, respChan)
						}
					case stateSkippingReasoning:
						// EOF时如果还在跳过推理状态，说明回复没有正式标记
						// 直接输出reasonBuf内容，避免吞掉整个回复
						if reasonBuf.Len() > 0 {
							_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
								Event:   "message",
								Content: utility.SanitizeOutput(reasonBuf.String()),
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

				consts.Logger.Infof(ctx, "perf ask_number_stream chunk state=%d content=%q", state, content)

				switch state {
				case stateBuffering:
					buffer.WriteString(content)
					buf := buffer.String()
					if strings.HasPrefix(buf, clarifyPrefix) {
						state = stateClarification
						clarifyBuf.WriteString(buf[len(clarifyPrefix):])
						buffer.Reset()
					} else if len(buf) >= clarifyDetectWindow {
						state = stateSkippingReasoning
						reasonBuf.WriteString(buf)
						buffer.Reset()
						if isAnswerStart(reasonBuf.String()) {
							content := extractFromMarker(reasonBuf.String())
							reasonBuf.Reset()
							state = stateNormal
							_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
								Event:   "message",
								Content: utility.SanitizeOutput(content),
								Role:    gconv.String(res.chunk.Role),
							}, respChan)
						}
					}

				case stateSkippingReasoning:
					reasonBuf.WriteString(content)
					if isAnswerStart(reasonBuf.String()) {
						content := extractFromMarker(reasonBuf.String())
						reasonBuf.Reset()
						state = stateNormal
						_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
							Event:   "message",
							Content: utility.SanitizeOutput(content),
							Role:    gconv.String(res.chunk.Role),
						}, respChan)
					}

				case stateNormal:
					_ = model.SendChatOutDataItem(ctx, model.ChatOutDataItem{
						Event:   "message",
						Content: utility.SanitizeOutput(content),
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
		sendPing := func() bool {
			select {
			case respChan <- "event: ping":
				return true
			case <-ctx.Done():
				return false
			}
		}
		if !sendPing() {
			ticker.Stop()
			return
		}
		for {
			select {
			case <-ticker.C:
				if !sendPing() {
					ticker.Stop()
					return
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}, func(ctx context.Context, exception error) {
		consts.Logger.Errorf(ctx, "AiChatHeartbeat 异常 %s", exception.Error())
	})
}
