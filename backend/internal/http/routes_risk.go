package http

import (
	"errors"
	"net/http"
	"strings"

	riskcontract "game-admin/backend/internal/contract/risk"
	"game-admin/backend/internal/domain/model"
	risksvc "game-admin/backend/internal/services/risk"
	"game-admin/backend/internal/syncclient"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerRiskRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	riskService := risksvc.NewServiceWithClients(deps.DB, reportContractClient(deps, cfg), accountContractClient(deps, cfg), withdrawalContractClient(deps, cfg))
	engine.POST("/internal/contracts/risk/release-pending-cases", func(c *gin.Context) {
		var payload riskcontract.ReleasePendingCasesRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		caseNos, err := riskService.ReleasePendingCases(ctx, payload.Scope, payload.AgentID, strings.TrimSpace(payload.Remark), strings.TrimSpace(payload.Reviewer))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusOK, riskcontract.ReleasePendingCasesResponse{ReleasedCount: len(caseNos), CaseNos: caseNos})
	})
	engine.POST("/internal/contracts/risk/restore-pending-cases", func(c *gin.Context) {
		var payload riskcontract.RestorePendingCasesRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		restoredCount, err := riskService.RestorePendingCases(ctx, payload.Scope, payload.CaseNos, strings.TrimSpace(payload.Remark), strings.TrimSpace(payload.Reviewer))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusOK, riskcontract.RestorePendingCasesResponse{RestoredCount: restoredCount})
	})
	api.GET("/risk/agent-accounts", requirePermission(permissionRiskRead), func(c *gin.Context) {
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		serviceItems, err := riskService.AgentAccounts(ctx, toServiceScope(tenantScopeFromContext(c)), risksvc.AgentAccountFilter{
			MinFrozenAmount: strings.TrimSpace(c.Query("minFrozenAmount")),
			RiskLevel:       strings.TrimSpace(c.Query("riskLevel")),
			AgentID:         strings.TrimSpace(c.Query("agentID")),
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]agentAccountRiskListItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPAgentAccountRiskItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[agentAccountRiskListItem]{Items: items, Total: len(items)})
	})
	api.GET("/risk/cases", requirePermission(permissionRiskRead), func(c *gin.Context) {
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		serviceItems, err := riskService.Cases(ctx, toServiceScope(tenantScopeFromContext(c)), risksvc.RiskCaseFilter{
			MinFrozenRatio: strings.TrimSpace(c.Query("minFrozenRatio")),
			RiskLevel:      strings.TrimSpace(c.Query("riskLevel")),
			Status:         strings.TrimSpace(c.Query("status")),
			AgentID:        strings.TrimSpace(c.Query("agentID")),
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]riskCaseListItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPRiskCaseItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[riskCaseListItem]{Items: items, Total: len(items)})
	})
	api.POST("/risk/cases", requirePermission(permissionRiskWrite), func(c *gin.Context) {
		var payload riskCasePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		caseRecord, frozenLedger, err := riskService.CreateCase(ctx, toServiceScope(tenantScopeFromContext(c)), risksvc.CreateCaseInput{
			CaseNo:            payload.CaseNo,
			TenantID:          payload.TenantID,
			BrandID:           payload.BrandID,
			AgentID:           payload.AgentID,
			Amount:            payload.Amount,
			Currency:          payload.Currency,
			Reason:            payload.Reason,
			Freeze:            payload.Freeze,
			FreezeIdempotency: payload.FreezeIdempotency,
			Remark:            payload.Remark,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			} else if isUniqueConstraintError(err) {
				status = http.StatusConflict
			}
			_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAccount, Action: "risk_case_create", TargetType: "risk_case", TargetID: strings.TrimSpace(payload.CaseNo), RequestID: requestIDFromContext(c), Result: model.AuditResultFailed, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: payload, ErrorMessage: err.Error()})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAccount, Action: "risk_case_create", TargetType: "risk_case", TargetID: strings.TrimSpace(payload.CaseNo), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: caseRecord})
		writeJSON(c, http.StatusCreated, riskCaseResponse{CaseNo: caseRecord.CaseNo, Status: string(caseRecord.Status), Frozen: frozenLedger != nil, FrozenLedger: frozenLedger})
	})
	api.POST("/risk/cases/:caseNo/review", requirePermission(permissionRiskRead), func(c *gin.Context) {
		var payload riskCaseReviewPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		caseRecord, ledger, err := riskService.ReviewCase(ctx, toServiceScope(tenantScopeFromContext(c)), c.Param("caseNo"), risksvc.ReviewCaseInput{
			Action: payload.Action,
			Remark: payload.Remark,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAccount, Action: "risk_case_review", TargetType: "risk_case", TargetID: strings.TrimSpace(c.Param("caseNo")), RequestID: requestIDFromContext(c), Result: model.AuditResultFailed, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: payload, ErrorMessage: err.Error()})
			respondError(c, status, err)
			return
		}
		_ = logAudit(deps, auditEntry{TraceID: traceIDFromContext(c), OperatorID: operatorIDFromContext(c), OperatorName: operatorNameFromContext(c), OperatorRole: operatorRoleFromContext(c), Module: model.AuditModuleAccount, Action: "risk_case_review", TargetType: "risk_case", TargetID: strings.TrimSpace(c.Param("caseNo")), RequestID: requestIDFromContext(c), Result: model.AuditResultSuccess, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), After: caseRecord})
		writeJSON(c, http.StatusOK, riskCaseResponse{CaseNo: caseRecord.CaseNo, Status: string(caseRecord.Status), Action: strings.ToLower(strings.TrimSpace(payload.Action)), Ledger: ledger})
	})
	api.GET("/risk/intelligence", requirePermission(permissionRiskRead), func(c *gin.Context) {
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		serviceItems, err := riskService.Intelligence(ctx, toServiceScope(tenantScopeFromContext(c)))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]riskIntelligenceItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPRiskIntelligenceItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[riskIntelligenceItem]{Items: items, Total: len(items)})
	})

}
