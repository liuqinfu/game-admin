package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"game-admin/backend/internal/domain/model"
	rechargesvc "game-admin/backend/internal/services/recharge"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/gin-gonic/gin"
)

func registerRechargeAdminRoutes(api *gin.RouterGroup, deps RouterDependencies) {
	rechargeService := rechargesvc.NewService(deps.DB)
	api.POST("/recharge/callback", requirePermission(permissionSettlementExecute), func(c *gin.Context) {
		var payload rechargeCallbackPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		if payload.GameID == 0 {
			respondError(c, http.StatusBadRequest, errors.New("gameID is required"))
			return
		}
		if _, err := rechargeService.ValidateCallbackScope(toServiceScope(tenantScopeFromContext(c)), payload.GameID); err != nil {
			respondError(c, http.StatusNotFound, err)
			return
		}
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
			RequestID:          sharedsvc.FirstNonEmpty(strings.TrimSpace(payload.RequestID), requestIDFromContext(c)),
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
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleCommission,
			Action:       "recharge_callback",
			TargetType:   "recharge_order",
			TargetID:     strconv.FormatUint(result.Order.ID, 10),
			RequestID:    requestIDFromContext(c),
			TraceID:      traceIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        result,
		})

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


