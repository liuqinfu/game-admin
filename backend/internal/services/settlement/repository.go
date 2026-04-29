package settlement

import (
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListBills(scope sharedsvc.Scope, filter BillFilter) ([]model.SettlementBill, error)
	FindAgent(id uint64) (model.Agent, error)
	FindScopedBill(scope sharedsvc.Scope, id uint64) (model.SettlementBill, error)
	ListBillDetails(billID uint64) ([]model.SettlementBillDetail, error)
	LoadRechargeOrderNo(orderID *uint64) string
	LoadAgentName(agentID uint64) string
	ListCommissionRecordsForBill(agentID uint64, periodStart, periodEnd time.Time) ([]model.CommissionRecord, error)
	CreateBillTx(tx *gorm.DB, bill *model.SettlementBill) error
	CreateBillDetailsTx(tx *gorm.DB, details []model.SettlementBillDetail) error
	UpdateBillConfirmationTx(tx *gorm.DB, id uint64, status model.SettlementBillStatus, confirmedAt *time.Time, confirmedBy string, updatedAt time.Time) error
	ReloadBillTx(tx *gorm.DB, id uint64) (model.SettlementBill, error)
	UpdateBillSummaryPayload(id uint64, payload []byte, updatedAt time.Time) error
	ReloadBill(id uint64) (model.SettlementBill, error)
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListBills(scope sharedsvc.Scope, filter BillFilter) ([]model.SettlementBill, error) {
	query := r.db.Model(&model.SettlementBill{}).
		Joins("JOIN agent ON agent.id = settlement_bill.agent_id").
		Order("settlement_bill.id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "settlement_bill.tenant_id", "settlement_bill.brand_id", "TRIM(agent.remark)")
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("settlement_bill.status = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("settlement_bill.agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.BillNo); value != "" {
		query = query.Where("settlement_bill.bill_no = ?", value)
	}
	var bills []model.SettlementBill
	return bills, query.Find(&bills).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) FindScopedBill(scope sharedsvc.Scope, id uint64) (model.SettlementBill, error) {
	query := r.db.Model(&model.SettlementBill{}).Joins("JOIN agent ON agent.id = settlement_bill.agent_id")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "settlement_bill.tenant_id", "settlement_bill.brand_id", "TRIM(agent.remark)")
	var bill model.SettlementBill
	return bill, query.First(&bill, id).Error
}

func (r *gormRepository) ListBillDetails(billID uint64) ([]model.SettlementBillDetail, error) {
	var rows []model.SettlementBillDetail
	return rows, r.db.Where("settlement_bill_id = ?", billID).Order("occurred_at asc, id asc").Find(&rows).Error
}

func (r *gormRepository) LoadRechargeOrderNo(orderID *uint64) string {
	if orderID == nil || *orderID == 0 {
		return ""
	}
	var order model.RechargeOrder
	if err := r.db.Select("order_no").First(&order, *orderID).Error; err != nil {
		return ""
	}
	return order.OrderNo
}

func (r *gormRepository) LoadAgentName(agentID uint64) string {
	var agent model.Agent
	if err := r.db.Select("name").First(&agent, agentID).Error; err != nil {
		return ""
	}
	return agent.Name
}

func (r *gormRepository) ListCommissionRecordsForBill(agentID uint64, periodStart, periodEnd time.Time) ([]model.CommissionRecord, error) {
	var records []model.CommissionRecord
	return records, r.db.Where("agent_id = ? AND estimated_at >= ? AND estimated_at < ?", agentID, periodStart, periodEnd).
		Order("estimated_at asc, id asc").
		Find(&records).Error
}

func (r *gormRepository) CreateBillTx(tx *gorm.DB, bill *model.SettlementBill) error {
	return tx.Create(bill).Error
}

func (r *gormRepository) CreateBillDetailsTx(tx *gorm.DB, details []model.SettlementBillDetail) error {
	return tx.Create(&details).Error
}

func (r *gormRepository) UpdateBillConfirmationTx(tx *gorm.DB, id uint64, status model.SettlementBillStatus, confirmedAt *time.Time, confirmedBy string, updatedAt time.Time) error {
	return tx.Model(&model.SettlementBill{}).Where("id = ?", id).Updates(map[string]any{
		"status":       status,
		"confirmed_at": confirmedAt,
		"confirmed_by": confirmedBy,
		"updated_at":   updatedAt,
	}).Error
}

func (r *gormRepository) ReloadBillTx(tx *gorm.DB, id uint64) (model.SettlementBill, error) {
	var bill model.SettlementBill
	return bill, tx.First(&bill, id).Error
}

func (r *gormRepository) UpdateBillSummaryPayload(id uint64, payload []byte, updatedAt time.Time) error {
	return r.db.Model(&model.SettlementBill{}).Where("id = ?", id).Updates(map[string]any{
		"summary_payload": payload,
		"updated_at":      updatedAt,
	}).Error
}

func (r *gormRepository) ReloadBill(id uint64) (model.SettlementBill, error) {
	var bill model.SettlementBill
	return bill, r.db.First(&bill, id).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}
