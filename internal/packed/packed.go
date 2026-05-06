package packed

import (
	"ai-chat-sql/internal/consts"
	"context"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/sqlite/v2"
	"github.com/gogf/gf/v2/frame/g"
)

func CheckDatabase(ctx context.Context) error {
	// 验证数据库连接
	_, err := g.DB("master").Exec(ctx, "SELECT 1")
	if err != nil {
		consts.Logger.Errorf(ctx, "数据库连接检查失败: %s", err.Error())
		return err
	}
	return nil
}
