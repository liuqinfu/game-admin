package withdrawal

import "game-admin/backend/internal/domain/model"

func withdrawalCreatedPayload(request model.WithdrawalRequest) map[string]any {
	return map[string]any{
		"id":         request.ID,
		"requestNo":  request.RequestNo,
		"agentID":    request.AgentID,
		"tenantCode": request.TenantCode,
		"amount":     request.Amount,
		"currency":   request.Currency,
		"status":     request.Status,
	}
}

func withdrawalReviewedPayload(request model.WithdrawalRequest) map[string]any {
	return map[string]any{
		"id":         request.ID,
		"requestNo":  request.RequestNo,
		"agentID":    request.AgentID,
		"status":     request.Status,
		"reviewedBy": request.ReviewedBy,
		"remark":     request.Remark,
	}
}

func withdrawalPayoutPayload(request model.WithdrawalRequest) map[string]any {
	return map[string]any{
		"id":                request.ID,
		"requestNo":         request.RequestNo,
		"agentID":           request.AgentID,
		"status":            request.Status,
		"payoutReference":   request.PayoutReference,
		"completedLedgerID": request.CompletedLedgerID,
		"failureLedgerID":   request.FailureLedgerID,
	}
}
