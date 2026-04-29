package http

import (
	"errors"
	"net/http"
	"strconv"

	accountcontract "game-admin/backend/internal/contract/account"
	"game-admin/backend/internal/domain/model"
	accountsvc "game-admin/backend/internal/services/account"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerFinanceOrderRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	service := accountsvc.NewService(deps.DB)
	engine.POST("/internal/contracts/account/ledger", func(c *gin.Context) {
		var payload accountcontract.CreateLedgerRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ledger, err := service.CreateLedger(payload.Scope, accountsvc.LedgerCreateInput{
			AgentID:        payload.Input.AgentID,
			ReferenceType:  payload.Input.ReferenceType,
			ReferenceID:    payload.Input.ReferenceID,
			LedgerType:     payload.Input.LedgerType,
			Amount:         payload.Input.Amount,
			Currency:       payload.Input.Currency,
			OccurredAt:     payload.Input.OccurredAt,
			IdempotencyKey: payload.Input.IdempotencyKey,
			Remark:         payload.Input.Remark,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusOK, accountcontract.CreateLedgerResponse{Ledger: ledger})
	})
	engine.POST("/internal/contracts/account/risk-accounts", func(c *gin.Context) {
		var payload accountcontract.RiskAccountListRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items, err := service.ListRiskAccounts(payload.Scope, payload.AgentID)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		writeJSON(c, http.StatusOK, accountcontract.RiskAccountListResponse{Items: accountcontract.ToContractRiskSnapshots(items)})
	})
	engine.POST("/internal/contracts/account/risk-account-snapshot", func(c *gin.Context) {
		var payload accountcontract.RiskAccountGetRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		item, err := service.GetRiskAccountSnapshot(payload.Scope, payload.AgentID)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		writeJSON(c, http.StatusOK, accountcontract.RiskAccountGetResponse{Item: accountcontract.ToContractRiskSnapshot(item)})
	})
	api.GET("/orders", requirePermission(permissionSettlementRead), func(c *gin.Context) {
		items, err := service.ListOrders(toServiceScope(tenantScopeFromContext(c)), accountsvc.OrderListFilter{
			Status:  c.Query("status"),
			AgentID: c.Query("agentID"),
			GameID:  c.Query("gameID"),
			OrderNo: c.Query("orderNo"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.RechargeOrder]{Items: items, Total: len(items)})
	})
	api.POST("/orders/:id/profit-facts", requirePermission(permissionSettlementExecute), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		var payload orderProfitFactsPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, after, err := service.UpdateOrderProfitFacts(toServiceScope(tenantScopeFromContext(c)), id, accountsvc.OrderProfitFactsInput{
			PaidAmount:         payload.PaidAmount,
			PaymentChannelCost: payload.PaymentChannelCost,
			GrossProfitAmount:  payload.GrossProfitAmount,
			Remark:             payload.Remark,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			_ = logAudit(deps, auditEntry{
				OperatorID:   operatorIDFromContext(c),
				OperatorName: operatorNameFromContext(c),
				OperatorRole: operatorRoleFromContext(c),
				Module:       model.AuditModuleAccount,
				Action:       "order_profit_facts_update",
				TargetType:   "recharge_order",
				TargetID:     strconv.FormatUint(id, 10),
				RequestID:    requestIDFromContext(c),
				Result:       model.AuditResultFailed,
				IP:           c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Before:       before,
				After:        payload,
				ErrorMessage: err.Error(),
			})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{
			OperatorID:   operatorIDFromContext(c),
			OperatorName: operatorNameFromContext(c),
			OperatorRole: operatorRoleFromContext(c),
			Module:       model.AuditModuleAccount,
			Action:       "order_profit_facts_update",
			TargetType:   "recharge_order",
			TargetID:     strconv.FormatUint(after.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        after,
		})
		writeJSON(c, http.StatusOK, after)
	})
	api.GET("/commissions", requirePermission(permissionSettlementRead), func(c *gin.Context) {
		items, err := service.ListCommissions(toServiceScope(tenantScopeFromContext(c)), accountsvc.CommissionListFilter{
			Status:  c.Query("status"),
			AgentID: c.Query("agentID"),
			GameID:  c.Query("gameID"),
			OrderNo: c.Query("orderNo"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.CommissionRecord]{Items: items, Total: len(items)})
	})
	api.GET("/ledger", requirePermission(permissionSettlementRead), func(c *gin.Context) {
		items, err := service.ListLedgers(toServiceScope(tenantScopeFromContext(c)), accountsvc.LedgerListFilter{
			AgentID: c.Query("agentID"),
			Type:    c.Query("type"),
			OrderNo: c.Query("orderNo"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.AgentAccountLedger]{Items: items, Total: len(items)})
	})
	api.GET("/agent-accounts/ledger", requirePermission(permissionSettlementRead), func(c *gin.Context) {
		items, err := service.ListLedgers(toServiceScope(tenantScopeFromContext(c)), accountsvc.LedgerListFilter{
			AgentID: c.Query("agentID"),
			Type:    c.Query("type"),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, err)
			return
		}
		writeJSON(c, http.StatusOK, listResponse[model.AgentAccountLedger]{Items: items, Total: len(items)})
	})
	api.POST("/agent-accounts/ledger", requirePermission(permissionSettlementExecute), func(c *gin.Context) {
		var payload agentAccountLedgerCreatePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ledger, err := service.CreateLedger(toServiceScope(tenantScopeFromContext(c)), accountsvc.LedgerCreateInput{
			AgentID:        payload.AgentID,
			ReferenceType:  payload.ReferenceType,
			ReferenceID:    payload.ReferenceID,
			LedgerType:     payload.LedgerType,
			Amount:         payload.Amount,
			Currency:       payload.Currency,
			OccurredAt:     payload.OccurredAt,
			IdempotencyKey: payload.IdempotencyKey,
			Remark:         payload.Remark,
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
			Module:       model.AuditModuleAccount,
			Action:       "agent_account_ledger_create",
			TargetType:   "agent_account_ledger",
			TargetID:     strconv.FormatUint(ledger.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			After:        ledger,
		})
		writeJSON(c, http.StatusCreated, ledger)
	})
}
