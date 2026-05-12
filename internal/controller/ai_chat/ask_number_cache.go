package ai_chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-chat-sql/internal/consts"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/gtime"
)

// AskNumberCache caches fast path results with Redis as primary
// and local memory as fallback.
type AskNumberCache struct {
	mu      sync.RWMutex
	cache   *gcache.Cache
	redisOK bool
	once    sync.Once // ensures initAdapter runs exactly once
}

var defaultAskNumberCache = &AskNumberCache{}

// ensureInit lazily initializes the cache adapter on first use.
// This avoids the init()-time config loading ordering problem.
func (c *AskNumberCache) ensureInit(ctx context.Context) {
	c.once.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.initAdapterLocked(ctx)
	})
}

// initAdapterLocked must be called with c.mu held.
func (c *AskNumberCache) initAdapterLocked(ctx context.Context) {
	cfg := consts.Config
	if cfg != nil && cfg.Redis != nil && cfg.Redis.Address != "" {
		redis, err := gredis.New(&gredis.Config{
			Address: cfg.Redis.Address,
			Db:      cfg.Redis.Db,
			Pass:    cfg.Redis.Password,
		})
		if err != nil {
			consts.Logger.Warningf(ctx, "AskNumberCache: Redis init failed: %s, using memory cache", err)
			c.setMemoryAdapterLocked()
			return
		}
		ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if _, pingErr := redis.Do(ctx2, "PING"); pingErr != nil {
			consts.Logger.Warningf(ctx, "AskNumberCache: Redis ping failed: %s, using memory cache", pingErr)
			c.setMemoryAdapterLocked()
			return
		}
		c.cache = gcache.New()
		c.cache.SetAdapter(gcache.NewAdapterRedis(redis))
		c.redisOK = true
		consts.Logger.Infof(ctx, "AskNumberCache: Redis connected at %s", cfg.Redis.Address)
		return
	}
	consts.Logger.Info(ctx, "AskNumberCache: Redis not configured, using memory cache")
	c.setMemoryAdapterLocked()
}

// setMemoryAdapterLocked must be called with c.mu held.
func (c *AskNumberCache) setMemoryAdapterLocked() {
	c.cache = gcache.New()
	c.cache.SetAdapter(gcache.NewAdapterMemory())
	c.redisOK = false
}

// IsRedis returns true if the cache is backed by Redis.
func (c *AskNumberCache) IsRedis() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.redisOK
}

// getCache safely returns the current cache instance.
func (c *AskNumberCache) getCache() *gcache.Cache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

// BuildCacheKey constructs a cache key from intent parameters.
// Format: asknum:result:{topic}:{intent}:{databaseId}:{dateRangeHash}:{paramsHash}
func BuildCacheKey(topic, intent string, databaseId int, params map[string]any) string {
	dateRangeHash := intentDateRangeHash(intent)
	pHash := paramsHash(params)
	return fmt.Sprintf("asknum:result:%s:%s:%d:%s:%s", topic, intent, databaseId, dateRangeHash, pHash)
}

// GetCache retrieves a cached FastPathResult. Returns nil if not found.
func GetCache(ctx context.Context, key string) (*FastPathResult, bool) {
	defaultAskNumberCache.ensureInit(ctx)

	val, err := defaultAskNumberCache.getCache().Get(ctx, key)
	if err != nil {
		consts.Logger.Warningf(ctx, "AskNumberCache.Get failed key=%s err=%s", key, err)
		defaultAskNumberCache.handleRedisError(ctx, err)
		return nil, false
	}
	if val == nil || val.IsNil() {
		return nil, false
	}
	data := val.Bytes()
	if len(data) == 0 {
		return nil, false
	}
	var result FastPathResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, false
	}
	return &result, true
}

// SetCache stores a FastPathResult in cache with intent-based TTL.
func SetCache(ctx context.Context, key string, result *FastPathResult) {
	defaultAskNumberCache.ensureInit(ctx)

	ttl := intentTTL(result.Intent)
	data, err := json.Marshal(result)
	if err != nil {
		consts.Logger.Errorf(ctx, "AskNumberCache.Set marshal failed key=%s err=%s", key, err)
		return
	}
	err = defaultAskNumberCache.getCache().Set(ctx, key, data, ttl)
	if err != nil {
		consts.Logger.Warningf(ctx, "AskNumberCache.Set failed key=%s err=%s", key, err)
		defaultAskNumberCache.handleRedisError(ctx, err)
		return
	}
	consts.Logger.Infof(ctx, "AskNumberCache.Set key=%s ttl=%s redis=%v", key, ttl, defaultAskNumberCache.IsRedis())
}

// DeleteCache removes a key from cache.
func DeleteCache(ctx context.Context, key string) {
	defaultAskNumberCache.ensureInit(ctx)

	_, err := defaultAskNumberCache.getCache().Remove(ctx, key)
	if err != nil {
		consts.Logger.Warningf(ctx, "AskNumberCache.Delete failed key=%s err=%s", key, err)
		defaultAskNumberCache.handleRedisError(ctx, err)
	}
}

// handleRedisError detects Redis failures and switches to memory cache.
// It also schedules a periodic reconnection attempt.
func (c *AskNumberCache) handleRedisError(ctx context.Context, err error) {
	c.mu.RLock()
	wasRedis := c.redisOK
	c.mu.RUnlock()
	if !wasRedis {
		return
	}

	c.mu.Lock()
	if !c.redisOK {
		c.mu.Unlock()
		return
	}
	c.setMemoryAdapterLocked()
	c.mu.Unlock()

	consts.Logger.Warning(ctx, "AskNumberCache: Redis error detected, switched to memory cache")

	// Schedule a background reconnection check
	go c.tryReconnect(ctx)
}

// tryReconnect periodically attempts to reconnect to Redis.
// On success, it switches back to Redis adapter and resets the once
// so subsequent requests use the Redis-backed cache.
func (c *AskNumberCache) tryReconnect(parentCtx context.Context) {
	cfg := consts.Config
	if cfg == nil || cfg.Redis == nil || cfg.Redis.Address == "" {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
		redis, err := gredis.New(&gredis.Config{
			Address: cfg.Redis.Address,
			Db:      cfg.Redis.Db,
			Pass:    cfg.Redis.Password,
		})
		if err != nil {
			cancel()
			continue
		}
		if _, pingErr := redis.Do(ctx, "PING"); pingErr != nil {
			cancel()
			continue
		}
		cancel()

		// Redis is back
		newCache := gcache.New()
		newCache.SetAdapter(gcache.NewAdapterRedis(redis))

		c.mu.Lock()
		c.cache = newCache
		c.redisOK = true
		c.mu.Unlock()

		consts.Logger.Infof(parentCtx, "AskNumberCache: Redis reconnected at %s", cfg.Redis.Address)
		return
	}
}

// intentDateRangeHash returns a short deterministic string for the date range.
func intentDateRangeHash(intent string) string {
	now := gtime.Now()
	today := now.Format("Ymd")
	switch {
	case strings.Contains(intent, "today"):
		return today
	case strings.Contains(intent, "compare.weekend"):
		from := now.AddDate(0, 0, -13).Format("Ymd")
		return from + "_" + today
	case strings.Contains(intent, "compare.holiday"):
		from := now.AddDate(0, 0, -13).Format("Ymd")
		return from + "_" + today
	case strings.Contains(intent, "trend"), strings.Contains(intent, "recent_days"):
		from := now.AddDate(0, 0, -6).Format("Ymd")
		return from + "_" + today
	case strings.Contains(intent, "rank"):
		from := now.AddDate(0, 0, -6).Format("Ymd")
		return from + "_" + today
	default:
		return today
	}
}

// paramsHash produces a deterministic short string from params.
func paramsHash(params map[string]any) string {
	if len(params) == 0 {
		return "all"
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%v", k, params[k]))
	}
	return strings.Join(parts, ",")
}

// intentTTL returns the cache TTL based on intent type.
func intentTTL(intent string) time.Duration {
	switch {
	case strings.Contains(intent, "today"):
		return 45 * time.Second
	case strings.Contains(intent, "trend"), strings.Contains(intent, "recent_days"):
		return 5 * time.Minute
	case strings.Contains(intent, "compare.holiday"):
		return 10 * time.Minute
	case strings.Contains(intent, "compare.weekend"):
		return 5 * time.Minute
	case strings.Contains(intent, "rank"):
		return 2 * time.Minute
	case strings.Contains(intent, "ratio"):
		return 1 * time.Minute
	default:
		return 2 * time.Minute
	}
}
