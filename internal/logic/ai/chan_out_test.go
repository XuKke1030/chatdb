package ai

import (
	"ai-chat-sql/internal/model"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/errors/gerror"
)

// collectSSE 从 ch 收集所有 ChatOutDataItem，直到 channel 关闭或超时
func collectSSE(ch chan any, timeout time.Duration) []model.ChatOutDataItem {
	var items []model.ChatOutDataItem
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return items
			}
			if item, isOk := v.(model.ChatOutDataItem); isOk {
				items = append(items, item)
			}
		case <-timer.C:
			return items
		}
	}
}

func TestEmitClarification_ValidJSON(t *testing.T) {
	ch := make(chan any, 10)
	raw := "\n{\"question\":\"您想查询哪个时间段？\",\"options\":[\"今天\",\"最近7天\",\"最近30天\"]}\n```"

	emitClarification(context.Background(), ch, raw)
	close(ch)

	items := collectSSE(ch, 2*time.Second)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "clarification" {
		t.Fatalf("expected event=clarification, got %s", items[0].Event)
	}
	data, ok := items[0].Data.(model.ClarificationData)
	if !ok {
		t.Fatal("expected ClarificationData type")
	}
	if data.Question != "您想查询哪个时间段？" {
		t.Fatalf("unexpected question: %s", data.Question)
	}
	if len(data.Options) != 3 || data.Options[0] != "今天" {
		t.Fatalf("unexpected options: %v", data.Options)
	}
}

func TestEmitClarification_InvalidJSON(t *testing.T) {
	ch := make(chan any, 10)
	raw := "not valid json```"

	emitClarification(context.Background(), ch, raw)
	close(ch)

	items := collectSSE(ch, 2*time.Second)
	if len(items) != 1 {
		t.Fatalf("expected 1 fallback item, got %d", len(items))
	}
	if items[0].Event != "message" {
		t.Fatalf("expected fallback event=message, got %s", items[0].Event)
	}
}

func TestStreamState_TransitionToClarification(t *testing.T) {
	messages := []*schema.Message{
		{Role: schema.Assistant, Content: "```chatdb-cla"},
		{Role: schema.Assistant, Content: "rify\n{\"question\":\"选择时间\",\"options\":[\"今天\",\"7天\"]}\n```"},
	}
	stream := schema.StreamReaderFromArray(messages)

	s := &sAiChat{}
	respChan := make(chan any, 20)
	closer := newChanCloser(respChan)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.AiChatStreamOut(ctx, closer, stream, cancel)

	items := collectSSE(respChan, 3*time.Second)
	var found bool
	for _, item := range items {
		if item.Event == "clarification" {
			found = true
			raw, _ := json.Marshal(item.Data)
			var data model.ClarificationData
			_ = json.Unmarshal(raw, &data)
			if data.Question != "选择时间" {
				t.Fatalf("unexpected question: %s", data.Question)
			}
		}
	}
	if !found {
		t.Fatal("clarification event not found in SSE output")
	}
}

func TestStreamState_TransitionToNormal(t *testing.T) {
	messages := []*schema.Message{
		{Role: schema.Assistant, Content: "## 精准结论\n今天车流量为1200辆。"},
		{Role: schema.Assistant, Content: "## 特征洞察\n数据来源为卡口实时统计。"},
	}
	stream := schema.StreamReaderFromArray(messages)

	s := &sAiChat{}
	respChan := make(chan any, 20)
	closer := newChanCloser(respChan)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.AiChatStreamOut(ctx, closer, stream, cancel)

	items := collectSSE(respChan, 3*time.Second)
	for _, item := range items {
		if item.Event == "clarification" {
			t.Fatal("should not have clarification event for normal answer")
		}
	}
	var msgCount int
	for _, item := range items {
		if item.Event == "message" {
			msgCount++
		}
	}
	if msgCount == 0 {
		t.Fatal("expected at least one message event")
	}
}

func TestStreamState_ErrorEvent(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](1)
	sw.Send(nil, errors.New("mock stream error"))
	sw.Close()

	s := &sAiChat{}
	respChan := make(chan any, 20)
	closer := newChanCloser(respChan)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.AiChatStreamOut(ctx, closer, sr, cancel)

	items := collectSSE(respChan, 3*time.Second)
	var hasError bool
	for _, item := range items {
		if item.Event == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected error event when stream fails")
	}
}

func TestEmitClarification_TrailingText(t *testing.T) {
	ch := make(chan any, 10)
	// 澄清 JSON 后还有额外文本
	raw := "\n{\"question\":\"选择时间\",\"options\":[\"今天\",\"7天\"]}\n```请选择后继续。"

	emitClarification(context.Background(), ch, raw)
	close(ch)

	items := collectSSE(ch, 2*time.Second)
	// 应该有 2 个事件：clarification + trailing message
	if len(items) != 2 {
		t.Fatalf("expected 2 items (clarification + trailing), got %d", len(items))
	}
	if items[0].Event != "clarification" {
		t.Fatalf("expected first event=clarification, got %s", items[0].Event)
	}
	if items[1].Event != "message" {
		t.Fatalf("expected second event=message, got %s", items[1].Event)
	}
	if items[1].Content != "请选择后继续。" {
		t.Fatalf("unexpected trailing content: %s", items[1].Content)
	}
}

func TestSendStreamError(t *testing.T) {
	ch := make(chan any, 10)
	ctx := context.Background()

	sendStreamError(ctx, ch, gerror.New("test error"))
	close(ch)

	items := collectSSE(ch, 2*time.Second)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "error" {
		t.Fatalf("expected event=error, got %s", items[0].Event)
	}
}
