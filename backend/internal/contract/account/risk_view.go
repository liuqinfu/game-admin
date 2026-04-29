package account

import (
	"context"
	"net/http"
	"strings"

	"game-admin/backend/internal/domain/model"
	accountsvc "game-admin/backend/internal/services/account"
	sharedsvc "game-admin/backend/internal/services/shared"
)

type RiskAccountSnapshot struct {
	AgentID             uint64              `json:"agentID"`
	AgentName           string              `json:"agentName"`
	Account             *model.AgentAccount `json:"account,omitempty"`
	IncomeAmount        float64             `json:"incomeAmount"`
	ManualReverseAmount float64             `json:"manualReverseAmount"`
}

type RiskAccountListRequest struct {
	Scope   sharedsvc.Scope `json:"scope"`
	AgentID string          `json:"agentID"`
}

type RiskAccountListResponse struct {
	Items []RiskAccountSnapshot `json:"items"`
}

type RiskAccountGetRequest struct {
	Scope   sharedsvc.Scope `json:"scope"`
	AgentID uint64          `json:"agentID"`
}

type RiskAccountGetResponse struct {
	Item RiskAccountSnapshot `json:"item"`
}

type riskAccountReader interface {
	ListRiskAccounts(sharedsvc.Scope, string) ([]accountsvc.RiskAccountSnapshot, error)
	GetRiskAccountSnapshot(sharedsvc.Scope, uint64) (accountsvc.RiskAccountSnapshot, error)
}

func (c *InProcessClient) ListRiskAccounts(_ context.Context, scope sharedsvc.Scope, agentID string) ([]RiskAccountSnapshot, error) {
	reader, ok := c.service.(riskAccountReader)
	if !ok {
		return nil, nil
	}
	items, err := reader.ListRiskAccounts(scope, agentID)
	if err != nil {
		return nil, err
	}
	return ToContractRiskSnapshots(items), nil
}

func (c *InProcessClient) GetRiskAccountSnapshot(_ context.Context, scope sharedsvc.Scope, agentID uint64) (RiskAccountSnapshot, error) {
	reader, ok := c.service.(riskAccountReader)
	if !ok {
		return RiskAccountSnapshot{}, nil
	}
	item, err := reader.GetRiskAccountSnapshot(scope, agentID)
	if err != nil {
		return RiskAccountSnapshot{}, err
	}
	return ToContractRiskSnapshot(item), nil
}

func (c *HTTPClient) ListRiskAccounts(ctx context.Context, scope sharedsvc.Scope, agentID string) ([]RiskAccountSnapshot, error) {
	var response RiskAccountListResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/account/risk-accounts", RiskAccountListRequest{
		Scope:   sharedsvc.NormalizeScope(scope),
		AgentID: strings.TrimSpace(agentID),
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *HTTPClient) GetRiskAccountSnapshot(ctx context.Context, scope sharedsvc.Scope, agentID uint64) (RiskAccountSnapshot, error) {
	var response RiskAccountGetResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/account/risk-account-snapshot", RiskAccountGetRequest{
		Scope:   sharedsvc.NormalizeScope(scope),
		AgentID: agentID,
	}, &response)
	if err != nil {
		return RiskAccountSnapshot{}, err
	}
	return response.Item, nil
}

func ToContractRiskSnapshots(items []accountsvc.RiskAccountSnapshot) []RiskAccountSnapshot {
	result := make([]RiskAccountSnapshot, 0, len(items))
	for _, item := range items {
		result = append(result, ToContractRiskSnapshot(item))
	}
	return result
}

func ToContractRiskSnapshot(item accountsvc.RiskAccountSnapshot) RiskAccountSnapshot {
	return RiskAccountSnapshot{
		AgentID:             item.AgentID,
		AgentName:           item.AgentName,
		Account:             item.Account,
		IncomeAmount:        item.IncomeAmount,
		ManualReverseAmount: item.ManualReverseAmount,
	}
}
