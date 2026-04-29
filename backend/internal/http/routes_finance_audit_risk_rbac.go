package http

import "github.com/gin-gonic/gin"

func registerFinanceAuditRiskReportRbacRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	registerFinanceRoutes(engine, api, deps, cfg)
	registerAuditRoutes(api, deps)
	registerRiskRoutes(engine, api, deps, cfg)
	registerReportRoutes(engine, api, deps)
	registerRBACRoutes(api, deps)
	registerRechargeAdminRoutes(api, deps)
}
