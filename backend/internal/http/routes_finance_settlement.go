package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	settlementcontract "game-admin/backend/internal/contract/settlement"
	"game-admin/backend/internal/domain/model"
	recalcsvc "game-admin/backend/internal/services/recalculation"
	settlementsvc "game-admin/backend/internal/services/settlement"
	"game-admin/backend/internal/syncclient"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerFinanceSettlementRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	settlementService := settlementsvc.NewService(deps.DB)
	recalculationService := recalcsvc.NewServiceWithSettlementClient(deps.DB, settlementContractClient(deps, cfg))
	engine.POST("/internal/contracts/settlement/generate-bill", func(c *gin.Context) {
		var payload settlementcontract.GenerateBillRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		bill, detailCount, err := settlementService.GenerateBillWithSourceTask(payload.Scope, settlementsvc.CreateBillInput{
			AgentID:     payload.Input.AgentID,
			PeriodStart: payload.Input.PeriodStart,
			PeriodEnd:   payload.Input.PeriodEnd,
			Currency:    payload.Input.Currency,
			Remark:      payload.Input.Remark,
			Adjustment:  payload.Input.Adjustment,
			Cycle:       payload.Input.Cycle,
		}, payload.SourceTaskID)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusOK, settlementcontract.GenerateBillResponse{
			BillID:           bill.ID,
			DetailCount:      detailCount,
			CommissionAmount: bill.CommissionAmount,
		})
	})
	api.GET("/settlement-bills", requirePermission(permissionSettlementBillRead), func(c *gin.Context) {
		serviceItems, err := settlementService.ListBillItems(toServiceScope(tenantScopeFromContext(c)), settlementsvc.BillFilter{
			AgentID: strings.TrimSpace(c.Query("agentID")),
			Status:  strings.TrimSpace(c.Query("status")),
			BillNo:  strings.TrimSpace(c.Query("billNo")),
		})
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]settlementBillListItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPSettlementBillListItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[settlementBillListItem]{Items: items, Total: len(items)})
	})
	api.POST("/settlement-bills/generate", requirePermission(permissionSettlementExecute), func(c *gin.Context) {
		var payload settlementBillCreatePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		serviceItem, err := settlementService.GenerateBillItem(toServiceScope(tenantScopeFromContext(c)), settlementsvc.CreateBillInput{
			AgentID:     payload.AgentID,
			PeriodStart: payload.PeriodStart,
			PeriodEnd:   payload.PeriodEnd,
			Currency:    payload.Currency,
			Remark:      payload.Remark,
			Adjustment:  payload.Adjustment,
			Cycle:       payload.Cycle,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusCreated, toHTTPSettlementBillListItem(serviceItem))
	})
	api.POST("/settlement-bills/:id/confirm", requirePermission(permissionSettlementBillConfirm), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, confirmed, serviceItem, err := settlementService.ConfirmBillItem(toServiceScope(tenantScopeFromContext(c)), id, operatorNameFromContext(c))
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
			Module:       model.AuditModuleCommission,
			Action:       "settlement_bill_confirm",
			TargetType:   "settlement_bill",
			TargetID:     strconv.FormatUint(confirmed.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        confirmed,
		})
		writeJSON(c, http.StatusOK, toHTTPSettlementBillListItem(serviceItem))
	})
	api.POST("/settlement-bills/:id/export", requirePermission(permissionSettlementBillExport), func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		before, exported, err := settlementService.ExportBill(toServiceScope(tenantScopeFromContext(c)), id)
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
			Module:       model.AuditModuleCommission,
			Action:       "settlement_bill_export",
			TargetType:   "settlement_bill",
			TargetID:     strconv.FormatUint(exported.ID, 10),
			RequestID:    requestIDFromContext(c),
			Result:       model.AuditResultSuccess,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Before:       before,
			After:        exported,
		})
		c.Status(http.StatusNoContent)
	})
	api.GET("/recalculation-tasks", requirePermission(permissionRecalculationTaskRead), func(c *gin.Context) {
		serviceItems, err := recalculationService.List(toServiceScope(tenantScopeFromContext(c)), strings.TrimSpace(c.Query("status")))
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		items := make([]recalculationTaskListItem, 0, len(serviceItems))
		for _, item := range serviceItems {
			items = append(items, toHTTPRecalculationTaskItem(item))
		}
		writeJSON(c, http.StatusOK, listResponse[recalculationTaskListItem]{Items: items, Total: len(items)})
	})
	api.POST("/recalculation-tasks", requirePermission(permissionRecalculationTaskCreate), func(c *gin.Context) {
		var payload recalculationTaskCreatePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondError(c, http.StatusBadRequest, err)
			return
		}
		ctx := syncclient.WithCorrelation(c.Request.Context(), requestIDFromContext(c), traceIDFromContext(c))
		task, err := recalculationService.Create(ctx, toServiceScope(tenantScopeFromContext(c)), recalcsvc.CreateInput{
			TaskType:         payload.TaskType,
			AgentID:          payload.AgentID,
			SettlementBillID: payload.SettlementBillID,
			PeriodStart:      payload.PeriodStart,
			PeriodEnd:        payload.PeriodEnd,
			Operator:         payload.Operator,
			Reason:           payload.Reason,
			Scope:            payload.Scope,
			Remark:           payload.Remark,
		}, operatorNameFromContext(c))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			respondError(c, status, err)
			return
		}
		writeJSON(c, http.StatusCreated, toHTTPRecalculationTaskItem(recalculationService.BuildListItem(task)))
	})
}
