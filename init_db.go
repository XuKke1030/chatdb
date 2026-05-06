//go:build ignore

package main

import (
	"context"
	"fmt"

	_ "ai-chat-sql/internal/logic"
	_ "ai-chat-sql/internal/packed"

	"github.com/gogf/gf/v2/frame/g"
)

func main() {
	ctx := context.Background()
	db := g.DB("master")

	// 创建 user 表
	_, err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS user (
    user_id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    verify INTEGER NOT NULL DEFAULT 0,
    rule_level INTEGER NOT NULL DEFAULT 1,
    last_login_tme INTEGER DEFAULT 0,
    create_time INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    update_time INTEGER DEFAULT 0
);
	`)
	if err != nil {
		fmt.Println("创建 user 表失败:", err)
		return
	}
	fmt.Println("user 表创建成功")

	// 创建 database_conf 表
	_, err = db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS database_conf (
    database_id INTEGER PRIMARY KEY AUTOINCREMENT,
    db_name TEXT NOT NULL,
    user_name TEXT NOT NULL,
    password TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    db_type TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    create_time INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    update_time INTEGER DEFAULT 0
);
	`)
	if err != nil {
		fmt.Println("创建 database_conf 表失败:", err)
		return
	}
	fmt.Println("database_conf 表创建成功")

	// 创建 chats 表
	_, err = db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS chats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    messages TEXT NOT NULL,
    create_time INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    update_time INTEGER DEFAULT 0
);
	`)
	if err != nil {
		fmt.Println("创建 chats 表失败:", err)
		return
	}
	fmt.Println("chats 表创建成功")

	fmt.Println("数据库初始化完成!")
}
