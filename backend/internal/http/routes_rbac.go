package http

import (
	"net/http"
	"strconv"

	"game-admin/backend/internal/domain/model"
	identitysvc "game-admin/backend/internal/services/identity"
	"github.com/gin-gonic/gin"
)

func registerRBACRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	identityService := identitysvc.NewService(deps.DB)
	api.GET("/rbac/permissions", requirePermission(permissionRBACPermissionsView), func(c *gin.Context) {
		items, err := identityService.ListPermissions()
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.AdminPermission]{Items: items, Total: len(items)})
	})
	api.GET("/rbac/roles", requirePermission(permissionRBACRolesView), func(c *gin.Context) {
		items, err := identityService.ListRoles(toServiceScope(tenantScopeFromContext(c)))
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		responses := make([]rbacRoleResponse, 0, len(items))
		for _, item := range items {
			responses = append(responses, rbacRoleResponse{AdminRole: item.AdminRole, Permissions: item.Permissions})
		}
		writeJSON(c, http.StatusOK, listResponse[rbacRoleResponse]{Items: responses, Total: len(responses)})
	})
	api.GET("/rbac/users", requirePermission(permissionRBACUsersView), func(c *gin.Context) {
		items, err := identityService.ListUsers(toServiceScope(tenantScopeFromContext(c)))
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		responses := make([]rbacUserResponse, 0, len(items))
		for _, item := range items {
			responses = append(responses, toHTTPRBACUser(item))
		}
		writeJSON(c, http.StatusOK, listResponse[rbacUserResponse]{Items: responses, Total: len(responses)})
	})
	api.POST("/rbac/users", requirePermission(permissionRBACUsersWrite), func(c *gin.Context) {
		var payload rbacUserPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		user, err := identityService.CreateUser(toServiceScope(tenantScopeFromContext(c)), identitysvc.UserInput{
			Username:    payload.Username,
			Password:    payload.Password,
			DisplayName: payload.DisplayName,
			Status:      payload.Status,
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			AgentID:     payload.AgentID,
			Roles:       payload.Roles,
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		response := toHTTPRBACUser(user)
		_ = logAudit(deps, auditEntry{OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "rbac_user_create", TargetType: "admin_user", TargetID: strconv.FormatUint(user.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: response})
		writeJSON(c, http.StatusCreated, response)
	})
	api.PUT("/rbac/users/:id", requirePermission(permissionRBACUsersWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload rbacUserPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, user, err := identityService.UpdateUser(toServiceScope(tenantScopeFromContext(c)), id, identitysvc.UserInput{
			Username:    payload.Username,
			Password:    payload.Password,
			DisplayName: payload.DisplayName,
			Status:      payload.Status,
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			AgentID:     payload.AgentID,
			Roles:       payload.Roles,
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		beforeResponse := toHTTPRBACUser(before)
		response := toHTTPRBACUser(user)
		_ = logAudit(deps, auditEntry{OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "rbac_user_update", TargetType: "admin_user", TargetID: strconv.FormatUint(user.ID, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: beforeResponse, After: response})
		writeJSON(c, http.StatusOK, response)
	})
	api.DELETE("/rbac/users/:id", requirePermission(permissionRBACUsersWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, err := identityService.DeleteUser(toServiceScope(tenantScopeFromContext(c)), id)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		_ = logAudit(deps, auditEntry{OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAgent, Action: "rbac_user_delete", TargetType: "admin_user", TargetID: strconv.FormatUint(id, 10), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Before: toHTTPRBACUser(before)})
		c.Status(http.StatusNoContent)
	})
	api.POST("/rbac/roles", requirePermission(permissionRBACRolesWrite), func(c *gin.Context) {
		var payload rbacRolePayload

		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		role, err := identityService.CreateRole(toServiceScope(tenantScopeFromContext(c)), identitysvc.RoleInput{
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			Code:        payload.Code,
			Name:        payload.Name,
			Permissions: payload.Permissions,
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		response := rbacRoleResponse{AdminRole: role.AdminRole, Permissions: role.Permissions}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleAgent,
			Action:       "rbac_role_create",
			TargetType:   "admin_role",
			TargetID:     strconv.FormatUint(role.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        response,
		})
		writeJSON(c, http.StatusCreated, response)
	})
	api.PUT("/rbac/roles/:id", requirePermission(permissionRBACRolesWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload rbacRolePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, role, err := identityService.UpdateRole(toServiceScope(tenantScopeFromContext(c)), id, identitysvc.RoleInput{
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			Code:        payload.Code,
			Name:        payload.Name,
			Permissions: payload.Permissions,
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		beforeResponse := rbacRoleResponse{AdminRole: before.AdminRole, Permissions: before.Permissions}
		response := rbacRoleResponse{AdminRole: role.AdminRole, Permissions: role.Permissions}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleAgent,
			Action:       "rbac_role_update",
			TargetType:   "admin_role",
			TargetID:     strconv.FormatUint(role.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       beforeResponse,
			After:        response,
		})
		writeJSON(c, http.StatusOK, response)
	})
	api.DELETE("/rbac/roles/:id", requirePermission(permissionRBACRolesWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, err := identityService.DeleteRole(toServiceScope(tenantScopeFromContext(c)), id)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		beforeResponse := rbacRoleResponse{AdminRole: before.AdminRole, Permissions: before.Permissions}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleAgent,
			Action:       "rbac_role_delete",
			TargetType:   "admin_role",
			TargetID:     strconv.FormatUint(id, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       beforeResponse,
		})
		c.Status(http.StatusNoContent)
	})

}


