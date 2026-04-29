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

func registerInvitePlayerBindingRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	service := agentsvc.NewService(deps.DB)
	api.GET("/invite-codes", requirePermission(permissionInviteCodeManage), func(c *gin.Context) {
		items, err := service.ListInviteCodes(toServiceScope(tenantScopeFromContext(c)), agentsvc.InviteCodeListFilter{
			AgentID: c.Query("agentID"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.InviteCode]{Items: items, Total: len(items)})
	})
	api.POST("/invite-codes", requirePermission(permissionInviteCodeManage), func(c *gin.Context) {
		var payload inviteCodePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		inviteCode, err := service.CreateInviteCode(toServiceScope(tenantScopeFromContext(c)), agentsvc.InviteCodeCreateInput{
			AgentID:      payload.AgentID,
			Code:         payload.Code,
			Status:       payload.Status,
			Channel:      payload.Channel,
			Remark:       payload.Remark,
			MaxUseCount:  payload.MaxUseCount,
			IsPrimary:    payload.IsPrimary,
			ValidFrom:    payload.ValidFrom,
			ValidTo:      payload.ValidTo,
			ChannelScope: payload.ChannelScope,
			GameScope:    payload.GameScope,
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
			Action:       "invite_code_create",
			TargetType:   "invite_code",
			TargetID:     strconv.FormatUint(inviteCode.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        inviteCode,
		})
		writeJSON(c, http.StatusCreated, inviteCode)
	})

	api.GET("/players", requirePermission(permissionPlayerRead), func(c *gin.Context) {
		items, err := service.ListPlayers(toServiceScope(tenantScopeFromContext(c)))
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.Player]{Items: items, Total: len(items)})
	})
	api.POST("/players", requirePermission(permissionPlayerWrite), func(c *gin.Context) {
		var payload playerPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		player, err := service.CreatePlayer(toServiceScope(tenantScopeFromContext(c)), agentsvc.PlayerCreateInput{
			TenantID:       payload.TenantID,
			BrandID:        payload.BrandID,
			PlayerNo:       payload.PlayerNo,
			PlatformUserID: payload.PlatformUserID,
			Nickname:       payload.Nickname,
			Phone:          payload.Phone,
			CountryCode:    payload.CountryCode,
			Currency:       payload.Currency,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusCreated, player)
	})
	api.POST("/players/register-with-invite", requirePermission(permissionPlayerWrite), func(c *gin.Context) {
		var payload registerWithInvitePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		player, binding, err := service.RegisterWithInvite(toServiceScope(tenantScopeFromContext(c)), agentsvc.RegisterWithInviteInput{
			PlayerNo:       payload.PlayerNo,
			PlatformUserID: payload.PlatformUserID,
			Nickname:       payload.Nickname,
			Phone:          payload.Phone,
			CountryCode:    payload.CountryCode,
			Currency:       payload.Currency,
			InviteCode:     payload.InviteCode,
			Remark:         payload.Remark,
		})
		if err != nil {
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleBinding,
				Action:       "register_with_invite",
				TargetType:   "binding",
				TargetID:     strings.TrimSpace(payload.PlatformUserID),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				After:        payload,
			})
			respondError(c, http.StatusBadRequest, err)
			return
		}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleBinding,
			Action:       "register_with_invite",
			TargetType:   "binding",
			TargetID:     strconv.FormatUint(binding.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After: gin.H{
				"player":  player,
				"binding": binding,
			},
		})
		writeJSON(c, http.StatusCreated, gin.H{"player": player, "binding": binding})
	})

	api.GET("/bindings", requirePermission(permissionBindingManage), func(c *gin.Context) {
		items, err := service.ListBindings(toServiceScope(tenantScopeFromContext(c)))
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.Binding]{Items: items, Total: len(items)})
	})
	api.GET("/binding-history", requirePermission(permissionBindingManage), func(c *gin.Context) {
		items, err := service.ListBindingHistory(toServiceScope(tenantScopeFromContext(c)), agentsvc.BindingHistoryListFilter{
			PlayerID: c.Query("playerID"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.BindingHistory]{Items: items, Total: len(items)})
	})
	api.POST("/bindings", requirePermission(permissionBindingManage), func(c *gin.Context) {
		var payload bindingPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}

		binding, err := service.CreateBinding(toServiceScope(tenantScopeFromContext(c)), agentsvc.BindingCreateInput{
			PlayerID: payload.PlayerID,
			Code:     payload.Code,
			Remark:   payload.Remark,
		})
		if err != nil {
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleBinding,
				Action:       "create",
				TargetType:   "binding",
				TargetID:     strconv.FormatUint(payload.PlayerID, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				After:        payload,
			})
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound),
				errors.Is(err, agentsvc.ErrAgentInviteRestricted),
				errors.Is(err, agentsvc.ErrInviteCodeUnavailable),
				errors.Is(err, agentsvc.ErrDuplicateBinding):
				respondError(c, http.StatusBadRequest, err)
			default:
				respondError(c, http.StatusInternalServerError, err)
			}
			return
		}

		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleBinding,
			Action:       "create",
			TargetType:   "binding",
			TargetID:     strconv.FormatUint(binding.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        binding,
		})
		writeJSON(c, http.StatusCreated, binding)
	})

	api.GET("/agent-invite-applications", requirePermission(permissionAgentRead), func(c *gin.Context) {
		items, err := service.ListInviteApplications(toServiceScope(tenantScopeFromContext(c)), agentsvc.InviteApplicationListFilter{
			Status: c.Query("status"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.AgentInviteApplication]{Items: items, Total: len(items)})
	})
	api.POST("/agent-invite-applications", requirePermission(permissionAgentWrite), func(c *gin.Context) {
		var payload agentInviteApplicationPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		application, err := service.SubmitInviteApplication(toServiceScope(tenantScopeFromContext(c)), agentsvc.InviteApplicationCreateInput{
			ApplicantAgentID: payload.ApplicantAgentID,
			InviteCode:       payload.InviteCode,
			ApplyRemark:      payload.ApplyRemark,
		})
		if err != nil {
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleAgent,
				Action:       "invite_apply",
				TargetType:   "agent_invite_application",
				TargetID:     strconv.FormatUint(payload.ApplicantAgentID, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
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
			Module:       model.AuditModuleAgent,
			Action:       "invite_apply",
			TargetType:   "agent_invite_application",
			TargetID:     strconv.FormatUint(application.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        application,
		})
		writeJSON(c, http.StatusCreated, application)
	})
	api.POST("/agent-invite-applications/:id/audit", requirePermission(permissionAgentWrite), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload agentInviteApplicationAuditPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		application, before, err := service.AuditInviteApplication(toServiceScope(tenantScopeFromContext(c)), id, agentsvc.InviteApplicationAuditInput{
			Status:      payload.Status,
			AuditRemark: payload.AuditRemark,
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
				Module:       model.AuditModuleAgent,
				Action:       "invite_audit",
				TargetType:   "agent_invite_application",
				TargetID:     strconv.FormatUint(id, 10),
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
			Module:       model.AuditModuleAgent,
			Action:       "invite_audit",
			TargetType:   "agent_invite_application",
			TargetID:     strconv.FormatUint(application.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        application,
		})
		writeJSON(c, http.StatusOK, application)
	})
}


