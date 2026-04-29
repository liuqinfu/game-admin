package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func hasPermission(c *gin.Context, permission string) bool {
	value, ok := c.Get("permissions")
	if !ok {
		return false
	}
	permissions, ok := value.(map[string]struct{})
	if !ok {
		return false
	}
	_, exists := permissions[permission]
	return exists
}

func requirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hasPermission(c, permission) {
			c.Next()
			return
		}
		respondError(c, http.StatusForbidden, errors.New("forbidden"))
		c.Abort()
	}
}

func requireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, permission := range permissions {
			if hasPermission(c, permission) {
				c.Next()
				return
			}
		}
		respondError(c, http.StatusForbidden, errors.New("forbidden"))
		c.Abort()
	}
}
