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

func registerGameCredentialRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	gameService := gamesvc.NewService(deps.DB)
	api.GET("/games/:id/integration-keys", requirePermission(permissionGameWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		credentials, _, err := gameService.ListIntegrationKeys(toServiceScope(tenantScopeFromContext(c)), id)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		items := make([]*gameCredentialResponse, 0, len(credentials))
		for _, credential := range credentials {
			items = append(items, toHTTPGameCredential(credential))
		}
		writeJSON(c, http.StatusOK, listResponse[*gameCredentialResponse]{Items: items, Total: len(items)})
	})
	api.POST("/games/:id/integration-keys/:keyID/rotate", requirePermission(permissionGameWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		keyID, err := strconv.ParseUint(c.Param("keyID"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		credential, err := gameService.RotateIntegrationKey(toServiceScope(tenantScopeFromContext(c)), id, keyID)
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
			Action:       "rotate_integration_key",
			TargetType:   "game_integration_key",
			TargetID:     strconv.FormatUint(credential.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        toHTTPGameCredential(credential),
		})
		writeJSON(c, http.StatusOK, toHTTPGameCredential(credential))
	})
}


