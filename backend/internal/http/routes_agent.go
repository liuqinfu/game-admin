package http

import (
	"net/http"

	"game-admin/backend/internal/domain/model"
	agentsvc "game-admin/backend/internal/services/agent"
	"github.com/gin-gonic/gin"
)

func registerAgentRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	agentService := agentsvc.NewService(deps.DB)
	api.GET("/agents", requirePermission(permissionAgentRead), func(c *gin.Context) {
		items, err := agentService.ListAgents(toServiceScope(tenantScopeFromContext(c)), agentsvc.AgentListFilter{
			TenantID: c.Query("tenantID"),
			BrandID:  c.Query("brandID"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.Agent]{Items: items, Total: len(items)})
	})

}
