//go:build ignore

package main

import (
	"context"
	"fmt"

	_ "ai-chat-sql/internal/logic"
	_ "ai-chat-sql/internal/packed"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func main() {
	ctx := context.Background()
	db := g.DB("master")
	now := int(gtime.Timestamp())

	users := []g.Map{
		{"username": "zhangsan", "password": "65ba747efffca3c8acb1952b6ca6e417", "verify": 81371, "rule_level": 7},
		{"username": "lisi", "password": "4728ea0394876cda4bf1b870724ac106", "verify": 81372, "rule_level": 2},
		{"username": "wangwu", "password": "9b67706762d7b7d6c06df6717fbdfb33", "verify": 81373, "rule_level": 4},
		{"username": "grid_user", "password": "7e7f8e85bdff9991d0271c0ea8d2d938", "verify": 81374, "rule_level": 1},
	}

	for _, user := range users {
		count, err := db.Model("user").Ctx(ctx).Where("username = ?", user["username"]).Count()
		if err != nil {
			panic(err)
		}
		if count > 0 {
			if _, err = db.Model("user").Ctx(ctx).Where("username = ?", user["username"]).Data(user).Update(); err != nil {
				panic(err)
			}
			continue
		}
		if _, err = db.Model("user").Ctx(ctx).Data(user).Insert(); err != nil {
			panic(err)
		}
	}

	adminCount, err := db.Model("admin_account").Ctx(ctx).Where("username = ?", "admin").Count()
	if err != nil {
		panic(err)
	}
	adminData := g.Map{
		"username":    "admin",
		"password":    "e9125c6228bcd40b8d58098c62e5fd6f",
		"verify":      8137,
		"role":        "administrator",
		"enabled":     1,
		"update_time": now,
	}
	if adminCount > 0 {
		if _, err = db.Model("admin_account").Ctx(ctx).Where("username = ?", "admin").Data(adminData).Update(); err != nil {
			panic(err)
		}
	} else {
		adminData["create_time"] = now
		if _, err = db.Model("admin_account").Ctx(ctx).Data(adminData).Insert(); err != nil {
			panic(err)
		}
	}

	fmt.Println("测试账号已写入：zhangsan/123456、lisi/123456、wangwu/123456、grid_user/123456、admin/admin123")
}
