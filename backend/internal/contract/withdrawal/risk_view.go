package withdrawal

import (
	"context"
	"net/http"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"gorm.io/gorm"
)

type RiskSignal struct {
	model.WithdrawalRequest
	AgentName string `json:"agentName"`
}

type RiskSignalListRequest struct {
	Scope sharedsvc.Scope `json:"scope"`
	Limit int             `json:"limit"`
}

type RiskSignalListResponse struct {
	Items []RiskSignal `json:"items"`
}

type LatestRiskWithdrawalRequest struct {
	AgentID  uint64  `json:"agentID"`
	Currency string  `json:"currency"`
	TenantID *uint64 `json:"tenantID,omitempty"`
	BrandID  *uint64 `json:"brandID,omitempty"`
}

type LatestRiskWithdrawalResponse struct {
	Item *model.WithdrawalRequest `json:"item,omitempty"`
}

type Client interface {
	ListRiskSignals(context.Context, sharedsvc.Scope, int) ([]RiskSignal, error)
	FindLatestRiskWithdrawal(context.Context, LatestRiskWithdrawalRequest) (*model.WithdrawalRequest, error)
}

type InProcessClient struct {
	service Client
}

func NewInProcessClient(service Client) *InProcessClient {
	return &InProcessClient{service: service}
}

func (c *InProcessClient) ListRiskSignals(_ context.Context, scope sharedsvc.Scope, limit int) ([]RiskSignal, error) {
	return c.service.ListRiskSignals(context.Background(), scope, limit)
}

func (c *InProcessClient) FindLatestRiskWithdrawal(_ context.Context, request LatestRiskWithdrawalRequest) (*model.WithdrawalRequest, error) {
	return c.service.FindLatestRiskWithdrawal(context.Background(), request)
}

type HTTPClient struct {
	client *syncclient.Client
}

func NewHTTPClient(baseURL string, options syncclient.Options) *HTTPClient {
	return &HTTPClient{client: syncclient.New("withdrawal-service", baseURL, options)}
}

func (c *HTTPClient) ListRiskSignals(ctx context.Context, scope sharedsvc.Scope, limit int) ([]RiskSignal, error) {
	var response RiskSignalListResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/withdrawal/risk-signals", RiskSignalListRequest{
		Scope: sharedsvc.NormalizeScope(scope),
		Limit: limit,
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *HTTPClient) FindLatestRiskWithdrawal(ctx context.Context, request LatestRiskWithdrawalRequest) (*model.WithdrawalRequest, error) {
	var response LatestRiskWithdrawalResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/withdrawal/latest-risk-withdrawal", request, &response)
	if err != nil {
		return nil, err
	}
	return response.Item, nil
}

type LocalDBClient struct {
	db *gorm.DB
}

func NewLocalDBClient(db *gorm.DB) *LocalDBClient {
	return &LocalDBClient{db: db}
}

func (c *LocalDBClient) ListRiskSignals(_ context.Context, scope sharedsvc.Scope, limit int) ([]RiskSignal, error) {
	query := c.db.Model(&model.WithdrawalRequest{}).
		Select("withdrawal_request.*, agent.name AS agent_name").
		Joins("JOIN agent ON agent.id = withdrawal_request.agent_id").
		Where("withdrawal_request.status IN ?", []model.WithdrawalStatus{
			model.WithdrawalStatusPending,
			model.WithdrawalStatusApproved,
			model.WithdrawalStatusFailed,
			model.WithdrawalStatusReturned,
		}).
		Order("withdrawal_request.id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "withdrawal_request.tenant_id", "withdrawal_request.brand_id", "TRIM(agent.remark)")
	var rows []RiskSignal
	if limit > 0 {
		query = query.Limit(limit)
	}
	return rows, query.Find(&rows).Error
}

func (c *LocalDBClient) FindLatestRiskWithdrawal(_ context.Context, request LatestRiskWithdrawalRequest) (*model.WithdrawalRequest, error) {
	query := c.db.Model(&model.WithdrawalRequest{}).Where("agent_id = ?", request.AgentID)
	if request.Currency != "" {
		query = query.Where("currency = ?", request.Currency)
	}
	if request.TenantID != nil {
		query = query.Where("tenant_id = ?", *request.TenantID)
	}
	if request.BrandID != nil {
		query = query.Where("brand_id = ?", *request.BrandID)
	}
	var item model.WithdrawalRequest
	if err := query.Order("id desc").First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}
