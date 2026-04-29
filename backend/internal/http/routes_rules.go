package http

import (
	"errors"
	"net/http"
	"strconv"

	"game-admin/backend/internal/domain/model"
	rulesvc "game-admin/backend/internal/services/rule"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerRulesRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	ruleService := rulesvc.NewService(deps.DB)
	api.GET("/rules", requirePermission(permissionRuleRead), func(c *gin.Context) {
		items, err := ruleService.List(toServiceScope(tenantScopeFromContext(c)), rulesvc.ListFilter{
			Status:  c.Query("status"),
			Scope:   c.Query("scope"),
			GameID:  c.Query("gameID"),
			AgentID: c.Query("agentID"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.CommissionRule]{Items: items, Total: len(items)})
	})
	api.POST("/rules", requirePermission(permissionRuleWrite), func(c *gin.Context) {
		var payload rulePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		rule, err := ruleService.Create(toServiceScope(tenantScopeFromContext(c)), rulesvc.CreateInput{
			TenantID:           payload.TenantID,
			BrandID:            payload.BrandID,
			RuleName:           payload.RuleName,
			Scope:              payload.Scope,
			RuleType:           payload.RuleType,
			Status:             payload.Status,
			Priority:           payload.Priority,
			Version:            payload.Version,
			AgentID:            payload.AgentID,
			GameID:             payload.GameID,
			MaxSettlementDepth: payload.MaxSettlementDepth,
			SettlementRate:     payload.SettlementRate,
			CommissionRate:     payload.CommissionRate,
			FixedAmount:        payload.FixedAmount,
			MinAgentLevel:      payload.MinAgentLevel,
			RechargeTypes:      payload.RechargeTypes,
			ActivityTags:       payload.ActivityTags,
			CapAmount:          payload.CapAmount,
			Currency:           payload.Currency,
			EffectiveFrom:      payload.EffectiveFrom,
			Remark:             payload.Remark,
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
			Module:       model.AuditModuleRule,
			Action:       "create",
			TargetType:   "rule",
			TargetID:     strconv.FormatUint(rule.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        rule,
		})
		writeJSON(c, http.StatusCreated, rule)
	})
	api.POST("/rules/:id/publish", requirePermission(permissionRulePublish), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload rulePublishPayload
		_ = c.ShouldBindJSON(&payload)
		before, rule, err := ruleService.Publish(toServiceScope(tenantScopeFromContext(c)), id, payload.PublishedBy)
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
			Module:       model.AuditModuleRule,
			Action:       "publish",
			TargetType:   "rule",
			TargetID:     strconv.FormatUint(rule.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        rule,
		})
		writeJSON(c, http.StatusOK, rule)
	})

}


