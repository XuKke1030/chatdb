//go:build ignore

package main

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	ctx := gctx.GetInitCtx()
	// 加载配置文件
	cfg, _ := gcfg.New()
	cfg.SetPath(".")
	g.Cfg().GetAdapter().(*gcfg.AdapterFile).SetPath(".")

	var result []map[string]interface{}
	err := g.DB("master").Model("user").Ctx(ctx).Scan(&result)
	if err != nil {
		fmt.Println("查询失败:", err)
		return
	}

	fmt.Println("用户列表:")
	for _, r := range result {
		fmt.Printf("ID: %v, 用户名: %v, 密码hash: %v, verify: %v\n", r["user_id"], r["username"], r["password"], r["verify"])
	}
}
