package config

import (
	"ai-chat-sql/internal/dao"
	"ai-chat-sql/internal/model/entity"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/database/gdb"
)

type sConfig struct {
	dbMap map[int]gdb.DB
	mutex *sync.RWMutex
}

func init() {
	service.RegisterConfig(NewConfig())
}

func NewConfig() *sConfig {
	return &sConfig{
		dbMap: make(map[int]gdb.DB),
		mutex: &sync.RWMutex{},
	}
}

// GetDataBase 获取数据库连接
func (s *sConfig) GetDataBase(ctx context.Context, databaseId int) (db gdb.DB, err error) {
	// 先检查缓存
	s.mutex.RLock()
	db, ok := s.dbMap[databaseId]
	s.mutex.RUnlock()

	if ok {
		return
	}

	// 创建新的数据库连接
	link, err := s.GenDataBaseLink(ctx, databaseId)
	if err != nil {
		return
	}
	db, err = gdb.New(gdb.ConfigNode{
		Link:   link,
		DryRun: true,
	})
	if err != nil {
		return
	}

	// 连接层只读加固：所有写入操作在驱动层被拒绝
	switch {
	case strings.HasPrefix(link, "mysql:"):
		_, _ = db.Exec(ctx, "SET SESSION TRANSACTION READ ONLY")
	case strings.HasPrefix(link, "pgsql:"):
		_, _ = db.Exec(ctx, "SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY")
	}

	// 写入缓存，使用写锁
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.dbMap[databaseId] = db
	return
}

// InvalidateDataBase 清理指定数据库连接缓存
func (s *sConfig) InvalidateDataBase(ctx context.Context, databaseId int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.dbMap, databaseId)
}

// GenDataBaseLink 生成数据库连接
//
// 格式
// mysql:root:12345678@tcp(127.0.0.1:3306)/test?loc=Local&parseTime=true
// pgsql:root:12345678@tcp(127.0.0.1:5432)/test
func (s *sConfig) GenDataBaseLink(ctx context.Context, databaseId int) (link string, err error) {
	// 根据数据库ID查询配置信息
	var config *entity.DatabaseConf
	err = dao.DatabaseConf.Ctx(ctx).Where("database_id = ?", databaseId).Scan(&config)
	if err != nil {
		return "", fmt.Errorf("查询数据库配置失败: %w", err)
	}
	if config == nil {
		return "", fmt.Errorf("数据库配置不存在")
	}

	// 根据数据库类型生成连接字符串
	switch config.DbType {
	case "mysql":
		link = fmt.Sprintf("mysql:%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			config.UserName,
			config.Password,
			config.Host,
			config.Port,
			config.DbName,
		)
	case "pgsql", "postgres", "postgresql":
		link = fmt.Sprintf("pgsql:%s:%s@tcp(%s:%d)/%s",
			config.UserName,
			config.Password,
			config.Host,
			config.Port,
			config.DbName,
		)
	default:
		return "", fmt.Errorf("不支持的数据库类型: %s", config.DbType)
	}

	return link, nil
}
