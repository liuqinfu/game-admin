package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	withdrawalcontract "game-admin/backend/internal/contract/withdrawal"
	"game-admin/backend/internal/domain/model"
	withdrawalsvc "game-admin/backend/internal/services/withdrawal"
	"game-admin/backend/internal/syncclient"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerFinanceWithdrawalRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	withdrawalService := withdrawalsvc.NewServiceWithClients(deps.DB, accountContractClient(deps, cfg), riskContractClient(deps, cfg))
	engine.POST("/internal/contracts/withdrawal/risk-signals", func(c *gin.Context) {
		var payload withdrawalcontract.RiskSignalListRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items, err := withdrawalService.ListRiskSignals(payload.Scope, payload.Limit)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		result := make([]withdrawalcontract.RiskSignal, 0, len(items))
		for _, item := range items {
			result = append(result, withdrawalcontract.RiskSignal{WithdrawalRequest: item.WithdrawalRequest, AgentName: item.AgentName})
		}
		writeJSON(c, http.StatusOK, withdrawalcontract.RiskSignalListResponse{Items: result})
	})
	engine.POST("/internal/contracts/withdrawal/latest-risk-withdrawal", func(c *gin.Context) {
		var payload withdrawalcontract.LatestRiskWithdrawalRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		item, err := withdrawalService.FindLatestRiskWithdrawal(withdrawalsvc.LatestRiskWithdrawalQuery{
			AgentID:  payload.AgentID,
			Currency: payload.Currency,
			TenantID: payload.TenantID,
			BrandID:  payload.BrandID,
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		writeJSON(c, http.StatusOK, withdrawalcontract.LatestRiskWithdrawalResponse{Item: item})
	})
	api.GET("/withdrawals", requirePermission(permissionWithdrawalRead), func(c *gin.Context) {
		listWithdrawalsRoute(c, withdrawalService)
	})
	api.GET("/withdrawal-requests", requirePermission(permissionWithdrawalRead), func(c *gin.Context) {
		listWithdrawalsRoute(c, withdrawalService)
	})
	api.POST("/withdrawals", requirePermission(permissionWithdrawalExecute), func(c *gin.Context) {
		var payload withdrawalCreatePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		request, err := withdrawalService.Create(syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c)), toServiceScope(tenantScopeFromContext(c)), withdrawalsvc.CreateInput{
			TenantCode:      payload.TenantCode,
			TenantID:        payload.TenantID,
			BrandID:         payload.BrandID,
			AgentID:         payload.AgentID,
			Amount:          payload.Amount,
			Currency:        payload.Currency,
			Channel:         payload.Channel,
			BankAccountName: payload.BankAccountName,
			BankAccountNo:   payload.BankAccountNo,
			BankName:        payload.BankName,
			IdempotencyKey:  payload.IdempotencyKey,
			Remark:          payload.Remark,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			} else if isUniqueConstraintError(err) {
				status = http.StatusConflict
			}
			writeWithdrawalAudit(c, deps, "withdrawal_create", strings.TrimSpace(payload.IdempotencyKey), model.AuditResultFailed, nil, payload, err)
			respondError(c, status, err)
			return
		}
		writeWithdrawalAudit(c, deps, "withdrawal_create", strconv.FormatUint(request.ID, 10), model.AuditResultSuccess, nil, request, nil)
		writeJSON(c, http.StatusCreated, request)
	})
	api.POST("/withdrawals/:id/review", requirePermission(permissionWithdrawalExecute), func(c *gin.Context) {
		reviewWithdrawalRoute(c, deps, withdrawalService)
	})
	api.POST("/withdrawal-requests/:id/review", requirePermission(permissionWithdrawalExecute), func(c *gin.Context) {
		reviewWithdrawalRoute(c, deps, withdrawalService)
	})
	api.POST("/withdrawals/:id/payout", requirePermission(permissionWithdrawalExecute), func(c *gin.Context) {
		payoutWithdrawalRoute(c, deps, withdrawalService)
	})
}

func reviewWithdrawalRoute(c *gin.Context, deps RouterDependencies, withdrawalService *withdrawalsvc.Service) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	var payload withdrawalReviewPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	before, after, err := withdrawalService.Review(syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c)), toServiceScope(tenantScopeFromContext(c)), id, withdrawalsvc.ReviewInput{
		Action: payload.Action,
		Remark: payload.Remark,
	}, operatorNameFromContext(c))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		writeWithdrawalAudit(c, deps, "withdrawal_review", strconv.FormatUint(id, 10), model.AuditResultFailed, before, payload, err)
		respondError(c, status, err)
		return
	}
	writeWithdrawalAudit(c, deps, "withdrawal_review", strconv.FormatUint(after.ID, 10), model.AuditResultSuccess, before, after, nil)
	writeJSON(c, http.StatusOK, after)
}

func payoutWithdrawalRoute(c *gin.Context, deps RouterDependencies, withdrawalService *withdrawalsvc.Service) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	var payload withdrawalPayoutPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	before, after, err := withdrawalService.Payout(syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c)), toServiceScope(tenantScopeFromContext(c)), id, withdrawalsvc.PayoutInput{
		Action:         payload.Action,
		Remark:         payload.Remark,
		Reference:      payload.Reference,
		ReceiptPayload: payload.ReceiptPayload,
	}, operatorNameFromContext(c))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		writeWithdrawalAudit(c, deps, "withdrawal_payout", strconv.FormatUint(id, 10), model.AuditResultFailed, before, payload, err)
		respondError(c, status, err)
		return
	}
	writeWithdrawalAudit(c, deps, "withdrawal_payout", strconv.FormatUint(after.ID, 10), model.AuditResultSuccess, before, after, nil)
	writeJSON(c, http.StatusOK, after)
}

func listWithdrawalsRoute(c *gin.Context, withdrawalService *withdrawalsvc.Service) {
	serviceItems, err := withdrawalService.List(toServiceScope(tenantScopeFromContext(c)), withdrawalsvc.ListFilter{
		TenantCode: strings.TrimSpace(c.Query("tenantCode")),
		Status:     strings.TrimSpace(c.Query("status")),
		AgentID:    strings.TrimSpace(c.Query("agentID")),
		RequestNo:  strings.TrimSpace(c.Query("requestNo")),
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	items := make([]withdrawalListItem, 0, len(serviceItems))
	for _, item := range serviceItems {
		items = append(items, toHTTPWithdrawalListItem(item))
	}
	writeJSON(c, http.StatusOK, listResponse[withdrawalListItem]{Items: items, Total: len(items)})
}

func writeWithdrawalAudit(c *gin.Context, deps RouterDependencies, action, targetID string, result model.AuditResult, before, after any, err error) {
	entry := auditEntry{
		OperatorID:   operatorIDFromContext(c),
		OperatorName: operatorNameFromContext(c),
		OperatorRole: operatorRoleFromContext(c),
		Module:       model.AuditModuleAccount,
		Action:       action,
		TargetType:   "withdrawal_request",
		TargetID:     targetID,
		RequestID:    requestIDFromContext(c),
		Result:       result,
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Before:       before,
		After:        after,
	}
	if err != nil {
		entry.ErrorMessage = err.Error()
	}
	_ = logAudit(deps, entry)
}
