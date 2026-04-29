package http

import "github.com/gin-gonic/gin"

func operatorIDFromContext(c *gin.Context) string {
	if value, ok := c.Get("operatorID"); ok {
		if s, ok := value.(string); ok {
			return s
		}
	}
	return adminUsername
}

func operatorNameFromContext(c *gin.Context) string {
	if value, ok := c.Get("operatorName"); ok {
		if s, ok := value.(string); ok {
			return s
		}
	}
	return adminUsername
}

func operatorRoleFromContext(c *gin.Context) string {
	if value, ok := c.Get("operatorRole"); ok {
		if s, ok := value.(string); ok {
			return s
		}
	}
	return "admin"
}
