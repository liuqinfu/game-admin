package settlement

import (
	"context"
	"net/http"

	"game-admin/backend/internal/domain/model"
	settlementsvc "game-admin/backend/internal/services/settlement"
	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"gorm.io/gorm"
)

type CreateBillInput struct {
	AgentID     uint64  `json:"agentID"`
	PeriodStart string  `json:"periodStart"`
	PeriodEnd   string  `json:"periodEnd"`
	Currency    string  `json:"currency"`
	Remark      string  `json:"remark"`
	Adjustment  float64 `json:"adjustment"`
	Cycle       string  `json:"cycle"`
}

type GenerateBillRequest struct {
	Scope        sharedsvc.Scope `json:"scope"`
	Input        CreateBillInput `json:"input"`
	SourceTaskID *uint64         `json:"sourceTaskID,omitempty"`
}

type GenerateBillResponse struct {
	BillID           uint64  `json:"billID"`
	DetailCount      int     `json:"detailCount"`
	CommissionAmount float64 `json:"commissionAmount"`
}

type Client interface {
	GenerateBillWithSourceTask(context.Context, sharedsvc.Scope, CreateBillInput, *uint64) (uint64, int, float64, error)
}

type billGenerator interface {
	GenerateBillWithSourceTask(sharedsvc.Scope, settlementsvc.CreateBillInput, *uint64) (model.SettlementBill, int, error)
}

type InProcessClient struct {
	service billGenerator
}

func NewInProcessClient(service billGenerator) *InProcessClient {
	return &InProcessClient{service: service}
}

func NewLocalDBClient(db *gorm.DB) *InProcessClient {
	return NewInProcessClient(settlementsvc.NewService(db))
}

func (c *InProcessClient) GenerateBillWithSourceTask(_ context.Context, scope sharedsvc.Scope, input CreateBillInput, sourceTaskID *uint64) (uint64, int, float64, error) {
	bill, detailCount, err := c.service.GenerateBillWithSourceTask(scope, toServiceInput(input), sourceTaskID)
	if err != nil {
		return 0, 0, 0, err
	}
	return bill.ID, detailCount, bill.CommissionAmount, nil
}

type HTTPClient struct {
	client *syncclient.Client
}

func NewHTTPClient(baseURL string, options syncclient.Options) *HTTPClient {
	return &HTTPClient{
		client: syncclient.New("settlement-service", baseURL, options),
	}
}

func (c *HTTPClient) GenerateBillWithSourceTask(ctx context.Context, scope sharedsvc.Scope, input CreateBillInput, sourceTaskID *uint64) (uint64, int, float64, error) {
	var response GenerateBillResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/settlement/generate-bill", GenerateBillRequest{
		Scope:        sharedsvc.NormalizeScope(scope),
		Input:        input,
		SourceTaskID: sourceTaskID,
	}, &response)
	if err != nil {
		return 0, 0, 0, err
	}
	return response.BillID, response.DetailCount, response.CommissionAmount, nil
}

func toServiceInput(input CreateBillInput) settlementsvc.CreateBillInput {
	return settlementsvc.CreateBillInput{
		AgentID:     input.AgentID,
		PeriodStart: input.PeriodStart,
		PeriodEnd:   input.PeriodEnd,
		Currency:    input.Currency,
		Remark:      input.Remark,
		Adjustment:  input.Adjustment,
		Cycle:       input.Cycle,
	}
}
