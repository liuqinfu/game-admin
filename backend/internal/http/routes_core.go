package http

import (
	"errors"
	"net/http"
	"time"

	identitysvc "game-admin/backend/internal/services/identity"
	tenantsvc "game-admin/backend/internal/services/tenant"
	"github.com/gin-gonic/gin"
)

func registerHealthRoutes(engine *gin.Engine, deps RouterDependencies) {
	engine.GET("/healthz", func(c *gin.Context) {
		writeJSON(c, http.StatusOK, deps.Health.Live(time.Now(), requestIDFromContext(c), traceIDFromContext(c)))
	})
	engine.GET("/readyz", func(c *gin.Context) {
		status := deps.Health.Ready(time.Now(), requestIDFromContext(c), traceIDFromContext(c))
		httpStatus := http.StatusOK
		if status.Status != "ok" {
			httpStatus = http.StatusServiceUnavailable
		}
		if deps.Metrics != nil {
			deps.Metrics.SetReady(status.Status == "ok")
		}
		writeJSON(c, httpStatus, status)
	})
	engine.GET("/metrics", func(c *gin.Context) {
		ensureRequestContext(c)
		deps.Metrics.Handler().ServeHTTP(c.Writer, c.Request)
	})
}

func registerAuthRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	if cfg.moduleEnabled(RouterModuleAuth) {
		identityService := identitysvc.NewService(deps.DB)
		engine.POST("/api/auth/login", func(c *gin.Context) {
			var payload authLoginPayload
			if err := c.ShouldBindJSON(&payload); err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			response, _, ok := identityService.Login(payload.Username, payload.Password)
			if !ok {
				respondError(c, http.StatusUnauthorized, errors.New("invalid credentials"))
				return
			}
			writeJSON(c, http.StatusOK, toHTTPAuthLoginResponse(response))
		})

		api.GET("/auth/me", func(c *gin.Context) {
			account, ok := authAccountFromContext(c)
			if !ok {
				respondError(c, http.StatusUnauthorized, errors.New("unauthorized"))
				return
			}
			serviceAccount := identitysvc.Account{
				UserID:       account.UserID,
				Username:     account.Username,
				DisplayName:  account.DisplayName,
				PasswordHash: account.PasswordHash,
				AgentID:      account.AgentID,
				Token:        account.Token,
				Role:         account.Role,
				Permissions:  account.Permissions,
				TenantScope:  toServiceScope(account.TenantScope),
			}
			writeJSON(c, http.StatusOK, toHTTPAuthLoginResponse(identityService.BuildAuthLoginResponse(serviceAccount)))
		})
	}
}

func registerSharedAPIRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	tenantService := tenantsvc.NewService(deps.DB)
	api.GET("/enum-dictionaries", func(c *gin.Context) {
		items, err := tenantService.ListEnumDictionaries(parseEnumDictionaryCodes(c.Query("codes")))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		responses, err := toHTTPEnumDictionaries(items)
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[enumDictionaryResponse]{Items: responses, Total: len(responses)})
	})
}
