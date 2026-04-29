package withdrawal

import (
	"strings"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListWithdrawals(scope sharedsvc.Scope, filter ListFilter) ([]model.WithdrawalRequest, error)
	FindWithdrawal(id uint64) (model.WithdrawalRequest, error)
	FindAgent(id uint64) (model.Agent, error)
	LoadAgentName(agentID uint64) string
	ListRiskSignals(scope sharedsvc.Scope, limit int) ([]RiskSignal, error)
	FindLatestRiskWithdrawal(query LatestRiskWithdrawalQuery) (*model.WithdrawalRequest, error)
	CurrentAgentScopeIDs(agentID uint64) ([]uint64, error)
	EnsureAgentExists(agentID uint64) error
	SaveWithdrawalTx(tx *gorm.DB, withdrawal *model.WithdrawalRequest) error
	CreateWithdrawalTx(tx *gorm.DB, withdrawal *model.WithdrawalRequest) error
	WithTx(func(*gorm.DB) error) error
}

type RiskSignal struct {
	model.WithdrawalRequest
	AgentName string
}

type LatestRiskWithdrawalQuery struct {
	AgentID  uint64
	Currency string
	TenantID *uint64
	BrandID  *uint64
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListWithdrawals(scope sharedsvc.Scope, filter ListFilter) ([]model.WithdrawalRequest, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("withdrawal_request.id desc"), scope, "withdrawal_request.tenant_id", "withdrawal_request.brand_id")
	if value := strings.TrimSpace(filter.TenantCode); value != "" {
		query = query.Where("withdrawal_request.tenant_code = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("withdrawal_request.status = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("withdrawal_request.agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.RequestNo); value != "" {
		query = query.Where("withdrawal_request.request_no = ?", value)
	}
	var rows []model.WithdrawalRequest
	return rows, query.Find(&rows).Error
}

func (r *gormRepository) FindWithdrawal(id uint64) (model.WithdrawalRequest, error) {
	var item model.WithdrawalRequest
	return item, r.db.First(&item, id).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) LoadAgentName(agentID uint64) string {
	var agent model.Agent
	if err := r.db.Select("name").First(&agent, agentID).Error; err != nil {
		return ""
	}
	return agent.Name
}

func (r *gormRepository) ListRiskSignals(scope sharedsvc.Scope, limit int) ([]RiskSignal, error) {
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
	var rows []RiskSignal
	if limit > 0 {
		query = query.Limit(limit)
	}
	return rows, query.Find(&rows).Error
}

func (r *gormRepository) FindLatestRiskWithdrawal(query LatestRiskWithdrawalQuery) (*model.WithdrawalRequest, error) {
	dbQuery := r.db.Model(&model.WithdrawalRequest{}).Where("agent_id = ?", query.AgentID)
	if value := strings.TrimSpace(query.Currency); value != "" {
		dbQuery = dbQuery.Where("currency = ?", value)
	}
	if query.TenantID != nil {
		dbQuery = dbQuery.Where("tenant_id = ?", *query.TenantID)
	}
	if query.BrandID != nil {
		dbQuery = dbQuery.Where("brand_id = ?", *query.BrandID)
	}
	var item model.WithdrawalRequest
	if err := dbQuery.Order("id desc").First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) CurrentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return sharedsvc.CurrentAgentScopeIDs(r.db, agentID)
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

func (r *gormRepository) SaveWithdrawalTx(tx *gorm.DB, withdrawal *model.WithdrawalRequest) error {
	return tx.Save(withdrawal).Error
}

func (r *gormRepository) CreateWithdrawalTx(tx *gorm.DB, withdrawal *model.WithdrawalRequest) error {
	return tx.Create(withdrawal).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}
