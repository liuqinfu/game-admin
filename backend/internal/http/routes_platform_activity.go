package http

import "github.com/gin-gonic/gin"

func registerPlatformAndActivityRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	registerActivityRoutes(api, deps)
	registerPlatformConfigRoutes(api, deps)
}
