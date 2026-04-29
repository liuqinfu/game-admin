package account

import (
	"errors"
	"slices"
	"strings"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type OrderListFilter struct {
	Status  string
	AgentID string
	GameID  string
	OrderNo string
}

type CommissionListFilter struct {
	Status  string
	AgentID string
	GameID  string
	OrderNo string
}

type LedgerListFilter struct {
	AgentID string
	Type    string
	OrderNo string
}

type OrderProfitFactsInput struct {
	PaidAmount         *float64
	PaymentChannelCost *float64
	GrossProfitAmount  *float64
	Remark             string
}

type LedgerCreateInput struct {
	AgentID        uint64
	ReferenceType  string
	ReferenceID    string
	LedgerType     model.LedgerType
	Amount         float64
	Currency       string
	OccurredAt     string
	IdempotencyKey string
	Remark         string
}

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) ListOrders(scope sharedsvc.Scope, filter OrderListFilter) ([]model.RechargeOrder, error) {
	var agentIDs []uint64
	if scope.AgentID != nil {
		ids, err := s.currentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return nil, err
		}
		agentIDs = ids
	}
	return s.repo.ListOrders(scope, filter, agentIDs)
}

func (s *Service) UpdateOrderProfitFacts(scope sharedsvc.Scope, id uint64, input OrderProfitFactsInput) (model.RechargeOrder, model.RechargeOrder, error) {
	before, err := s.repo.FindRechargeOrderInScope(scope, id)
	if err != nil {
		return model.RechargeOrder{}, model.RechargeOrder{}, err
	}
	if before.Status != model.OrderStatusPaid {
		return before, model.RechargeOrder{}, errors.New("only paid recharge orders can update profit facts")
	}
	if input.PaidAmount != nil && *input.PaidAmount <= 0 {
		return before, model.RechargeOrder{}, errors.New("paidAmount must be greater than zero")
	}
	if input.PaymentChannelCost != nil && *input.PaymentChannelCost < 0 {
		return before, model.RechargeOrder{}, errors.New("paymentChannelCost must be greater than or equal to zero")
	}
	if input.GrossProfitAmount != nil && *input.GrossProfitAmount < 0 {
		return before, model.RechargeOrder{}, errors.New("grossProfitAmount must be greater than or equal to zero")
	}
	after := before
	if input.PaidAmount != nil {
		after.PaidAmount = sharedsvc.Round2(*input.PaidAmount)
	}
	if input.PaymentChannelCost != nil {
		after.PaymentChannelCost = sharedsvc.NormalizeOptionalAmount(input.PaymentChannelCost)
	}
	if input.GrossProfitAmount != nil {
		after.GrossProfitAmount = sharedsvc.NormalizeOptionalAmount(input.GrossProfitAmount)
	}
	if remark := strings.TrimSpace(input.Remark); remark != "" {
		after.Remark = sharedsvc.FirstNonEmpty(strings.TrimSpace(before.Remark+" | "+remark), remark)
	}
	if err := s.repo.SaveRechargeOrder(&after); err != nil {
		return before, model.RechargeOrder{}, err
	}
	return before, after, nil
}

func (s *Service) ListCommissions(scope sharedsvc.Scope, filter CommissionListFilter) ([]model.CommissionRecord, error) {
	return s.repo.ListCommissions(scope, filter)
}

func (s *Service) ListLedgers(scope sharedsvc.Scope, filter LedgerListFilter) ([]model.AgentAccountLedger, error) {
	return s.repo.ListLedgers(scope, filter)
}

func (s *Service) ListRiskAccounts(scope sharedsvc.Scope, agentID string) ([]RiskAccountSnapshot, error) {
	return s.repo.ListRiskAccounts(scope, agentID)
}

func (s *Service) GetRiskAccountSnapshot(scope sharedsvc.Scope, agentID uint64) (RiskAccountSnapshot, error) {
	return s.repo.GetRiskAccountSnapshot(scope, agentID)
}

func (s *Service) CreateLedger(scope sharedsvc.Scope, input LedgerCreateInput) (model.AgentAccountLedger, error) {
	if err := s.ensureAgentInCurrentScope(scope, input.AgentID); err != nil {
		return model.AgentAccountLedger{}, err
	}
	return s.createLedger(input)
}

func (s *Service) CreateLedgerWithDB(db *gorm.DB, scope sharedsvc.Scope, input LedgerCreateInput) (model.AgentAccountLedger, error) {
	if err := s.ensureAgentInCurrentScope(scope, input.AgentID); err != nil {
		return model.AgentAccountLedger{}, err
	}
	return s.createLedgerWithDB(db, input)
}

func (s *Service) createLedger(input LedgerCreateInput) (model.AgentAccountLedger, error) {
	return s.createLedgerWithDB(s.db, input)
}

func (s *Service) createLedgerWithDB(db *gorm.DB, input LedgerCreateInput) (model.AgentAccountLedger, error) {
	ledger, err := sharedsvc.CreateAgentAccountLedger(db, sharedsvc.AgentLedgerInput{
		AgentID:        input.AgentID,
		ReferenceType:  input.ReferenceType,
		ReferenceID:    input.ReferenceID,
		LedgerType:     input.LedgerType,
		Amount:         input.Amount,
		Currency:       input.Currency,
		OccurredAt:     input.OccurredAt,
		IdempotencyKey: input.IdempotencyKey,
		Remark:         input.Remark,
		ReversePolicy:  sharedsvc.LedgerReversePolicyConditional,
	})
	if err == nil {
		return ledger, nil
	}
	if !sharedsvc.IsUniqueConstraintError(err) {
		return model.AgentAccountLedger{}, err
	}
	existing, lookupErr := s.repo.FindLedgerByIdempotencyKey(input.IdempotencyKey)
	if lookupErr != nil {
		return model.AgentAccountLedger{}, err
	}
	return existing, nil
}

func (s *Service) ensureAgentInCurrentScope(scope sharedsvc.Scope, agentID uint64) error {
	if agentID == 0 {
		return errors.New("agentID is required")
	}
	agent, err := s.repo.FindAgent(agentID)
	if err != nil {
		return err
	}
	if !sharedsvc.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
		return gorm.ErrRecordNotFound
	}
	if scope.AgentID != nil {
		ids, err := s.currentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return err
		}
		if !slices.Contains(ids, agentID) {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func (s *Service) applyAgentAccountScope(query *gorm.DB, scope sharedsvc.Scope, column string) (*gorm.DB, error) {
	if scope.AgentID == nil {
		return query, nil
	}
	ids, err := s.currentAgentScopeIDs(*scope.AgentID)
	if err != nil {
		return nil, err
	}
	return query.Where(column+" IN ?", ids), nil
}

func (s *Service) applyAgentSelfScope(query *gorm.DB, scope sharedsvc.Scope, column string) *gorm.DB {
	if scope.AgentID != nil {
		return query.Where(column+" = ?", *scope.AgentID)
	}
	return query
}

func (s *Service) currentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return s.repo.CurrentAgentScopeIDs(agentID)
}
