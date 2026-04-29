package http

import (
	"errors"
	"net/http"

	"game-admin/backend/internal/domain/model"
	auditsvc "game-admin/backend/internal/services/audit"
	"github.com/gin-gonic/gin"
)

func registerAuditRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	service := auditsvc.NewService(deps.DB)
	api.GET("/audit", requirePermission(permissionAuditRead), func(c *gin.Context) {
		items, err := service.List(toServiceScope(tenantScopeFromContext(c)), auditsvc.ListFilter{
			Module:   c.Query("module"),
			Action:   c.Query("action"),
			TargetID: c.Query("targetID"),
		})
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, auditsvc.ErrPlatformScopeRequired) {
				status = http.StatusForbidden
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.OperationAuditLog]{Items: items, Total: len(items)})
	})

}
