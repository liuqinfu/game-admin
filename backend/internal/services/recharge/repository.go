package recharge

import (
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

type Repository interface {
	FindGame(id uint64) (model.Game, error)
	FindPlayerTx(tx *gorm.DB, id uint64) (model.Player, error)
	FindGameTx(tx *gorm.DB, id uint64) (model.Game, error)
	FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error)
	FindRechargeOrderByIdempotencyKeyTx(tx *gorm.DB, idempotencyKey string) (model.RechargeOrder, error)
	FindRechargeOrderByOrderNoTx(tx *gorm.DB, orderNo string) (model.RechargeOrder, error)
	CreateRechargeCallbackLogTx(tx *gorm.DB, callbackLog *model.RechargeCallbackLog) error
	FindBoundBindingByPlayerTx(tx *gorm.DB, playerID uint64) (model.Binding, error)
	ListActiveRelationsByDescendantTx(tx *gorm.DB, agentID uint64) ([]model.Relation, error)
	ListActiveRelationClosuresByDescendantTx(tx *gorm.DB, agentID uint64) ([]model.AgentRelationClosure, error)
	FindLatestFreezeLedgerByAgentTx(tx *gorm.DB, agentID uint64) (model.AgentAccountLedger, error)
	CreateRechargeOrderTx(tx *gorm.DB, order *model.RechargeOrder) error
	FindAgentGameAccessTx(tx *gorm.DB, agentID, gameID uint64) (model.AgentGameAccess, error)
	CountRechargeReversalRecordsTx(tx *gorm.DB, rechargeOrderID uint64) (int64, error)
	UpdateRechargeOrderRefundedTx(tx *gorm.DB, orderID uint64, occurredAt time.Time) error
	ListRechargeCommissionRecordsTx(tx *gorm.DB, rechargeOrderID uint64) ([]model.CommissionRecord, error)
	CreateCommissionRecordTx(tx *gorm.DB, record *model.CommissionRecord) error
	UpdateCommissionRecordReversedTx(tx *gorm.DB, recordID uint64, occurredAt time.Time) error
	UpdateAgentAccountSettlementTx(tx *gorm.DB, accountID uint64, balance, withdrawable float64, occurredAt time.Time, incomeDelta, reversedDelta *float64) error
	CreateAgentAccountLedgerTx(tx *gorm.DB, ledger *model.AgentAccountLedger) error
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindGame(id uint64) (model.Game, error) {
	var game model.Game
	return game, r.db.First(&game, id).Error
}

func (r *gormRepository) FindPlayerTx(tx *gorm.DB, id uint64) (model.Player, error) {
	var player model.Player
	return player, tx.First(&player, id).Error
}

func (r *gormRepository) FindGameTx(tx *gorm.DB, id uint64) (model.Game, error) {
	var game model.Game
	return game, tx.First(&game, id).Error
}

func (r *gormRepository) FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, tx.First(&agent, id).Error
}

func (r *gormRepository) FindRechargeOrderByIdempotencyKeyTx(tx *gorm.DB, idempotencyKey string) (model.RechargeOrder, error) {
	var order model.RechargeOrder
	return order, tx.Where("idempotency_key = ?", strings.TrimSpace(idempotencyKey)).First(&order).Error
}

func (r *gormRepository) FindRechargeOrderByOrderNoTx(tx *gorm.DB, orderNo string) (model.RechargeOrder, error) {
	var order model.RechargeOrder
	return order, tx.Where("order_no = ?", strings.TrimSpace(orderNo)).First(&order).Error
}

func (r *gormRepository) CreateRechargeCallbackLogTx(tx *gorm.DB, callbackLog *model.RechargeCallbackLog) error {
	return tx.Create(callbackLog).Error
}

func (r *gormRepository) FindBoundBindingByPlayerTx(tx *gorm.DB, playerID uint64) (model.Binding, error) {
	var binding model.Binding
	return binding, tx.Where("player_id = ? AND status = ?", playerID, model.BindingStatusBound).First(&binding).Error
}

func (r *gormRepository) ListActiveRelationsByDescendantTx(tx *gorm.DB, agentID uint64) ([]model.Relation, error) {
	var ancestors []model.Relation
	return ancestors, tx.Where("descendant_agent_id = ? AND status = ?", agentID, model.RelationStatusActive).Order("depth asc").Find(&ancestors).Error
}

func (r *gormRepository) ListActiveRelationClosuresByDescendantTx(tx *gorm.DB, agentID uint64) ([]model.AgentRelationClosure, error) {
	var closures []model.AgentRelationClosure
	return closures, tx.Where("descendant_agent_id = ? AND status = ?", agentID, model.RelationStatusActive).Order("depth asc").Find(&closures).Error
}

func (r *gormRepository) FindLatestFreezeLedgerByAgentTx(tx *gorm.DB, agentID uint64) (model.AgentAccountLedger, error) {
	var ledger model.AgentAccountLedger
	return ledger, tx.Where("agent_id = ? AND ledger_type = ?", agentID, model.LedgerTypeFreeze).Order("id desc").First(&ledger).Error
}

func (r *gormRepository) CreateRechargeOrderTx(tx *gorm.DB, order *model.RechargeOrder) error {
	return tx.Create(order).Error
}

func (r *gormRepository) FindAgentGameAccessTx(tx *gorm.DB, agentID, gameID uint64) (model.AgentGameAccess, error) {
	var access model.AgentGameAccess
	return access, tx.Where("agent_id = ? AND game_id = ?", agentID, gameID).First(&access).Error
}

func (r *gormRepository) CountRechargeReversalRecordsTx(tx *gorm.DB, rechargeOrderID uint64) (int64, error) {
	var count int64
	err := tx.Model(&model.CommissionRecord{}).Where("recharge_order_id = ? AND reversed_from_id IS NOT NULL", rechargeOrderID).Count(&count).Error
	return count, err
}

func (r *gormRepository) UpdateRechargeOrderRefundedTx(tx *gorm.DB, orderID uint64, occurredAt time.Time) error {
	return tx.Model(&model.RechargeOrder{}).Where("id = ?", orderID).Updates(map[string]any{
		"status":      model.OrderStatusRefunded,
		"callback_at": occurredAt,
	}).Error
}

func (r *gormRepository) ListRechargeCommissionRecordsTx(tx *gorm.DB, rechargeOrderID uint64) ([]model.CommissionRecord, error) {
	var records []model.CommissionRecord
	return records, tx.Where("recharge_order_id = ? AND reversed_from_id IS NULL", rechargeOrderID).Order("id asc").Find(&records).Error
}

func (r *gormRepository) CreateCommissionRecordTx(tx *gorm.DB, record *model.CommissionRecord) error {
	return tx.Create(record).Error
}

func (r *gormRepository) UpdateCommissionRecordReversedTx(tx *gorm.DB, recordID uint64, occurredAt time.Time) error {
	return tx.Model(&model.CommissionRecord{}).Where("id = ?", recordID).Updates(map[string]any{
		"status":     model.CommissionStatusReversed,
		"settled_at": occurredAt,
	}).Error
}

func (r *gormRepository) UpdateAgentAccountSettlementTx(tx *gorm.DB, accountID uint64, balance, withdrawable float64, occurredAt time.Time, incomeDelta, reversedDelta *float64) error {
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
	if reversedDelta != nil {
		updates["total_reversed"] = gorm.Expr("total_reversed + ?", *reversedDelta)
	}
	return tx.Model(&model.AgentAccount{}).Where("id = ?", accountID).Updates(updates).Error
}

func (r *gormRepository) CreateAgentAccountLedgerTx(tx *gorm.DB, ledger *model.AgentAccountLedger) error {
	return tx.Create(ledger).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}
