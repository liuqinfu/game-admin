package risk

import (
	"strings"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type agentAccountRow struct {
	model.AgentAccount
	AgentName string
}

type riskCaseRow struct {
	model.RiskCase
	AgentName string
}

type withdrawalSignal struct {
	model.WithdrawalRequest
	AgentName string
}

type Repository interface {
	ListAgentAccounts(scope sharedsvc.Scope, agentIDText string) ([]agentAccountRow, error)
	ListRiskCases(scope sharedsvc.Scope, filter RiskCaseFilter) ([]riskCaseRow, error)
	ListWithdrawalSignals(scope sharedsvc.Scope, limit int) ([]withdrawalSignal, error)
	LoadLatestWithdrawalForRisk(riskCase model.RiskCase) (*model.WithdrawalRequest, error)
	LoadAgentAccountByAgentID(agentID uint64) (*model.AgentAccount, error)
	LoadLatestWithdrawal(agentID uint64) (*model.WithdrawalRequest, error)
	LoadLedgerRiskMetrics(agentID uint64) (float64, float64, error)
	EnsureAgentExists(agentID uint64) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListAgentAccounts(scope sharedsvc.Scope, agentIDText string) ([]agentAccountRow, error) {
	query := r.db.Model(&model.AgentAccount{}).
		Select("agent_account.*, agent.name as agent_name").
		Joins("JOIN agent ON agent.id = agent_account.agent_id").
		Order("agent_account.id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "agent_account.tenant_id", "agent_account.brand_id", "TRIM(agent.remark)")
	if value := strings.TrimSpace(agentIDText); value != "" {
		query = query.Where("agent_account.agent_id = ?", value)
	}
	var rows []agentAccountRow
	return rows, query.Find(&rows).Error
}

func (r *gormRepository) ListRiskCases(scope sharedsvc.Scope, filter RiskCaseFilter) ([]riskCaseRow, error) {
	query := r.db.Model(&model.RiskCase{}).
		Select("risk_case.*, agent.name as agent_name").
		Joins("JOIN agent ON agent.id = risk_case.agent_id").
		Order("risk_case.id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "risk_case.tenant_id", "risk_case.brand_id", "TRIM(agent.remark)")
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("risk_case.status = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("risk_case.agent_id = ?", value)
	}
	var rows []riskCaseRow
	return rows, query.Find(&rows).Error
}

func (r *gormRepository) ListWithdrawalSignals(scope sharedsvc.Scope, limit int) ([]withdrawalSignal, error) {
	query := r.db.Model(&model.WithdrawalRequest{}).
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
	var items []withdrawalSignal
	if limit > 0 {
		query = query.Limit(limit)
	}
	return items, query.Find(&items).Error
}

func (r *gormRepository) LoadLatestWithdrawalForRisk(riskCase model.RiskCase) (*model.WithdrawalRequest, error) {
	query := r.db.Model(&model.WithdrawalRequest{}).Where("agent_id = ?", riskCase.AgentID)
	if currency := strings.TrimSpace(riskCase.Currency); currency != "" {
		query = query.Where("currency = ?", currency)
	}
	if riskCase.TenantID != nil {
		query = query.Where("tenant_id = ?", *riskCase.TenantID)
	}
	if riskCase.BrandID != nil {
		query = query.Where("brand_id = ?", *riskCase.BrandID)
	}
	var withdrawal model.WithdrawalRequest
	if err := query.Order("id desc").First(&withdrawal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &withdrawal, nil
}

func (r *gormRepository) LoadAgentAccountByAgentID(agentID uint64) (*model.AgentAccount, error) {
	var account model.AgentAccount
	if err := r.db.Where("agent_id = ?", agentID).First(&account).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (r *gormRepository) LoadLatestWithdrawal(agentID uint64) (*model.WithdrawalRequest, error) {
	var latestWithdrawal model.WithdrawalRequest
	if err := r.db.Where("agent_id = ?", agentID).Order("id desc").First(&latestWithdrawal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &latestWithdrawal, nil
}

func (r *gormRepository) LoadLedgerRiskMetrics(agentID uint64) (float64, float64, error) {
	type ledgerAgg struct {
		IncomeAmount        float64
		ManualReverseAmount float64
	}
	var agg ledgerAgg
	err := r.db.Model(&model.AgentAccountLedger{}).
		Select(strings.Join([]string{
			"COALESCE(SUM(CASE WHEN ledger_type = 'income' THEN amount ELSE 0 END), 0) AS income_amount",
			"COALESCE(SUM(CASE WHEN ledger_type = 'reverse' AND reference_type <> 'withdrawal_request' THEN amount ELSE 0 END), 0) AS manual_reverse_amount",
		}, ", ")).
		Where("agent_id = ?", agentID).
		Scan(&agg).Error
	return agg.IncomeAmount, agg.ManualReverseAmount, err
}

func (r *gormRepository) EnsureAgentExists(agentID uint64) error {
	var count int64
	if err := r.db.Model(&model.Agent{}).Where("id = ?", agentID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
