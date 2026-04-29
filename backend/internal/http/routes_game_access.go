package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"game-admin/backend/internal/domain/model"
	gamesvc "game-admin/backend/internal/services/game"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerAgentGameAccessRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	gameService := gamesvc.NewService(deps.DB)
	api.GET("/agent-game-access", requirePermission(permissionAgentGameAccessRead), func(c *gin.Context) {
		serviceItems, err := gameService.ListAccess(toServiceScope(tenantScopeFromContext(c)), gamesvc.AccessFilter{
			AgentID: c.Query("agentID"),
			GameID:  c.Query("gameID"),
			Status:  c.Query("status"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		items := make([]agentGameAccessListItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, agentGameAccessListItem{
				AgentGameAccess: item.AgentGameAccess,
				AgentName:       item.AgentName,
				GameCode:        item.GameCode,
				GameName:        item.GameName,
			})
		}
		writeJSON(c, http.StatusOK, listResponse[agentGameAccessListItem]{Items: items, Total: len(items)})
	})
	api.POST("/agent-game-access", requirePermission(permissionAgentGameAccessWrite), func(c *gin.Context) {
		var payload agentGameAccessPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		access, err := gameService.UpsertAccess(toServiceScope(tenantScopeFromContext(c)), gamesvc.AccessInput{
			AgentID: payload.AgentID,
			GameID:  payload.GameID,
			Status:  payload.Status,
			Remark:  payload.Remark,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleGame,
				Action:       "agent_game_access_upsert",
				TargetType:   "agent_game_access",
				TargetID:     strconv.FormatUint(payload.AgentID, 10) + ":" + strconv.FormatUint(payload.GameID, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				After:        payload,
			})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleGame,
			Action:       "agent_game_access_upsert",
			TargetType:   "agent_game_access",
			TargetID:     strconv.FormatUint(access.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        access,
		})
		writeJSON(c, http.StatusCreated, access)
	})
	api.DELETE("/agent-game-access", requirePermission(permissionAgentGameAccessWrite), func(c *gin.Context) {
		agentID, err := strconv.ParseUint(strings.TrimSpace(c.Query("agentID")), 10, 64)
		if err != nil || agentID == 0 {
			respondError(c, http.StatusBadRequest, errors.New("agentID is required"))
			return
		}
		gameID, err := strconv.ParseUint(strings.TrimSpace(c.Query("gameID")), 10, 64)
		if err != nil || gameID == 0 {
			respondError(c, http.StatusBadRequest, errors.New("gameID is required"))
			return
		}
		before, err := gameService.RevokeAccess(toServiceScope(tenantScopeFromContext(c)), agentID, gameID)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleGame,
				Action:       "agent_game_access_revoke",
				TargetType:   "agent_game_access",
				TargetID:     strconv.FormatUint(agentID, 10) + ":" + strconv.FormatUint(gameID, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
			})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleGame,
			Action:       "agent_game_access_revoke",
			TargetType:   "agent_game_access",
			TargetID:     strconv.FormatUint(before.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
		})
		c.Status(http.StatusNoContent)
	})
}


