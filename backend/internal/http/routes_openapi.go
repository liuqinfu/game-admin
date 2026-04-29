package http

import (
	"errors"
	"net/http"

	"game-admin/backend/internal/domain/model"
	agentsvc "game-admin/backend/internal/services/agent"
	gamesvc "game-admin/backend/internal/services/game"
	rechargesvc "game-admin/backend/internal/services/recharge"
	rulesvc "game-admin/backend/internal/services/rule"
	tenantsvc "game-admin/backend/internal/services/tenant"
	"github.com/gin-gonic/gin"
)

func registerGameOpenAPIRoutes(engine *gin.Engine, deps RouterDependencies, cfg RouterConfig) {
	if !cfg.moduleEnabled(RouterModuleOpenAPI) {
		return
	}

	gameService := gamesvc.NewService(deps.DB)
	openGameAPI := engine.Group("/openapi/game")
	openGameAPI.Use(gameOpenAPIAuthMiddleware(gameService))
	openGameAPI.Use(openAPIRateLimitMiddleware(deps.Redis))
	agentService := agentsvc.NewService(deps.DB)
	rechargeService := rechargesvc.NewService(deps.DB)
	ruleService := rulesvc.NewService(deps.DB)
	tenantService := tenantsvc.NewService(deps.DB)
	{
		openGameAPI.POST("/players", requireGameOpenAPIScope("player:create"), func(c *gin.Context) {
			ctx := gameOpenAPIContextFromGin(c)
			var payload playerPayload
			if err := c.ShouldBindJSON(&payload); err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			player, err := agentService.CreatePlayerForGame(ctx.Game, agentsvc.PlayerCreateInput{
				PlayerNo:       payload.PlayerNo,
				PlatformUserID: payload.PlatformUserID,
				Nickname:       payload.Nickname,
				Phone:          payload.Phone,
				CountryCode:    payload.CountryCode,
				Currency:       payload.Currency,
			})
			if err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			writeJSON(c, http.StatusCreated, player)
		})

		openGameAPI.POST("/players/register-with-invite", requireGameOpenAPIScope("player:register_with_invite"), func(c *gin.Context) {
			ctx := gameOpenAPIContextFromGin(c)
			var payload registerWithInvitePayload
			if err := c.ShouldBindJSON(&payload); err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			player, binding, err := agentService.RegisterWithInviteForGame(ctx.Game, agentsvc.RegisterWithInviteInput{
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
				respondError(c, http.StatusBadRequest, err)
				return
			}
			writeJSON(c, http.StatusCreated, gin.H{"player": player, "binding": binding})
		})

		openGameAPI.GET("/agent-game-access", requireGameOpenAPIScope("game_access:read"), func(c *gin.Context) {
			ctx := gameOpenAPIContextFromGin(c)
			items, err := gameService.ListAccessForGame(ctx.Game, gamesvc.OpenAPIAccessFilter{
				AgentID: c.Query("agentID"),
				Status:  c.Query("status"),
			})
			if err != nil {
				respondError(c, http.StatusInternalServerError, err)
				return
			}
			responses := make([]agentGameAccessListItem, 0, len(items))
			for _, item := range items {
				responses = append(responses, agentGameAccessListItem{
					AgentGameAccess: item.AgentGameAccess,
					AgentName:       item.AgentName,
					GameCode:        item.GameCode,
					GameName:        item.GameName,
				})
			}
			writeJSON(c, http.StatusOK, listResponse[agentGameAccessListItem]{Items: responses, Total: len(responses)})
		})

		openGameAPI.GET("/rules", requireGameOpenAPIScope("rule:read"), func(c *gin.Context) {
			ctx := gameOpenAPIContextFromGin(c)
			items, err := ruleService.ListForGame(ctx.Game, rulesvc.OpenAPIListFilter{
				Status:  c.Query("status"),
				Scope:   c.Query("scope"),
				AgentID: c.Query("agentID"),
			})
			if err != nil {
				respondError(c, http.StatusInternalServerError, err)
				return
			}
			writeJSON(c, http.StatusOK, listResponse[model.CommissionRule]{Items: items, Total: len(items)})
		})

		openGameAPI.GET("/enum-dictionaries", func(c *gin.Context) {
			items, err := tenantService.ListEnumDictionaries(parseEnumDictionaryCodes(c.Query("codes")))
			if err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			responses, err := toHTTPEnumDictionaries(items)
			if err != nil {
				respondError(c, http.StatusInternalServerError, err)
				return
			}
			writeJSON(c, http.StatusOK, listResponse[enumDictionaryResponse]{Items: responses, Total: len(responses)})
		})

		openGameAPI.POST("/recharge/callback", requireGameOpenAPIScope("recharge:callback"), func(c *gin.Context) {
			ctx := gameOpenAPIContextFromGin(c)
			var payload rechargeCallbackPayload
			if err := c.ShouldBindJSON(&payload); err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			if payload.GameID != 0 && payload.GameID != ctx.Game.ID {
				respondError(c, http.StatusBadRequest, errors.New("gameID does not match access key"))
				return
			}
			payload.GameID = ctx.Game.ID
			result, err := rechargeService.ProcessCallback(rechargesvc.CallbackInput{
				OrderNo:            payload.OrderNo,
				ExternalOrderNo:    payload.ExternalOrderNo,
				PlayerID:           payload.PlayerID,
				GameID:             payload.GameID,
				Amount:             payload.Amount,
				PaidAmount:         payload.PaidAmount,
				PaymentChannelCost: payload.PaymentChannelCost,
				GrossProfitAmount:  payload.GrossProfitAmount,
				Currency:           payload.Currency,
				Status:             payload.Status,
				PaidAt:             payload.PaidAt,
				CallbackAt:         payload.CallbackAt,
				Channel:            payload.Channel,
				RechargeType:       payload.RechargeType,
				RequestID:          payload.RequestID,
				CallbackSource:     payload.CallbackSource,
				Signature:          payload.Signature,
				IdempotencyKey:     payload.IdempotencyKey,
				ActivityTags:       payload.ActivityTags,
				Remark:             payload.Remark,
			}, c.ClientIP())
			if err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			status := http.StatusCreated
			if result.Duplicate {
				status = http.StatusOK
			}
			writeJSON(c, status, rechargeCallbackResult{
				Order:       result.Order,
				Commissions: result.Commissions,
				Ledgers:     result.Ledgers,
				Duplicate:   result.Duplicate,
				Frozen:      result.Frozen,
			})
		})
	}
}
