package account

import (
	"strings"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListOrders(scope sharedsvc.Scope, filter OrderListFilter, agentIDs []uint64) ([]model.RechargeOrder, error)
	FindRechargeOrderInScope(scope sharedsvc.Scope, id uint64) (model.RechargeOrder, error)
	SaveRechargeOrder(order *model.RechargeOrder) error
	ListCommissions(scope sharedsvc.Scope, filter CommissionListFilter) ([]model.CommissionRecord, error)
	ListLedgers(scope sharedsvc.Scope, filter LedgerListFilter) ([]model.AgentAccountLedger, error)
	ListRiskAccounts(scope sharedsvc.Scope, agentID string) ([]RiskAccountSnapshot, error)
	GetRiskAccountSnapshot(scope sharedsvc.Scope, agentID uint64) (RiskAccountSnapshot, error)
	FindLedgerByIdempotencyKey(idempotencyKey string) (model.AgentAccountLedger, error)
	FindAgent(id uint64) (model.Agent, error)
	CurrentAgentScopeIDs(agentID uint64) ([]uint64, error)
}

type RiskAccountSnapshot struct {
	AgentID             uint64
	AgentName           string
	Account             *model.AgentAccount
	IncomeAmount        float64
	ManualReverseAmount float64
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListOrders(scope sharedsvc.Scope, filter OrderListFilter, agentIDs []uint64) ([]model.RechargeOrder, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if len(agentIDs) > 0 {
		query = query.Where("agent_id IN ?", agentIDs)
	} else if scope.AgentID != nil {
		query = query.Where("agent_id = ?", *scope.AgentID)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.GameID); value != "" {
		query = query.Where("game_id = ?", value)
	}
	if value := strings.TrimSpace(filter.OrderNo); value != "" {
		query = query.Where("order_no LIKE ?", "%"+value+"%")
	}
	var items []model.RechargeOrder
	return items, query.Find(&items).Error
}

func (r *gormRepository) FindRechargeOrderInScope(scope sharedsvc.Scope, id uint64) (model.RechargeOrder, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Model(&model.RechargeOrder{}), scope, "tenant_id", "brand_id")
	if scope.AgentID != nil {
		query = query.Where("agent_id = ?", *scope.AgentID)
	}
	var order model.RechargeOrder
	return order, query.Where("id = ?", id).First(&order).Error
}

func (r *gormRepository) SaveRechargeOrder(order *model.RechargeOrder) error {
	return r.db.Save(order).Error
}

func (r *gormRepository) ListCommissions(scope sharedsvc.Scope, filter CommissionListFilter) ([]model.CommissionRecord, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("commission_record.id desc"), scope, "commission_record.tenant_id", "commission_record.brand_id")
	if scope.AgentID != nil {
		query = query.Where("commission_record.agent_id = ?", *scope.AgentID)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("commission_record.status = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("commission_record.agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.GameID); value != "" {
		query = query.Where("commission_record.game_id = ?", value)
	}
	if value := strings.TrimSpace(filter.OrderNo); value != "" {
		query = query.Joins("JOIN recharge_order ON recharge_order.id = commission_record.recharge_order_id").Where("recharge_order.order_no LIKE ?", "%"+value+"%")
	}
	var items []model.CommissionRecord
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListLedgers(scope sharedsvc.Scope, filter LedgerListFilter) ([]model.AgentAccountLedger, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("agent_account_ledger.id desc"), scope, "agent_account_ledger.tenant_id", "agent_account_ledger.brand_id")
	if scope.AgentID != nil {
		query = query.Where("agent_account_ledger.agent_id = ?", *scope.AgentID)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("agent_account_ledger.agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Type); value != "" {
		query = query.Where("agent_account_ledger.ledger_type = ?", value)
	}
	if value := strings.TrimSpace(filter.OrderNo); value != "" {
		query = query.
			Joins("JOIN commission_record ON commission_record.id = CAST(agent_account_ledger.reference_id AS INTEGER)").
			Joins("JOIN recharge_order ON recharge_order.id = commission_record.recharge_order_id").
			Where("agent_account_ledger.reference_type = ? AND recharge_order.order_no LIKE ?", "commission_record", "%"+value+"%")
	}
	var items []model.AgentAccountLedger
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListRiskAccounts(scope sharedsvc.Scope, agentID string) ([]RiskAccountSnapshot, error) {
	query := r.db.Model(&model.AgentAccount{}).
		Select("agent_account.*, agent.name as agent_name").
		Joins("JOIN agent ON agent.id = agent_account.agent_id").
		Order("agent_account.id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "agent_account.tenant_id", "agent_account.brand_id", "TRIM(agent.remark)")
	if value := strings.TrimSpace(agentID); value != "" {
		query = query.Where("agent_account.agent_id = ?", value)
	}
	type row struct {
		model.AgentAccount
		AgentName string
	}
	var rows []row
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]RiskAccountSnapshot, 0, len(rows))
	for _, row := range rows {
		type ledgerAgg struct {
			IncomeAmount        float64
			ManualReverseAmount float64
		}
		var agg ledgerAgg
		if err := r.db.Model(&model.AgentAccountLedger{}).
			Select(strings.Join([]string{
				"COALESCE(SUM(CASE WHEN ledger_type = 'income' THEN amount ELSE 0 END), 0) AS income_amount",
				"COALESCE(SUM(CASE WHEN ledger_type = 'reverse' AND reference_type <> 'withdrawal_request' THEN amount ELSE 0 END), 0) AS manual_reverse_amount",
			}, ", ")).
			Where("agent_id = ?", row.AgentID).
			Scan(&agg).Error; err != nil {
			return nil, err
		}
		account := row.AgentAccount
		items = append(items, RiskAccountSnapshot{
			AgentID:             row.AgentID,
			AgentName:           row.AgentName,
			Account:             &account,
			IncomeAmount:        agg.IncomeAmount,
			ManualReverseAmount: agg.ManualReverseAmount,
		})
	}
	return items, nil
}

func (r *gormRepository) GetRiskAccountSnapshot(scope sharedsvc.Scope, agentID uint64) (RiskAccountSnapshot, error) {
	query := r.db.Model(&model.AgentAccount{}).
		Select("agent_account.*, agent.name as agent_name").
		Joins("JOIN agent ON agent.id = agent_account.agent_id")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "agent_account.tenant_id", "agent_account.brand_id", "TRIM(agent.remark)")
	type row struct {
		model.AgentAccount
		AgentName string
	}
	var accountRow row
	if err := query.Where("agent_account.agent_id = ?", agentID).First(&accountRow).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return RiskAccountSnapshot{AgentID: agentID}, nil
		}
		return RiskAccountSnapshot{}, err
	}
	type ledgerAgg struct {
		IncomeAmount        float64
		ManualReverseAmount float64
	}
	var agg ledgerAgg
	if err := r.db.Model(&model.AgentAccountLedger{}).
		Select(strings.Join([]string{
			"COALESCE(SUM(CASE WHEN ledger_type = 'income' THEN amount ELSE 0 END), 0) AS income_amount",
			"COALESCE(SUM(CASE WHEN ledger_type = 'reverse' AND reference_type <> 'withdrawal_request' THEN amount ELSE 0 END), 0) AS manual_reverse_amount",
		}, ", ")).
		Where("agent_id = ?", agentID).
		Scan(&agg).Error; err != nil {
		return RiskAccountSnapshot{}, err
	}
	account := accountRow.AgentAccount
	return RiskAccountSnapshot{
		AgentID:             agentID,
		AgentName:           accountRow.AgentName,
		Account:             &account,
		IncomeAmount:        agg.IncomeAmount,
		ManualReverseAmount: agg.ManualReverseAmount,
	}, nil
}

func (r *gormRepository) FindLedgerByIdempotencyKey(idempotencyKey string) (model.AgentAccountLedger, error) {
	var ledger model.AgentAccountLedger
	return ledger, r.db.Where("idempotency_key = ?", strings.TrimSpace(idempotencyKey)).First(&ledger).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) CurrentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return sharedsvc.CurrentAgentScopeIDs(r.db, agentID)
}
