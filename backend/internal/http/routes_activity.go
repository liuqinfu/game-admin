package http

import (
	"errors"
	"net/http"
	"strconv"

	"game-admin/backend/internal/domain/model"
	activitysvc "game-admin/backend/internal/services/activity"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func activityRewardRecordResponse(record model.ActivityRewardRecord) activityRewardRecordView {
	userID := record.PlayerID
	return activityRewardRecordView{
		ID:           record.ID,
		RecordNo:     record.RecordNo,
		RuleID:       record.RuleID,
		RuleName:     record.RuleName,
		UserID:       userID,
		PlayerID:     record.PlayerID,
		AgentID:      record.AgentID,
		RewardType:   record.RewardType,
		RewardValue:  record.RewardValue,
		Currency:     record.Currency,
		ActivityType: record.ActivityType,
		Status:       record.Status,
		GrantedAt:    record.GrantedAt,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		Remark:       record.Remark,
	}
}

func registerActivityRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	service := activitysvc.NewService(deps.DB)

	api.GET("/activity-reward-rules", requirePermission(permissionActivityRewardRead), func(c *gin.Context) {
		items, err := service.ListRules(toServiceScope(tenantScopeFromContext(c)), activitysvc.RuleListFilter{
			TenantID:     c.Query("tenantID"),
			BrandID:      c.Query("brandID"),
			Status:       c.Query("status"),
			ActivityType: c.Query("activityType"),
			Keyword:      c.Query("keyword"),
		})
		if err != nil {
			statusCode := http.StatusInternalServerError
			if err.Error() == "invalid tenantID" || err.Error() == "invalid brandID" {
				statusCode = http.StatusBadRequest
			}
			respondError(c, statusCode, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.ActivityRewardRule]{Items: items, Total: len(items)})
	})
	api.POST("/activity-reward-rules", requirePermission(permissionActivityRewardWrite), func(c *gin.Context) {
		var payload activityRewardRulePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		rule, err := service.CreateRule(toServiceScope(tenantScopeFromContext(c)), activitysvc.RuleCreateInput{
			TenantID:     payload.TenantID,
			BrandID:      payload.BrandID,
			Name:         payload.Name,
			ActivityType: payload.ActivityType,
			RewardType:   payload.RewardType,
			Status:       payload.Status,
			RewardValue:  payload.RewardValue,
			Currency:     payload.Currency,
			TriggerValue: payload.TriggerValue,
			DailyLimit:   payload.DailyLimit,
			TotalLimit:   payload.TotalLimit,
			StartAt:      payload.StartAt,
			EndAt:        payload.EndAt,
			Remark:       payload.Remark,
		})
		if err != nil {
			statusCode := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				statusCode = http.StatusNotFound
			}
			respondError(c, statusCode, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "activity_reward_rule_create", TargetType: "activity_reward_rule", TargetID: strconv.FormatUint(rule.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: rule})
		writeJSON(c, http.StatusCreated, rule)
	})
	api.PATCH("/activity-reward-rules/:id/status", requirePermission(permissionActivityRewardPublish), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload activityRewardRuleStatusPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, rule, err := service.UpdateRuleStatus(toServiceScope(tenantScopeFromContext(c)), id, payload.Status)
		if err != nil {
			statusCode := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				statusCode = http.StatusNotFound
			}
			respondError(c, statusCode, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "activity_reward_rule_update_status", TargetType: "activity_reward_rule", TargetID: strconv.FormatUint(rule.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: before, After: rule})
		writeJSON(c, http.StatusOK, rule)
	})
	api.GET("/activity-reward-records", requirePermission(permissionActivityRewardRead), func(c *gin.Context) {
		items, err := service.ListRecords(toServiceScope(tenantScopeFromContext(c)), activitysvc.RecordListFilter{
			TenantID: c.Query("tenantID"),
			BrandID:  c.Query("brandID"),
			Status:   c.Query("status"),
			Keyword:  c.Query("keyword"),
		})
		if err != nil {
			statusCode := http.StatusInternalServerError
			if err.Error() == "invalid tenantID" || err.Error() == "invalid brandID" {
				statusCode = http.StatusBadRequest
			}
			respondError(c, statusCode, err)
			return
		}
		responses := make([]activityRewardRecordView, 0, len(items))
		for _, item := range items {
			responses = append(responses, activityRewardRecordResponse(item))
		}
		writeJSON(c, http.StatusOK, listResponse[activityRewardRecordView]{Items: responses, Total: len(responses)})
	})
	api.POST("/activity-reward-rules/:id/generate", requirePermission(permissionActivityRewardWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload activityRewardRecordGeneratePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		record, statusCode, err := service.GenerateRecord(toServiceScope(tenantScopeFromContext(c)), id, activitysvc.RecordGenerateInput{
			PlayerID:      payload.PlayerID,
			UserID:        payload.UserID,
			AgentID:       payload.AgentID,
			ReferenceType: payload.ReferenceType,
			ReferenceID:   payload.ReferenceID,
			Remark:        payload.Remark,
		})
		if err != nil {
			respondError(c, statusCode, err)
			return
		}
		_ = logAudit(deps, auditActivityRewardRecordEntry(auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "activity_reward_record_generate", TargetType: "activity_reward_record", TargetID: strconv.FormatUint(record.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: record}))
		writeJSON(c, http.StatusCreated, activityRewardRecordResponse(record))
	})
	api.POST("/activity-reward-records/:id/reverse", requirePermission(permissionActivityRewardPublish), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload activityRewardRecordReversePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, record, statusCode, err := service.ReverseRecord(toServiceScope(tenantScopeFromContext(c)), id, activitysvc.RecordReverseInput{Remark: payload.Remark})
		if err != nil {
			respondError(c, statusCode, err)
			return
		}
		_ = logAudit(deps, auditActivityRewardRecordEntry(auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "activity_reward_record_reverse", TargetType: "activity_reward_record", TargetID: strconv.FormatUint(record.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: before, After: record}))
		writeJSON(c, http.StatusOK, activityRewardRecordResponse(record))
	})

}


