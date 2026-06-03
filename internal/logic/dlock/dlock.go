package dlock

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TryAcquire attempts to acquire a distributed lock. Returns true on success.
func TryAcquire(ctx context.Context, lockKey string, ttlMinutes int) (bool, error) {
	db := g.DB("master")
	now := int(time.Now().Unix())
	expire := now + ttlMinutes*60
	instance := consts.GetConfig().Server.Address

	result, err := db.Exec(ctx, `
INSERT INTO distributed_lock (lock_key, locked_by, locked_until)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE
    locked_by = CASE WHEN locked_until < ? OR locked_by = ? THEN ? ELSE locked_by END,
    locked_until = CASE WHEN locked_until < ? OR locked_by = ? THEN ? ELSE locked_until END`,
		lockKey, instance, expire,
		now, instance, instance,
		now, instance, expire)
	if err != nil {
		return false, err
	}

	affected, _ := result.RowsAffected()
	return affected > 0, nil
}

// Release releases a distributed lock.
func Release(ctx context.Context, lockKey string) {
	db := g.DB("master")
	instance := consts.GetConfig().Server.Address
	_, err := db.Exec(ctx, `UPDATE distributed_lock SET locked_by = '', locked_until = 0 WHERE lock_key = ? AND locked_by = ?`,
		lockKey, instance)
	if err != nil {
		consts.Logger.Warningf(ctx, "release lock %s failed: %v", lockKey, err)
	}
}

// EnsureTable creates the distributed_lock table if not exists.
func EnsureTable(ctx context.Context) error {
	db := g.DB("master")
	for _, sql := range model.DLockSQL {
		if _, err := db.Exec(ctx, sql); err != nil {
			return fmt.Errorf("create distributed_lock table: %w", err)
		}
	}
	return nil
}
