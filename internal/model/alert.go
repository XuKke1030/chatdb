package model

// AlertItem 告警项
type AlertItem struct {
	Id              int    `json:"id"`
	Title           string `json:"title"`
	Content         string `json:"content"`
	Question        string `json:"question"`
	Topic           string `json:"topic"` // grid|population|traffic
	Level           string `json:"level"` // warning|critical
	CreateTime      int    `json:"createTime"`
	Dismissed       bool   `json:"dismissed"`
	DisplayTimeText string `json:"displayTimeText,omitempty"`
}

// AlertListInput 告警列表查询输入
type AlertListInput struct {
	Topic string `json:"topic" dc:"主题筛选"`
}

// AlertListOutput 告警列表输出
type AlertListOutput struct {
	List []AlertItem `json:"list"`
}

// AlertDismissInput 关闭告警输入
type AlertDismissInput struct {
	Id int `json:"id" v:"required#告警ID不能为空" dc:"告警ID"`
}
