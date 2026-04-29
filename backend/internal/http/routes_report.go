package http

import (
	"net/http"
	"strings"

	reportcontract "game-admin/backend/internal/contract/report"
	reportsvc "game-admin/backend/internal/services/report"
	"github.com/gin-gonic/gin"
)

func registerReportRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies) {
	reportService := reportsvc.NewService(deps.DB)
	engine.POST("/internal/contracts/report/team-performance", func(c *gin.Context) {
		var payload reportcontract.TeamPerformanceRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items, err := reportService.TeamPerformance(payload.Scope, strings.TrimSpace(payload.AgentID), strings.TrimSpace(payload.Currency))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		writeJSON(c, http.StatusOK, reportcontract.TeamPerformanceResponse{Items: reportcontract.ToContractItems(items)})
	})
	api.GET("/report/agent-performance", requirePermission(permissionReportRead), func(c *gin.Context) {
		serviceItems, err := reportService.AgentPerformance(toServiceScope(tenantScopeFromContext(c)), strings.TrimSpace(c.Query("agentID")))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]agentPerformanceReportItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPAgentPerformanceItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[agentPerformanceReportItem]{Items: items, Total: len(items)})
	})
	api.GET("/report/game-settlement", requirePermission(permissionReportRead), func(c *gin.Context) {
		serviceItems, err := reportService.GameSettlement(toServiceScope(tenantScopeFromContext(c)), strings.TrimSpace(c.Query("currency")))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]gameSettlementReportItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPGameSettlementItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[gameSettlementReportItem]{Items: items, Total: len(items)})
	})
	api.GET("/report/team-performance", requirePermission(permissionReportRead), func(c *gin.Context) {
		serviceItems, err := reportService.TeamPerformance(toServiceScope(tenantScopeFromContext(c)), strings.TrimSpace(c.Query("agentID")), strings.TrimSpace(c.Query("currency")))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]teamPerformanceReportItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPTeamPerformanceItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[teamPerformanceReportItem]{Items: items, Total: len(items)})
	})
	api.GET("/report/settlement-progress", requirePermission(permissionReportRead), func(c *gin.Context) {
		serviceItems, err := reportService.SettlementProgress(toServiceScope(tenantScopeFromContext(c)), strings.TrimSpace(c.Query("currency")))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]settlementProgressReportItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPSettlementProgressItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[settlementProgressReportItem]{Items: items, Total: len(items)})
	})
	api.GET("/report/data-platform-layers", requirePermission(permissionReportRead), func(c *gin.Context) {
		serviceItems, err := reportService.DataPlatformLayers(
			toServiceScope(tenantScopeFromContext(c)),
			strings.TrimSpace(c.Query("tenantID")),
			strings.TrimSpace(c.Query("brandID")),
		)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]dataPlatformLayerItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, dataPlatformLayerItem{
				Layer:        item.Layer,
				Status:       item.Status,
				SyncMode:     item.SyncMode,
				Description:  item.Description,
				TableCount:   item.TableCount,
				RecordCount:  item.RecordCount,
				LastSyncedAt: item.LastSyncedAt,
			})
		}
		writeJSON(c, http.StatusOK, listResponse[dataPlatformLayerItem]{Items: items, Total: len(items)})
	})
	api.GET("/report/data-platform-metrics", requirePermission(permissionReportRead), func(c *gin.Context) {
		serviceItems, err := reportService.DataPlatformMetrics(
			toServiceScope(tenantScopeFromContext(c)),
			strings.TrimSpace(c.Query("tenantID")),
			strings.TrimSpace(c.Query("brandID")),
		)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]dataPlatformMetricItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, dataPlatformMetricItem{
				Key:         item.Key,
				Value:       item.Value,
				Unit:        item.Unit,
				Description: item.Description,
			})
		}
		writeJSON(c, http.StatusOK, listResponse[dataPlatformMetricItem]{Items: items, Total: len(items)})
	})
}
