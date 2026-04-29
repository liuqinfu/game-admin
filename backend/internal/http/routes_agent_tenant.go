package http

import "github.com/gin-gonic/gin"

func registerAgentTenantRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	registerAgentRoutes(api, deps)
	registerTenantBrandRelationRoutes(api, deps)
}
