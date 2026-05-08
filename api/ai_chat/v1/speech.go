package v1

import "github.com/gogf/gf/v2/frame/g"

type SpeechTranscribeReq struct {
	g.Meta `path:"/speech/transcribe" method:"post" tags:"V1/语音" sm:"语音转写" dc:"接收音频文件并转写为文字"`
	Topic  string `json:"topic" in:"form" dc:"主题：grid|population|traffic"`
}

type SpeechTranscribeRes struct {
	Text       string `json:"text"`
	DurationMs int    `json:"durationMs"`
	Language   string `json:"language"`
}
