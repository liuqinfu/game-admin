package app

import (
	"game-admin/backend/internal/cache"
	"game-admin/backend/internal/config"
)

type RedisRuntime = cache.Runtime

func NewRedisRuntime(cfg config.RedisConfig) (*RedisRuntime, error) {
	return cache.NewRuntime(cfg)
}
