package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func tenantScopeFromContext(c *gin.Context) TenantScope {
	if c == nil {
		return TenantScope{}
	}
	if value, ok := c.Get("tenantScope"); ok {
		switch scope := value.(type) {
		case TenantScope:
			return normalizeTenantScope(scope)
		case *TenantScope:
			if scope != nil {
				return normalizeTenantScope(*scope)
			}
		}
	}
	return TenantScope{}
}

func agentIDFromContext(c *gin.Context) *uint64 {
	if c == nil {
		return nil
	}
	if value, ok := c.Get("agentID"); ok {
		switch typed := value.(type) {
		case uint64:
			return &typed
		case *uint64:
			return typed
		}
	}
	scope := tenantScopeFromContext(c)
	return scope.AgentID
}

func requireAdminUser(c *gin.Context) bool {
	if agentIDFromContext(c) == nil {
		return true
	}
	respondError(c, http.StatusForbidden, errors.New("agent account cannot access admin operation"))
	c.Abort()
	return false
}

func agentScopeIDsFromContext(c *gin.Context) []uint64 {
	if c == nil {
		return nil
	}
	if value, ok := c.Get("agentScopeIDs"); ok {
		switch typed := value.(type) {
		case []uint64:
			return typed
		case *[]uint64:
			if typed != nil {
				return *typed
			}
		}
	}
	if agentID := agentIDFromContext(c); agentID != nil {
		return []uint64{*agentID}
	}
	return nil
}
