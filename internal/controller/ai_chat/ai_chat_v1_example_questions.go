package ai_chat

import (
	"context"

	v1 "ai-chat-sql/api/ai_chat/v1"

	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ExampleQuestions(ctx context.Context, req *v1.ExampleQuestionsReq) (res *v1.ExampleQuestionsRes, err error) {
	db := g.DB("master").Model("admin_example_question").Ctx(ctx).Where("enabled = ?", 1)
	if req.Topic != "" {
		db = db.Where("topic = ?", req.Topic)
	}
	records, err := db.OrderAsc("sort").OrderAsc("id").All()
	if err != nil {
		return nil, err
	}

	list := make([]v1.ExampleQuestionSimple, 0, len(records))
	for _, r := range records {
		list = append(list, v1.ExampleQuestionSimple{
			Id:       r["id"].Int(),
			Topic:    r["topic"].String(),
			Question: r["question"].String(),
			Sort:     r["sort"].Int(),
		})
	}
	return &v1.ExampleQuestionsRes{List: list}, nil
}
