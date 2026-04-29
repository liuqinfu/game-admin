package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"game-admin/backend/internal/domain/model"
	agentsvc "game-admin/backend/internal/services/agent"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerAgentManagementRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	agentService := agentsvc.NewService(deps.DB)
	api.POST("/agents", requirePermission(permissionAgentWrite), func(c *gin.Context) {
		if !requireAdminUser(c) {
			return
		}
		var payload agentPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		agent, err := agentService.Create(toServiceScope(tenantScopeFromContext(c)), agentsvc.CreateInput{
			TenantID:              payload.TenantID,
			BrandID:               payload.BrandID,
			AgentNo:               payload.AgentNo,
			Name:                  payload.Name,
			DisplayName:           payload.DisplayName,
			Phone:                 payload.Phone,
			Email:                 payload.Email,
			Status:                payload.Status,
			Level:                 payload.Level,
			SettlementAccountNo:   strings.TrimSpace(payload.SettlementAccountNo),
			SettlementAccountName: strings.TrimSpace(payload.SettlementAccountName),
			Remark:                payload.Remark,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleAgent,
			Action:       "create",
			TargetType:   "agent",
			TargetID:     strconv.FormatUint(agent.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        agent,
		})
		writeJSON(c, http.StatusCreated, agent)
	})
	api.PATCH("/agents/:id/status", requirePermission(permissionAgentWrite), func(c *gin.Context) {
		if !requireAdminUser(c) {
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var body struct {
			Status model.AgentStatus `json:"status"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, agent, err := agentService.UpdateStatus(toServiceScope(tenantScopeFromContext(c)), id, body.Status)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleAgent,
			Action:       "update_status",
			TargetType:   "agent",
			TargetID:     strconv.FormatUint(agent.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        agent,
		})
		writeJSON(c, http.StatusOK, agent)
	})

}


