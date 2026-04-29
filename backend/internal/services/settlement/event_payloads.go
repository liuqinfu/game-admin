package settlement

import "game-admin/backend/internal/domain/model"

func settlementBillGeneratedPayload(bill model.SettlementBill, detailCount int) map[string]any {
	return map[string]any{
		"id":          bill.ID,
		"billNo":      bill.BillNo,
		"agentID":     bill.AgentID,
		"status":      bill.Status,
		"detailCount": detailCount,
		"payable":     bill.PayableAmount,
		"currency":    bill.Currency,
	}
}

func settlementBillConfirmedPayload(bill model.SettlementBill) map[string]any {
	return map[string]any{
		"id":          bill.ID,
		"billNo":      bill.BillNo,
		"agentID":     bill.AgentID,
		"status":      bill.Status,
		"confirmedBy": bill.ConfirmedBy,
		"payable":     bill.PayableAmount,
		"currency":    bill.Currency,
	}
}
