package shared

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"game-admin/backend/internal/cache"
	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
)

var (
	redisRuntimeOnce sync.Once
	redisRuntimeInst *cache.Runtime
)

func sharedRedisRuntime() *cache.Runtime {
	redisRuntimeOnce.Do(func() {
		runtime, err := cache.NewRuntime(config.Load().Redis)
		if err == nil {
			redisRuntimeInst = runtime
		}
	})
	return redisRuntimeInst
}

func resolveCachedJSON[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	var zero T
	runtime := sharedRedisRuntime()
	if runtime == nil {
		return loader()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if payload, ok, err := runtime.Get(ctx, key); err == nil && ok && strings.TrimSpace(payload) != "" {
		var item T
		if json.Unmarshal([]byte(payload), &item) == nil {
			return item, nil
		}
	}
	item, err := loader()
	if err != nil {
		return zero, err
	}
	if payload, err := json.Marshal(item); err == nil {
		_ = runtime.Set(ctx, key, string(payload), ttl)
	}
	return item, nil
}

func bumpCacheVersion(namespace string) {
	runtime := sharedRedisRuntime()
	if runtime == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = runtime.Set(ctx, "cache-version:"+strings.TrimSpace(namespace), time.Now().UTC().Format(time.RFC3339Nano), 24*time.Hour)
}

func cacheVersion(namespace string) string {
	runtime := sharedRedisRuntime()
	if runtime == nil {
		return "local"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, ok, err := runtime.Get(ctx, "cache-version:"+strings.TrimSpace(namespace))
	if err != nil || !ok || strings.TrimSpace(value) == "" {
		return "v0"
	}
	return strings.TrimSpace(value)
}

func cachedRuleListKey(game model.Game, filter any) string {
	payload, _ := json.Marshal(filter)
	return "rule-openapi:" + strings.TrimSpace(cacheVersion("rule-openapi")) + ":" + string(payload) + ":game:" + formatOptionalUint64(game.TenantID) + ":" + formatOptionalUint64(game.BrandID) + ":" + formatUint64(game.ID)
}

func cachedEnumDictionaryKey(code string) string {
	return "enum-dictionary:" + strings.TrimSpace(cacheVersion("enum-dictionary")) + ":" + strings.TrimSpace(code)
}

func formatOptionalUint64(value *uint64) string {
	if value == nil {
		return "nil"
	}
	return formatUint64(*value)
}

func formatUint64(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func ResolveCachedRuleList(game model.Game, filter any, loader func() ([]model.CommissionRule, error)) ([]model.CommissionRule, error) {
	return resolveCachedJSON(cachedRuleListKey(game, filter), 30*time.Second, loader)
}

func BumpRuleOpenAPICache() {
	bumpCacheVersion("rule-openapi")
}

func BumpEnumDictionaryCache() {
	bumpCacheVersion("enum-dictionary")
}
