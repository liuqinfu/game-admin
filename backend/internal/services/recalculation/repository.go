package recalculation

import (
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListTasks(scope sharedsvc.Scope, statusValue string) ([]model.RecalculationTask, error)
	FindSettlementBillInScope(scope sharedsvc.Scope, id uint64) (model.SettlementBill, error)
	FindSettlementBill(id uint64) (model.SettlementBill, error)
	FindAgent(id uint64) (model.Agent, error)
	CountAgent(agentID uint64) (int64, error)
	CurrentAgentScopeIDs(agentID uint64) ([]uint64, error)
	LoadAgentName(agentID uint64) string
	LoadSettlementBillNo(billID uint64) string
	CreateTask(task *model.RecalculationTask) error
	UpdateTaskResult(taskID uint64, updates map[string]any) error
	ReloadTask(id uint64) (model.RecalculationTask, error)
	ListRechargeOrdersForRecalculationTx(tx *gorm.DB, agentID uint64, periodStart, periodEnd time.Time) ([]model.RechargeOrder, error)
	FindBindingTx(tx *gorm.DB, id uint64) (model.Binding, error)
	ListActiveRelationsByDescendantTx(tx *gorm.DB, agentID uint64) ([]model.Relation, error)
	ListExistingCommissionRecordsTx(tx *gorm.DB, rechargeOrderID uint64) ([]model.CommissionRecord, error)
	UpdateAgentAccountForReversalTx(tx *gorm.DB, accountID uint64, balance, withdrawable float64) error
	CreateLedgerTx(tx *gorm.DB, ledger *model.AgentAccountLedger) error
	CreateCommissionRecordTx(tx *gorm.DB, record *model.CommissionRecord) error
	UpdateCommissionRecordAfterRecalcTx(tx *gorm.DB, recordID uint64, settledAt *time.Time, remark string) error
	FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error)
	UpdateAgentAccountSettlementTx(tx *gorm.DB, accountID uint64, balance, withdrawable float64, occurredAt time.Time, incomeDelta *float64) error
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListTasks(scope sharedsvc.Scope, statusValue string) ([]model.RecalculationTask, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("recalculation_task.id desc"), scope, "recalculation_task.tenant_id", "recalculation_task.brand_id")
	if statusValue != "" {
		query = query.Where("recalculation_task.status = ?", statusValue)
	}
	var tasks []model.RecalculationTask
	return tasks, query.Find(&tasks).Error
}

func (r *gormRepository) FindSettlementBillInScope(scope sharedsvc.Scope, id uint64) (model.SettlementBill, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Model(&model.SettlementBill{}), scope, "tenant_id", "brand_id")
	var bill model.SettlementBill
	return bill, query.Select("id,agent_id,tenant_id,brand_id").First(&bill, id).Error
}

func (r *gormRepository) FindSettlementBill(id uint64) (model.SettlementBill, error) {
	var bill model.SettlementBill
	return bill, r.db.First(&bill, id).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) CountAgent(agentID uint64) (int64, error) {
	var count int64
	return count, r.db.Model(&model.Agent{}).Where("id = ?", agentID).Count(&count).Error
}

func (r *gormRepository) CurrentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return sharedsvc.CurrentAgentScopeIDs(r.db, agentID)
}

func (r *gormRepository) LoadAgentName(agentID uint64) string {
	var agent model.Agent
	if err := r.db.Select("name").First(&agent, agentID).Error; err != nil {
		return ""
	}
	return agent.Name
}

func (r *gormRepository) LoadSettlementBillNo(billID uint64) string {
	var bill model.SettlementBill
	if err := r.db.Select("bill_no").First(&bill, billID).Error; err != nil {
		return ""
	}
	return bill.BillNo
}

func (r *gormRepository) CreateTask(task *model.RecalculationTask) error {
	return r.db.Create(task).Error
}

func (r *gormRepository) UpdateTaskResult(taskID uint64, updates map[string]any) error {
	return r.db.Model(&model.RecalculationTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *gormRepository) ReloadTask(id uint64) (model.RecalculationTask, error) {
	var task model.RecalculationTask
	return task, r.db.First(&task, id).Error
}

func (r *gormRepository) ListRechargeOrdersForRecalculationTx(tx *gorm.DB, agentID uint64, periodStart, periodEnd time.Time) ([]model.RechargeOrder, error) {
	var orders []model.RechargeOrder
	err := tx.Where("agent_id = ? AND paid_at IS NOT NULL AND paid_at >= ? AND paid_at < ? AND status = ?", agentID, periodStart, periodEnd, model.OrderStatusPaid).
		Order("paid_at asc, id asc").Find(&orders).Error
	return orders, err
}

func (r *gormRepository) FindBindingTx(tx *gorm.DB, id uint64) (model.Binding, error) {
	var binding model.Binding
	return binding, tx.First(&binding, id).Error
}

func (r *gormRepository) ListActiveRelationsByDescendantTx(tx *gorm.DB, agentID uint64) ([]model.Relation, error) {
	var ancestors []model.Relation
	return ancestors, tx.Where("descendant_agent_id = ? AND status = ?", agentID, model.RelationStatusActive).Order("depth asc").Find(&ancestors).Error
}

func (r *gormRepository) ListExistingCommissionRecordsTx(tx *gorm.DB, rechargeOrderID uint64) ([]model.CommissionRecord, error) {
	var existing []model.CommissionRecord
	return existing, tx.Where("recharge_order_id = ? AND reversed_from_id IS NULL", rechargeOrderID).Order("id asc").Find(&existing).Error
}

func (r *gormRepository) UpdateAgentAccountForReversalTx(tx *gorm.DB, accountID uint64, balance, withdrawable float64) error {
	return tx.Model(&model.AgentAccount{}).Where("id = ?", accountID).Updates(map[string]any{
		"balance":             balance,
		"withdrawable_amount": withdrawable,
		"version":             gorm.Expr("version + 1"),
	}).Error
}

func (r *gormRepository) CreateLedgerTx(tx *gorm.DB, ledger *model.AgentAccountLedger) error {
	return tx.Create(ledger).Error
}

func (r *gormRepository) CreateCommissionRecordTx(tx *gorm.DB, record *model.CommissionRecord) error {
	return tx.Create(record).Error
}

func (r *gormRepository) UpdateCommissionRecordAfterRecalcTx(tx *gorm.DB, recordID uint64, settledAt *time.Time, remark string) error {
	return tx.Model(&model.CommissionRecord{}).Where("id = ?", recordID).Updates(map[string]any{
		"status":     model.CommissionStatusReversed,
		"settled_at": settledAt,
		"remark":     remark,
	}).Error
}

func (r *gormRepository) FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, tx.First(&agent, id).Error
}

func (r *gormRepository) UpdateAgentAccountSettlementTx(tx *gorm.DB, accountID uint64, balance, withdrawable float64, occurredAt time.Time, incomeDelta *float64) error {
	updates := map[string]any{
		"balance":             balance,
		"available_balance":   balance,
		"withdrawable_amount": withdrawable,
		"last_settled_at":     occurredAt,
		"version":             gorm.Expr("version + 1"),
	}
	if incomeDelta != nil {
		updates["total_income"] = gorm.Expr("total_income + ?", *incomeDelta)
	}
	return tx.Model(&model.AgentAccount{}).Where("id = ?", accountID).Updates(updates).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}
