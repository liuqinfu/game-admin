package http

import (
	"errors"
	"net/http"
	"strconv"

	"game-admin/backend/internal/domain/model"
	tenantsvc "game-admin/backend/internal/services/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerPlatformConfigRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	service := tenantsvc.NewService(deps.DB)
	api.GET("/platform-configs", requirePermission(permissionPlatformConfigRead), func(c *gin.Context) {
		items, err := service.ListPlatformConfigs(toServiceScope(tenantScopeFromContext(c)), tenantsvc.PlatformConfigListFilter{
			TenantCode: c.Query("tenantCode"),
			TenantID:   c.Query("tenantID"),
			BrandID:    c.Query("brandID"),
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		responses := make([]platformConfigListItem, 0, len(items))
		for _, item := range items {
			responses = append(responses, platformConfigListItem{
				ID:          item.ID,
				TenantID:    item.TenantID,
				TenantCode:  item.TenantCode,
				BrandID:     item.BrandID,
				BrandCode:   item.BrandCode,
				Key:         item.Key,
				Value:       item.Value,
				Description: item.Description,
			})
		}
		writeJSON(c, http.StatusOK, listResponse[platformConfigListItem]{Items: responses, Total: len(responses)})
	})
	api.POST("/platform-configs", requirePermission(permissionPlatformConfigWrite), func(c *gin.Context) {
		var payload platformConfigPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		configRecord, err := service.CreatePlatformConfig(toServiceScope(tenantScopeFromContext(c)), tenantsvc.PlatformConfigInput{
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			Key:         payload.Key,
			Value:       payload.Value,
			Description: payload.Description,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			} else if isUniqueConstraintError(err) {
				status = http.StatusConflict
			}
			_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "platform_config_create", TargetType: "platform_config", TargetID: strconv.FormatUint(configRecord.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultFailed, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: payload, ErrorMessage: err.Error()})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "platform_config_create", TargetType: "platform_config", TargetID: strconv.FormatUint(configRecord.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: configRecord})
		writeJSON(c, http.StatusCreated, configRecord)
	})
	api.PUT("/platform-configs/:id", requirePermission(permissionPlatformConfigWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload platformConfigPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, configRecord, err := service.UpdatePlatformConfig(toServiceScope(tenantScopeFromContext(c)), id, tenantsvc.PlatformConfigInput{
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			Key:         payload.Key,
			Value:       payload.Value,
			Description: payload.Description,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "platform_config_update", TargetType: "platform_config", TargetID: strconv.FormatUint(id, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultFailed, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: payload, ErrorMessage: err.Error()})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "platform_config_update", TargetType: "platform_config", TargetID: strconv.FormatUint(configRecord.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: before, After: configRecord})
		writeJSON(c, http.StatusOK, configRecord)
	})
}


