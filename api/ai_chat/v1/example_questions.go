package v1

import "github.com/gogf/gf/v2/frame/g"

type ExampleQuestionsReq struct {
	g.Meta `path:"/example-questions" method:"get" tags:"V1/示例问题" sm:"示例问题列表" dc:"获取按主题分类的推荐问题列表"`
	Topic  string `json:"topic" p:"topic" dc:"主题过滤：grid|population|traffic"`
}

type ExampleQuestionsRes struct {
	List []ExampleQuestionSimple `json:"list"`
}

type ExampleQuestionSimple struct {
	Id       int    `json:"id"`
	Topic    string `json:"topic"`
	Question string `json:"question"`
	Sort     int    `json:"sort"`
}
