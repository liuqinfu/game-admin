package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	identitysvc "game-admin/backend/internal/services/identity"
	"github.com/gin-gonic/gin"
)

type agentScopeLoader func(agentID uint64) ([]uint64, error)

func authMiddleware(identityService *identitysvc.Service, loadAgentScope agentScopeLoader) gin.HandlerFunc {
	return func(c *gin.Context) {
		ensureRequestContext(c)
		if identityService == nil {
			respondError(c, http.StatusInternalServerError, errors.New("identity service is nil"))
			c.Abort()
			return
		}
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			respondError(c, http.StatusUnauthorized, errors.New("unauthorized"))
			c.Abort()
			return
		}
		account, ok := identityService.AccountByToken(strings.TrimSpace(authorization[7:]))
		if !ok {
			respondError(c, http.StatusUnauthorized, errors.New("unauthorized"))
			c.Abort()
			return
		}
		httpAccount := fromServiceAccount(account)
		c.Set("operatorID", strconv.FormatUint(httpAccount.UserID, 10))
		c.Set("operatorName", httpAccount.DisplayName)
		c.Set("operatorRole", httpAccount.Role)
		if httpAccount.AgentID != nil {
			if loadAgentScope == nil {
				respondError(c, http.StatusInternalServerError, errors.New("agent scope loader is nil"))
				c.Abort()
				return
			}
			scopeIDs, err := loadAgentScope(*httpAccount.AgentID)
			if err != nil {
				respondError(c, http.StatusInternalServerError, err)
				c.Abort()
				return
			}
			c.Set("agentID", *httpAccount.AgentID)
			c.Set("agentScopeIDs", scopeIDs)
		}
		c.Set("authAccount", httpAccount)
		c.Set("permissions", httpAccount.Permissions)
		c.Set("tenantScope", httpAccount.TenantScope)
		c.Next()
	}
}

const (
	gameAccessKeyHeader   = "X-Game-Access-Key"
	gameTimestampHeader   = "X-Game-Timestamp"
	gameNonceHeader       = "X-Game-Nonce"
	gameSignatureHeader   = "X-Game-Signature"
	gameOpenAPIContextKey = "gameOpenAPIContext"
)
