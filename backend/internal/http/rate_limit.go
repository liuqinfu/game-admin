package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"game-admin/backend/internal/cache"
	"github.com/gin-gonic/gin"
)

func openAPIRateLimitMiddleware(redisRuntime *cache.Runtime) gin.HandlerFunc {
	limiter := cache.FixedWindowLimiter{
		Runtime: redisRuntime,
		Prefix:  "ratelimit:openapi",
		Limit:   60,
		Window:  time.Minute,
	}
	return func(c *gin.Context) {
		subject := strings.TrimSpace(c.ClientIP()) + "|" + c.FullPath()
		allowed, _, err := limiter.Allow(context.Background(), subject)
		if err != nil {
			c.Next()
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "too_many_requests",
				"message": "too many requests",
				"error":   "too many requests",
			})
			return
		}
		c.Next()
	}
}
