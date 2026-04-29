package http

import (
	"errors"
	"net/http"
	"strconv"

	"game-admin/backend/internal/domain/model"
	gamesvc "game-admin/backend/internal/services/game"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerGameRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	registerGameCatalogRoutes(api, deps)
	registerGameCredentialRoutes(api, deps)
	registerAgentGameAccessRoutes(api, deps)
}

func registerGameCatalogRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	gameService := gamesvc.NewService(deps.DB)
	api.GET("/games", requirePermission(permissionGameRead), func(c *gin.Context) {
		items, err := gameService.List(toServiceScope(tenantScopeFromContext(c)), gamesvc.ListFilter{
			Status:  c.Query("status"),
			Keyword: c.Query("keyword"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.Game]{Items: items, Total: len(items)})
	})
	api.POST("/games", requirePermission(permissionGameWrite), func(c *gin.Context) {
		var payload gamePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		game, credential, err := gameService.Create(toServiceScope(tenantScopeFromContext(c)), gamesvc.CreateInput{
			TenantID:    payload.TenantID,
			BrandID:     payload.BrandID,
			GameCode:    payload.GameCode,
			Name:        payload.Name,
			Vendor:      payload.Vendor,
			Category:    payload.Category,
			Status:      payload.Status,
			IsAgentable: payload.IsAgentable,
			Sort:        payload.Sort,
			Remark:      payload.Remark,
		}, operatorNameFromContext(c))
		if err != nil {
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleGame,
				Action:       "create",
				TargetType:   "game",
				TargetID:     game.GameCode,
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				After:        game,
			})
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
			Module:       model.AuditModuleGame,
			Action:       "create",
			TargetType:   "game",
			TargetID:     strconv.FormatUint(game.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        game,
		})
		writeJSON(c, http.StatusCreated, gameCreateResponse{Game: game, IntegrationCredential: toHTTPGameCredential(credential)})
	})
	api.PUT("/games/:id", requirePermission(permissionGameWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload gamePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, game, err := gameService.Update(toServiceScope(tenantScopeFromContext(c)), id, gamesvc.UpdateInput{
			GameCode:    payload.GameCode,
			Name:        payload.Name,
			Vendor:      payload.Vendor,
			Category:    payload.Category,
			Status:      payload.Status,
			IsAgentable: payload.IsAgentable,
			Sort:        payload.Sort,
			Remark:      payload.Remark,
		})
		if err != nil {
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleGame,
				Action:       "update",
				TargetType:   "game",
				TargetID:     strconv.FormatUint(game.ID, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Before:       before,
				After:        payload,
			})
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
			Module:       model.AuditModuleGame,
			Action:       "update",
			TargetType:   "game",
			TargetID:     strconv.FormatUint(game.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        game,
		})
		writeJSON(c, http.StatusOK, game)
	})
	api.PATCH("/games/:id/status", requirePermission(permissionGamePublish), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload gameStatusPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, game, err := gameService.UpdateStatus(toServiceScope(tenantScopeFromContext(c)), id, payload.Status)
		if err != nil {
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleGame,
				Action:       "update_status",
				TargetType:   "game",
				TargetID:     strconv.FormatUint(game.ID, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Before:       before,
				After:        payload,
			})
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
			Module:       model.AuditModuleGame,
			Action:       "update_status",
			TargetType:   "game",
			TargetID:     strconv.FormatUint(game.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        game,
		})
		writeJSON(c, http.StatusOK, game)
	})
	api.DELETE("/games/:id", requirePermission(permissionGameWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		deleted, err := gameService.Delete(toServiceScope(tenantScopeFromContext(c)), id)
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
			Module:       model.AuditModuleGame,
			Action:       "delete",
			TargetType:   "game",
			TargetID:     strconv.FormatUint(deleted.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       deleted,
		})
		c.Status(http.StatusNoContent)
	})
}


