package http

import (
	"errors"
	"net/http"

	agentsvc "game-admin/backend/internal/services/agent"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerRelationRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	service := agentsvc.NewService(deps.DB)
	api.GET("/agents/:id/ancestors", requirePermission(permissionAgentRead), func(c *gin.Context) {
		agentID, err := parseAgentIDParam(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		depthRange, err := parseHierarchyDepthRange(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items, err := service.Ancestors(toServiceScope(tenantScopeFromContext(c)), agentID, agentsvc.HierarchyDepthRange{Min: depthRange.Min, Max: depthRange.Max})
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		response := make([]agentHierarchyNode, 0, len(items))
		for _, item := range items {
			response = append(response, agentHierarchyNode(item))
		}
		writeJSON(c, http.StatusOK, listResponse[agentHierarchyNode]{Items: response, Total: len(response)})
	})
	api.GET("/agents/:id/descendants", requirePermission(permissionAgentRead), func(c *gin.Context) {
		agentID, err := parseAgentIDParam(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		depthRange, err := parseHierarchyDepthRange(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items, err := service.Descendants(toServiceScope(tenantScopeFromContext(c)), agentID, agentsvc.HierarchyDepthRange{Min: depthRange.Min, Max: depthRange.Max})
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		response := make([]agentHierarchyNode, 0, len(items))
		for _, item := range items {
			response = append(response, agentHierarchyNode(item))
		}
		writeJSON(c, http.StatusOK, listResponse[agentHierarchyNode]{Items: response, Total: len(response)})
	})
	api.GET("/agents/:id/team-stats", requirePermission(permissionAgentRead), func(c *gin.Context) {
		agentID, err := parseAgentIDParam(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		depthRange, err := parseHierarchyDepthRange(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		stats, err := service.TeamStatsByAgent(toServiceScope(tenantScopeFromContext(c)), agentID, agentsvc.HierarchyDepthRange{Min: depthRange.Min, Max: depthRange.Max})
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		response := agentTeamStatsResponse{
			AgentID:            stats.AgentID,
			DirectDescendants:  stats.DirectDescendants,
			TotalDescendants:   stats.TotalDescendants,
			TotalAncestors:     stats.TotalAncestors,
			MaxDescendantDepth: stats.MaxDescendantDepth,
			MaxAncestorDepth:   stats.MaxAncestorDepth,
			LeafDescendants:    stats.LeafDescendants,
		}
		for _, item := range stats.DepthBreakdown {
			response.DepthBreakdown = append(response.DepthBreakdown, agentDepthStatResponse{Depth: item.Depth, Count: item.Count})
		}
		writeJSON(c, http.StatusOK, response)
	})

}
