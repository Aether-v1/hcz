package rediscache

import (
	"context"
	"encoding/json"
	"time"

	exchangerate "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	"github.com/Aether-v1/hcz/internal/cache"
)

const cacheKey = "global_exchange_rate:state"

// TTL：缓存有效期。到期后下次读穿 settings 并回填。
const cacheTTL = 2 * time.Minute

// CachedStore 是 exchangerate.Store 的 Redis 读穿缓存装饰器。
// Settings KV 仍是持久化真源；Redis 仅加速。Redis 故障/未配置时回落 settings，绝不 1:1。
type CachedStore struct {
	inner exchangerate.Store
}

func New(inner exchangerate.Store) *CachedStore {
	return &CachedStore{inner: inner}
}

func (c *CachedStore) GetState() (exchangerate.State, error) {
	ctx := context.Background()
	// 1. Redis hit
	var cached exchangerate.State
	if hit, err := cache.GetJSON(ctx, cacheKey, &cached); err == nil && hit {
		return cached, nil
	}
	// 2. miss / down → settings 真源（错误吞掉，回落 settings，不 1:1）
	state, err := c.inner.GetState()
	if err != nil {
		return state, err
	}
	// 3. 回填 Redis（失败不影响正确性）
	_ = cache.SetJSON(ctx, cacheKey, state, cacheTTL)
	return state, nil
}

func (c *CachedStore) SaveState(state exchangerate.State) error {
	if err := c.inner.SaveState(state); err != nil {
		return err
	}
	// 写穿：更新后立即刷新 Redis 缓存
	ctx := context.Background()
	_ = cache.SetJSON(ctx, cacheKey, state, cacheTTL)
	return nil
}

// 确保 encoding/json 被引用（decimal/time 序列化路径）
var _ = json.Marshal
