package account

import (
	"context"
	"net/http"

	"game-admin/backend/internal/domain/model"
	accountsvc "game-admin/backend/internal/services/account"
	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"gorm.io/gorm"
)

type LedgerCreateInput struct {
	AgentID        uint64           `json:"agentID"`
	ReferenceType  string           `json:"referenceType"`
	ReferenceID    string           `json:"referenceID"`
	LedgerType     model.LedgerType `json:"ledgerType"`
	Amount         float64          `json:"amount"`
	Currency       string           `json:"currency"`
	OccurredAt     string           `json:"occurredAt"`
	IdempotencyKey string           `json:"idempotencyKey"`
	Remark         string           `json:"remark"`
}

type CreateLedgerRequest struct {
	Scope sharedsvc.Scope   `json:"scope"`
	Input LedgerCreateInput `json:"input"`
}

type CreateLedgerResponse struct {
	Ledger model.AgentAccountLedger `json:"ledger"`
}

type Client interface {
	CreateLedger(context.Context, sharedsvc.Scope, LedgerCreateInput) (model.AgentAccountLedger, error)
	ListRiskAccounts(context.Context, sharedsvc.Scope, string) ([]RiskAccountSnapshot, error)
	GetRiskAccountSnapshot(context.Context, sharedsvc.Scope, uint64) (RiskAccountSnapshot, error)
}

type ledgerCreator interface {
	CreateLedger(sharedsvc.Scope, accountsvc.LedgerCreateInput) (model.AgentAccountLedger, error)
}

type txLedgerCreator interface {
	CreateLedgerWithDB(*gorm.DB, sharedsvc.Scope, accountsvc.LedgerCreateInput) (model.AgentAccountLedger, error)
}

type InProcessClient struct {
	service ledgerCreator
}

func NewInProcessClient(service ledgerCreator) *InProcessClient {
	return &InProcessClient{service: service}
}

func NewLocalDBClient(db *gorm.DB) *InProcessClient {
	return NewInProcessClient(accountsvc.NewService(db))
}

type txContextKey string

const dbTxContextKey txContextKey = "account_contract_db_tx"

func WithDBTx(ctx context.Context, tx *gorm.DB) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, dbTxContextKey, tx)
}

func dbTxFromContext(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(dbTxContextKey).(*gorm.DB)
	return tx
}

func (c *InProcessClient) CreateLedger(ctx context.Context, scope sharedsvc.Scope, input LedgerCreateInput) (model.AgentAccountLedger, error) {
	if tx := dbTxFromContext(ctx); tx != nil {
		if creator, ok := c.service.(txLedgerCreator); ok {
			return creator.CreateLedgerWithDB(tx, scope, toServiceInput(input))
		}
	}
	return c.service.CreateLedger(scope, toServiceInput(input))
}

type HTTPClient struct {
	client *syncclient.Client
}

func NewHTTPClient(baseURL string, options syncclient.Options) *HTTPClient {
	return &HTTPClient{
		client: syncclient.New("account-service", baseURL, options),
	}
}

func (c *HTTPClient) CreateLedger(ctx context.Context, scope sharedsvc.Scope, input LedgerCreateInput) (model.AgentAccountLedger, error) {
	var response CreateLedgerResponse
	err := c.client.Do(ctx, http.MethodPost, "/internal/contracts/account/ledger", CreateLedgerRequest{
		Scope: sharedsvc.NormalizeScope(scope),
		Input: input,
	}, &response)
	if err != nil {
		return model.AgentAccountLedger{}, err
	}
	return response.Ledger, nil
}

func toServiceInput(input LedgerCreateInput) accountsvc.LedgerCreateInput {
	return accountsvc.LedgerCreateInput{
		AgentID:        input.AgentID,
		ReferenceType:  input.ReferenceType,
		ReferenceID:    input.ReferenceID,
		LedgerType:     input.LedgerType,
		Amount:         input.Amount,
		Currency:       input.Currency,
		OccurredAt:     input.OccurredAt,
		IdempotencyKey: input.IdempotencyKey,
		Remark:         input.Remark,
	}
}
