package report

import (
	"context"
	"net/http"

	reportsvc "game-admin/backend/internal/services/report"
	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"gorm.io/gorm"
)

type TeamPerformanceItem struct {
	AgentID                 uint64  `json:"agentID"`
	AgentName               string  `json:"agentName"`
	Level                   uint32  `json:"level"`
	Currency                string  `json:"currency"`
	DirectDescendants       int64   `json:"directDescendants"`
	TotalDescendants        int64   `json:"totalDescendants"`
	ActiveDescendants       int64   `json:"activeDescendants"`
	LeafDescendants         int64   `json:"leafDescendants"`
	BoundPlayers            int64   `json:"boundPlayers"`
	TotalTeamBalance        float64 `json:"totalTeamBalance"`
	TotalTeamFrozenBalance  float64 `json:"totalTeamFrozenBalance"`
	TotalWithdrawableAmount float64 `json:"totalWithdrawableAmount"`
	PendingWithdrawals      int64   `json:"pendingWithdrawals"`
	PendingWithdrawalAmount float64 `json:"pendingWithdrawalAmount"`
	ConfirmedBills          int64   `json:"confirmedBills"`
	ConfirmedBillAmount     float64 `json:"confirmedBillAmount"`
}

type TeamPerformanceRequest struct {
	Scope    sharedsvc.Scope `json:"scope"`
	AgentID  string          `json:"agentID"`
	Currency string          `json:"currency"`
}

type TeamPerformanceResponse struct {
	Items []TeamPerformanceItem `json:"items"`
}

type Client interface {
	TeamPerformance(context.Context, sharedsvc.Scope, string, string) ([]TeamPerformanceItem, error)
}

type teamPerformanceProvider interface {
	TeamPerformance(sharedsvc.Scope, string, string) ([]reportsvc.TeamPerformanceItem, error)
}

type InProcessClient struct {
	service teamPerformanceProvider
}

func NewInProcessClient(service teamPerformanceProvider) *InProcessClient {
	return &InProcessClient{service: service}
}

func NewLocalDBClient(db *gorm.DB) *InProcessClient {
	return NewInProcessClient(reportsvc.NewService(db))
}

func (c *InProcessClient) TeamPerformance(_ context.Context, scope sharedsvc.Scope, agentID, currency string) ([]TeamPerformanceItem, error) {
	items, err := c.service.TeamPerformance(scope, agentID, currency)
	if err != nil {
		return nil, err
	}
	return ToContractItems(items), nil
}

type HTTPClient struct {
	client *syncclient.Client
}

func NewHTTPClient(baseURL string, options syncclient.Options) *HTTPClient {
	return &HTTPClient{
		client: syncclient.New("report-service", baseURL, options),
	}
}

func (c *HTTPClient) TeamPerformance(ctx context.Context, scope sharedsvc.Scope, agentID, currency string) ([]TeamPerformanceItem, error) {
	var response TeamPerformanceResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/report/team-performance", TeamPerformanceRequest{
		Scope:    sharedsvc.NormalizeScope(scope),
		AgentID:  agentID,
		Currency: currency,
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.Items, nil
}

func ToContractItems(items []reportsvc.TeamPerformanceItem) []TeamPerformanceItem {
	result := make([]TeamPerformanceItem, 0, len(items))
	for _, item := range items {
		result = append(result, TeamPerformanceItem{
			AgentID:                 item.AgentID,
			AgentName:               item.AgentName,
			Level:                   item.Level,
			Currency:                item.Currency,
			DirectDescendants:       item.DirectDescendants,
			TotalDescendants:        item.TotalDescendants,
			ActiveDescendants:       item.ActiveDescendants,
			LeafDescendants:         item.LeafDescendants,
			BoundPlayers:            item.BoundPlayers,
			TotalTeamBalance:        item.TotalTeamBalance,
			TotalTeamFrozenBalance:  item.TotalTeamFrozenBalance,
			TotalWithdrawableAmount: item.TotalWithdrawableAmount,
			PendingWithdrawals:      item.PendingWithdrawals,
			PendingWithdrawalAmount: item.PendingWithdrawalAmount,
			ConfirmedBills:          item.ConfirmedBills,
			ConfirmedBillAmount:     item.ConfirmedBillAmount,
		})
	}
	return result
}
