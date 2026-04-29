package risk

import (
	"context"
	"net/http"

	risksvc "game-admin/backend/internal/services/risk"
	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"gorm.io/gorm"
)

type ReleasePendingCasesRequest struct {
	Scope    sharedsvc.Scope `json:"scope"`
	AgentID  uint64          `json:"agentID"`
	Remark   string          `json:"remark"`
	Reviewer string          `json:"reviewer"`
}

type ReleasePendingCasesResponse struct {
	ReleasedCount int      `json:"releasedCount"`
	CaseNos       []string `json:"caseNos"`
}

type RestorePendingCasesRequest struct {
	Scope    sharedsvc.Scope `json:"scope"`
	CaseNos  []string        `json:"caseNos"`
	Remark   string          `json:"remark"`
	Reviewer string          `json:"reviewer"`
}

type RestorePendingCasesResponse struct {
	RestoredCount int `json:"restoredCount"`
}

type Client interface {
	ReleasePendingCases(context.Context, sharedsvc.Scope, uint64, string, string) ([]string, error)
	RestorePendingCases(context.Context, sharedsvc.Scope, []string, string, string) (int, error)
}

type releaser interface {
	ReleasePendingCases(context.Context, sharedsvc.Scope, uint64, string, string) ([]string, error)
	RestorePendingCases(context.Context, sharedsvc.Scope, []string, string, string) (int, error)
}

type InProcessClient struct {
	service releaser
}

func NewInProcessClient(service releaser) *InProcessClient {
	return &InProcessClient{service: service}
}

func NewLocalDBClient(db *gorm.DB) *InProcessClient {
	return NewInProcessClient(risksvc.NewService(db))
}

func (c *InProcessClient) ReleasePendingCases(ctx context.Context, scope sharedsvc.Scope, agentID uint64, remark, reviewer string) ([]string, error) {
	return c.service.ReleasePendingCases(ctx, scope, agentID, remark, reviewer)
}

func (c *InProcessClient) RestorePendingCases(ctx context.Context, scope sharedsvc.Scope, caseNos []string, remark, reviewer string) (int, error) {
	return c.service.RestorePendingCases(ctx, scope, caseNos, remark, reviewer)
}

type HTTPClient struct {
	client *syncclient.Client
}

func NewHTTPClient(baseURL string, options syncclient.Options) *HTTPClient {
	return &HTTPClient{
		client: syncclient.New("risk-service", baseURL, options),
	}
}

func (c *HTTPClient) ReleasePendingCases(ctx context.Context, scope sharedsvc.Scope, agentID uint64, remark, reviewer string) ([]string, error) {
	var response ReleasePendingCasesResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/risk/release-pending-cases", ReleasePendingCasesRequest{
		Scope:    sharedsvc.NormalizeScope(scope),
		AgentID:  agentID,
		Remark:   remark,
		Reviewer: reviewer,
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.CaseNos, nil
}

func (c *HTTPClient) RestorePendingCases(ctx context.Context, scope sharedsvc.Scope, caseNos []string, remark, reviewer string) (int, error) {
	var response RestorePendingCasesResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/risk/restore-pending-cases", RestorePendingCasesRequest{
		Scope:    sharedsvc.NormalizeScope(scope),
		CaseNos:  caseNos,
		Remark:   remark,
		Reviewer: reviewer,
	}, &response)
	if err != nil {
		return 0, err
	}
	return response.RestoredCount, nil
}

var _ releaser = (*risksvc.Service)(nil)
