package http

import (
	"encoding/json"
	"net/http"

	"game-admin/backend/internal/domain/model"
	gametrics "game-admin/backend/internal/metrics"
	"game-admin/backend/internal/routing"
	identitysvc "game-admin/backend/internal/services/identity"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Credentials", "true")
		headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Content-Length, Accept, Origin, X-Requested-With, X-Request-ID, X-Trace-ID, X-Game-Access-Key, X-Game-Timestamp, X-Game-Nonce, X-Game-Signature")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func moduleAccessMiddleware(cfg RouterConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(cfg.EnabledModules) == 0 {
			c.Next()
			return
		}

		match := routing.MatchPath(c.Request.URL.Path)
		if match.Shared || cfg.moduleEnabled(match.Module) {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusNotFound)
	}
}

func NewRouter(deps RouterDependencies, configs ...RouterConfig) *gin.Engine {
	var cfg RouterConfig
	if len(configs) > 0 {
		cfg = configs[0]
	}
	if deps.Metrics == nil {
		deps.Metrics = gametrics.NewRuntime(cfg.ServiceName, "http")
	}
	engine := gin.New()
	engine.Use(gin.Recovery(), corsMiddleware(), requestContextMiddleware(), requestMetricsMiddleware(deps.Metrics))
	engine.Use(moduleAccessMiddleware(cfg))
	registerHealthRoutes(engine, deps)
	registerGameOpenAPIRoutes(engine, deps, cfg)

	api := engine.Group("/api")
	identityService := identitysvc.NewService(deps.DB)
	api.Use(authMiddleware(identityService, func(agentID uint64) ([]uint64, error) {
		return sharedsvc.CurrentAgentScopeIDs(deps.DB, agentID)
	}))
	{
		registerAuthRoutes(engine, api, deps, cfg)
		registerSharedAPIRoutes(api, deps)

		registerAgentTenantRoutes(api, deps)
		registerPlatformAndActivityRoutes(api, deps)
		registerGameRoutes(api, deps)
		registerRulesRoutes(api, deps)
		registerFinanceAuditRiskReportRbacRoutes(engine, api, deps, cfg)

	}

	return engine
}

func cloneActivityRewardRecordWithRuleScope(record model.ActivityRewardRecord) model.ActivityRewardRecord {
	cloned := record
	if cloned.TenantID != nil && cloned.BrandID != nil {
		return cloned
	}
	if len(record.SnapshotPayload) == 0 {
		return cloned
	}
	var snapshot struct {
		TenantID *uint64 `json:"tenantID"`
		BrandID  *uint64 `json:"brandID"`
	}
	if err := json.Unmarshal(record.SnapshotPayload, &snapshot); err != nil {
		return cloned
	}
	if cloned.TenantID == nil && snapshot.TenantID != nil {
		cloned.TenantID = snapshot.TenantID
	}
	if cloned.BrandID == nil && snapshot.BrandID != nil {
		cloned.BrandID = snapshot.BrandID
	}
	return cloned
}

func auditActivityRewardRecordEntry(entry auditEntry) auditEntry {
	cloned := entry
	if afterRecord, ok := entry.After.(model.ActivityRewardRecord); ok {
		cloned.After = cloneActivityRewardRecordWithRuleScope(afterRecord)
	}
	if beforeRecord, ok := entry.Before.(model.ActivityRewardRecord); ok {
		cloned.Before = cloneActivityRewardRecordWithRuleScope(beforeRecord)
	}
	return cloned
}
