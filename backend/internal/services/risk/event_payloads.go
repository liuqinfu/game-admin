package risk

import "game-admin/backend/internal/domain/model"

func riskCaseCreatedPayload(riskCase model.RiskCase, ledgerID *uint64) map[string]any {
	payload := map[string]any{
		"id":              riskCase.ID,
		"caseNo":          riskCase.CaseNo,
		"agentID":         riskCase.AgentID,
		"status":          riskCase.Status,
		"freezeRequested": riskCase.FreezeRequested,
		"amount":          riskCase.Amount,
		"currency":        riskCase.Currency,
	}
	if ledgerID != nil {
		payload["ledgerID"] = *ledgerID
	}
	return payload
}

func riskCaseReviewedPayload(riskCase model.RiskCase, ledgerID *uint64) map[string]any {
	payload := map[string]any{
		"id":         riskCase.ID,
		"caseNo":     riskCase.CaseNo,
		"agentID":    riskCase.AgentID,
		"status":     riskCase.Status,
		"reviewedBy": riskCase.ReviewedBy,
	}
	if ledgerID != nil {
		payload["ledgerID"] = *ledgerID
	}
	return payload
}
