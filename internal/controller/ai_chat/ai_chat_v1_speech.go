package ai_chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	v1 "ai-chat-sql/api/ai_chat/v1"
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
)

const maxSpeechUploadBytes int64 = 10 * 1024 * 1024

var allowedSpeechExt = map[string]bool{
	".webm": true,
	".wav":  true,
	".mp3":  true,
	".m4a":  true,
	".mp4":  true,
	".ogg":  true,
}

func (c *ControllerV1) SpeechTranscribe(ctx context.Context, req *v1.SpeechTranscribeReq) (res *v1.SpeechTranscribeRes, err error) {
	userId := 0
	if userIdVal := ctx.Value(model.UserGroup{}); userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}
	if req.Topic != "" && userId > 0 {
		user, userErr := service.User().GetUserInfoById(ctx, int64(userId))
		if userErr != nil {
			return nil, userErr
		}
		ruleLevel := effectiveTopicRuleLevel(ctx, userId, user.RuleLevel)
		if !checkTopicPermission(ruleLevel, req.Topic) {
			return nil, gerror.New("没有该主题的访问权限")
		}
	}

	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return nil, gerror.New("请求上下文不存在")
	}
	uploadFile := request.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("音频文件不能为空")
	}
	if uploadFile.Size > maxSpeechUploadBytes {
		return nil, gerror.New("音频文件不能超过 10MB")
	}
	ext := strings.ToLower(filepath.Ext(uploadFile.Filename))
	if !allowedSpeechExt[ext] {
		return nil, gerror.New("仅支持 webm、wav、mp3、m4a、mp4、ogg 音频格式")
	}

	file, err := uploadFile.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxSpeechUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxSpeechUploadBytes {
		return nil, gerror.New("音频文件不能超过 10MB")
	}
	if len(content) == 0 {
		return nil, gerror.New("音频文件不能为空")
	}

	text, err := transcribeWithOpenAICompatibleASR(ctx, uploadFile.Filename, content)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, gerror.New("语音识别结果为空，请重试或手动输入")
	}
	runes := []rune(text)
	if len(runes) > 100 {
		text = string(runes[:100])
	}
	return &v1.SpeechTranscribeRes{
		Text:       text,
		DurationMs: request.Get("durationMs").Int(),
		Language:   "zh-CN",
	}, nil
}

func transcribeWithOpenAICompatibleASR(ctx context.Context, fileName string, content []byte) (string, error) {
	baseURL, apiKey, modelName := resolveASRConfig()
	if baseURL == "" || apiKey == "" {
		return "", gerror.New("ASR 服务未配置，请设置 CHATDB_ASR_BASE_URL 与 CHATDB_ASR_KEY")
	}
	if modelName == "" {
		modelName = "whisper-1"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(content); err != nil {
		return "", err
	}
	_ = writer.WriteField("model", modelName)
	_ = writer.WriteField("language", "zh")
	_ = writer.WriteField("response_format", "json")
	if err = writer.Close(); err != nil {
		return "", err
	}

	url := strings.TrimRight(baseURL, "/") + "/audio/transcriptions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		consts.Logger.Errorf(ctx, "ASR 调用失败: status=%d body=%s", resp.StatusCode, string(respBody))
		return "", gerror.Newf("ASR 服务调用失败，状态码 %d", resp.StatusCode)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err = json.Unmarshal(respBody, &payload); err != nil {
		return "", fmt.Errorf("解析 ASR 响应失败: %w", err)
	}
	return payload.Text, nil
}

func resolveASRConfig() (baseURL string, apiKey string, modelName string) {
	if consts.Config != nil && consts.Config.AiConfig != nil {
		if consts.Config.AiConfig.Asr != nil {
			baseURL = consts.Config.AiConfig.Asr.BaseUrl
			apiKey = consts.Config.AiConfig.Asr.Key
			modelName = consts.Config.AiConfig.Asr.Model
		}
		if baseURL == "" && consts.Config.AiConfig.OpenAI != nil {
			baseURL = consts.Config.AiConfig.OpenAI.BaseUrl
		}
		if apiKey == "" && consts.Config.AiConfig.OpenAI != nil {
			apiKey = consts.Config.AiConfig.OpenAI.Key
		}
	}
	return
}
