package http

import "github.com/gin-gonic/gin"

func registerTenantBrandRelationRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	registerTenantBrandRoutes(api, deps)
	registerRelationRoutes(api, deps)
	registerAgentManagementRoutes(api, deps)
	registerInvitePlayerBindingRoutes(api, deps)
}
