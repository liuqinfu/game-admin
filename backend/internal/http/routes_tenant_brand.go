package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"game-admin/backend/internal/domain/model"
	tenantsvc "game-admin/backend/internal/services/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerTenantBrandRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	tenantService := tenantsvc.NewService(deps.DB)
	api.GET("/tenants", requirePermission(permissionTenantRead), func(c *gin.Context) {
		items, err := tenantService.ListTenants(toServiceScope(tenantScopeFromContext(c)))
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.Tenant]{Items: items, Total: len(items)})
	})
	api.POST("/tenants", requirePermission(permissionTenantWrite), func(c *gin.Context) {
		var payload tenantPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		tenant, err := tenantService.CreateTenant(toServiceScope(tenantScopeFromContext(c)), tenantsvc.TenantInput{
			Code:        payload.Code,
			Name:        payload.Name,
			DisplayName: payload.DisplayName,
			Status:      payload.Status,
			Remark:      payload.Remark,
		})
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "platform scope") {
				status = http.StatusForbidden
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "tenant_create", TargetType: "tenant", TargetID: strconv.FormatUint(tenant.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: tenant})
		writeJSON(c, http.StatusCreated, tenant)
	})
	api.PATCH("/tenants/:id/status", requirePermission(permissionTenantWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload tenantStatusPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, tenant, err := tenantService.UpdateTenantStatus(toServiceScope(tenantScopeFromContext(c)), id, payload.Status)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "tenant_update_status", TargetType: "tenant", TargetID: strconv.FormatUint(tenant.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: before, After: tenant})
		writeJSON(c, http.StatusOK, tenant)
	})
	api.DELETE("/tenants/:id", requirePermission(permissionTenantWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		deleted, err := tenantService.DeleteTenant(toServiceScope(tenantScopeFromContext(c)), id)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "tenant_delete", TargetType: "tenant", TargetID: strconv.FormatUint(deleted.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: deleted})
		c.Status(http.StatusNoContent)
	})

	api.GET("/brands", requirePermission(permissionBrandRead), func(c *gin.Context) {
		var tenantID *uint64
		if tenantIDText := strings.TrimSpace(c.Query("tenantID")); tenantIDText != "" {
			value, err := strconv.ParseUint(tenantIDText, 10, 64)
			if err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			tenantID = &value
		}
		items, err := tenantService.ListBrands(toServiceScope(tenantScopeFromContext(c)), tenantID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.Brand]{Items: items, Total: len(items)})
	})
	api.POST("/brands", requirePermission(permissionBrandWrite), func(c *gin.Context) {
		var payload brandPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		brand, err := tenantService.CreateBrand(toServiceScope(tenantScopeFromContext(c)), tenantsvc.BrandInput{
			TenantID:    payload.TenantID,
			Code:        payload.Code,
			Name:        payload.Name,
			DisplayName: payload.DisplayName,
			Status:      payload.Status,
			Domain:      payload.Domain,
			IsDefault:   payload.IsDefault,
			Remark:      payload.Remark,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "brand_create", TargetType: "brand", TargetID: strconv.FormatUint(brand.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: brand})
		writeJSON(c, http.StatusCreated, brand)
	})
	api.PATCH("/brands/:id/status", requirePermission(permissionBrandWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload brandStatusPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, brand, err := tenantService.UpdateBrandStatus(toServiceScope(tenantScopeFromContext(c)), id, payload.Status)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "brand_update_status", TargetType: "brand", TargetID: strconv.FormatUint(brand.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: before, After: brand})
		writeJSON(c, http.StatusOK, brand)
	})
	api.DELETE("/brands/:id", requirePermission(permissionBrandWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		deleted, err := tenantService.DeleteBrand(toServiceScope(tenantScopeFromContext(c)), id)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "brand_delete", TargetType: "brand", TargetID: strconv.FormatUint(deleted.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: deleted})
		c.Status(http.StatusNoContent)
	})

}


