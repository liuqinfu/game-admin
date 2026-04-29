package recharge

import "game-admin/backend/internal/domain/model"

func rechargeOrderPayload(order model.RechargeOrder, duplicate bool, commissions, ledgers int) map[string]any {
	payload := map[string]any{
		"id":          order.ID,
		"orderNo":     order.OrderNo,
		"playerID":    order.PlayerID,
		"gameID":      order.GameID,
		"agentID":     order.AgentID,
		"status":      order.Status,
		"duplicate":   duplicate,
		"commissions": commissions,
		"ledgers":     ledgers,
		"amount":      order.Amount,
		"paidAmount":  order.PaidAmount,
		"currency":    order.Currency,
	}
	if order.RiskFlag != "" {
		payload["riskFlag"] = order.RiskFlag
	}
	return payload
}
