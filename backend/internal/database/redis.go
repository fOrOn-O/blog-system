package database

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"blog-system/internal/config"

	"github.com/go-redis/redis/v8"
)

// Redis 全局Redis客户端
var Redis *redis.Client

// Redis是否可用
var redisAvailable bool
var redisOnce sync.Once

// 内存缓存降级（当Redis不可用时使用）
var memoryCache = &MemoryCache{
	data: make(map[string]*CacheItem),
}

// CacheItem 缓存项
type CacheItem struct {
	Value     string
	ExpiresAt time.Time
}

// MemoryCache 内存缓存
type MemoryCache struct {
	data map[string]*CacheItem
	mu   sync.RWMutex
}

// Get 获取缓存
func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists {
		return "", false
	}

	if time.Now().After(item.ExpiresAt) {
		delete(c.data, key)
		return "", false
	}

	return item.Value, true
}

// Set 设置缓存
func (c *MemoryCache) Set(key string, value string, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = &CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(expiration),
	}
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// DeletePrefix 删除指定前缀的所有缓存项
func (c *MemoryCache) DeletePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			delete(c.data, key)
		}
	}
}

// InitRedis 初始化Redis连接
func InitRedis() {
	cfg := config.AppConfig.Redis

	// 如果Redis配置为空，跳过连接
	if cfg.Host == "" {
		log.Println("Redis未配置，使用内存缓存")
		redisAvailable = false
		return
	}

	Redis = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Redis.Ping(ctx).Result()
	if err != nil {
		log.Printf("Redis连接失败，使用内存缓存: %v", err)
		redisAvailable = false
		return
	}

	redisAvailable = true
	log.Println("Redis连接成功")
}

// IsRedisAvailable 检查Redis是否可用
func IsRedisAvailable() bool {
	return redisAvailable
}

// CacheSet 设置缓存（自动降级）
func CacheSet(key string, value string, expiration time.Duration) {
	if redisAvailable {
		ctx := context.Background()
		Redis.Set(ctx, key, value, expiration)
	} else {
		memoryCache.Set(key, value, expiration)
	}
}

// CacheGet 获取缓存（自动降级）
func CacheGet(key string) (string, error) {
	if redisAvailable {
		ctx := context.Background()
		return Redis.Get(ctx, key).Result()
	}

	value, exists := memoryCache.Get(key)
	if !exists {
		return "", redis.Nil
	}
	return value, nil
}

// CacheDelete 删除缓存（自动降级）
func CacheDelete(key string) {
	if redisAvailable {
		ctx := context.Background()
		Redis.Del(ctx, key)
	} else {
		memoryCache.Delete(key)
	}
}

// CacheDeletePrefix 删除指定前缀的所有缓存（自动降级）
func CacheDeletePrefix(prefix string) {
	if redisAvailable {
		ctx := context.Background()
		iterator := Redis.Scan(ctx, 0, prefix+"*", 100).Iterator()
		keys := make([]string, 0)
		for iterator.Next(ctx) {
			keys = append(keys, iterator.Val())
		}
		if len(keys) > 0 {
			Redis.Del(ctx, keys...)
		}
		return
	}

	memoryCache.DeletePrefix(prefix)
}

// CloseRedis 关闭Redis连接
func CloseRedis() {
	if Redis != nil {
		Redis.Close()
		log.Println("Redis连接已关闭")
	}
}
