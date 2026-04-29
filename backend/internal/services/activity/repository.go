package activity

import (
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListRules(scope sharedsvc.Scope, filter RuleListFilter) ([]model.ActivityRewardRule, error)
	CreateRule(rule *model.ActivityRewardRule) error
	FindRuleInScope(scope sharedsvc.Scope, id uint64) (model.ActivityRewardRule, error)
	SaveRule(rule *model.ActivityRewardRule) error
	ListRecords(scope sharedsvc.Scope, filter RecordListFilter) ([]model.ActivityRewardRecord, error)
	FindRuleTx(tx *gorm.DB, id uint64) (model.ActivityRewardRule, error)
	CountRewardRecordsByRuleTx(tx *gorm.DB, ruleID uint64) (int64, error)
	CountDailyRewardRecordsByRuleTx(tx *gorm.DB, ruleID uint64, startOfDay, endOfDay time.Time) (int64, error)
	FindRewardRecordByReferenceTx(tx *gorm.DB, ruleID uint64, referenceType, referenceID string) (model.ActivityRewardRecord, error)
	CreateRewardRecordTx(tx *gorm.DB, record *model.ActivityRewardRecord) error
	FindRecordInScope(scope sharedsvc.Scope, recordID uint64) (model.ActivityRewardRecord, error)
	FindRecordTx(tx *gorm.DB, recordID uint64) (model.ActivityRewardRecord, error)
	SaveRecordTx(tx *gorm.DB, record *model.ActivityRewardRecord) error
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListRules(scope sharedsvc.Scope, filter RuleListFilter) ([]model.ActivityRewardRule, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Model(&model.ActivityRewardRule{}).Order("id desc"), scope, "tenant_id", "brand_id")
	if value := strings.TrimSpace(filter.TenantID); value != "" {
		query = query.Where("tenant_id = ?", value)
	}
	if value := strings.TrimSpace(filter.BrandID); value != "" {
		query = query.Where("brand_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.ActivityType); value != "" {
		query = query.Where("activity_type = ?", value)
	}
	if value := strings.TrimSpace(filter.Keyword); value != "" {
		like := "%" + value + "%"
		query = query.Where("name LIKE ? OR remark LIKE ?", like, like)
	}
	var items []model.ActivityRewardRule
	return items, query.Find(&items).Error
}

func (r *gormRepository) CreateRule(rule *model.ActivityRewardRule) error {
	return r.db.Create(rule).Error
}

func (r *gormRepository) FindRuleInScope(scope sharedsvc.Scope, id uint64) (model.ActivityRewardRule, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Model(&model.ActivityRewardRule{}), scope, "tenant_id", "brand_id")
	var rule model.ActivityRewardRule
	return rule, query.First(&rule, id).Error
}

func (r *gormRepository) SaveRule(rule *model.ActivityRewardRule) error { return r.db.Save(rule).Error }

func (r *gormRepository) ListRecords(scope sharedsvc.Scope, filter RecordListFilter) ([]model.ActivityRewardRecord, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Model(&model.ActivityRewardRecord{}).Order("id desc"), scope, "tenant_id", "brand_id")
	if value := strings.TrimSpace(filter.TenantID); value != "" {
		query = query.Where("tenant_id = ?", value)
	}
	if value := strings.TrimSpace(filter.BrandID); value != "" {
		query = query.Where("brand_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.Keyword); value != "" {
		like := "%" + value + "%"
		query = query.Where("reference_id LIKE ? OR record_no LIKE ? OR rule_name LIKE ? OR remark LIKE ?", like, like, like, like)
	}
	var items []model.ActivityRewardRecord
	return items, query.Find(&items).Error
}

func (r *gormRepository) FindRuleTx(tx *gorm.DB, id uint64) (model.ActivityRewardRule, error) {
	var rule model.ActivityRewardRule
	return rule, tx.First(&rule, id).Error
}

func (r *gormRepository) CountRewardRecordsByRuleTx(tx *gorm.DB, ruleID uint64) (int64, error) {
	var total int64
	return total, tx.Model(&model.ActivityRewardRecord{}).Where("rule_id = ?", ruleID).Count(&total).Error
}

func (r *gormRepository) CountDailyRewardRecordsByRuleTx(tx *gorm.DB, ruleID uint64, startOfDay, endOfDay time.Time) (int64, error) {
	var daily int64
	return daily, tx.Model(&model.ActivityRewardRecord{}).Where("rule_id = ? AND created_at >= ? AND created_at < ?", ruleID, startOfDay, endOfDay).Count(&daily).Error
}

func (r *gormRepository) FindRewardRecordByReferenceTx(tx *gorm.DB, ruleID uint64, referenceType, referenceID string) (model.ActivityRewardRecord, error) {
	var existing model.ActivityRewardRecord
	return existing, tx.Where("rule_id = ? AND reference_type = ? AND reference_id = ?", ruleID, referenceType, referenceID).First(&existing).Error
}

func (r *gormRepository) CreateRewardRecordTx(tx *gorm.DB, record *model.ActivityRewardRecord) error {
	return tx.Create(record).Error
}

func (r *gormRepository) FindRecordInScope(scope sharedsvc.Scope, recordID uint64) (model.ActivityRewardRecord, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Model(&model.ActivityRewardRecord{}), scope, "tenant_id", "brand_id")
	var record model.ActivityRewardRecord
	return record, query.First(&record, recordID).Error
}

func (r *gormRepository) FindRecordTx(tx *gorm.DB, recordID uint64) (model.ActivityRewardRecord, error) {
	var record model.ActivityRewardRecord
	return record, tx.First(&record, recordID).Error
}

func (r *gormRepository) SaveRecordTx(tx *gorm.DB, record *model.ActivityRewardRecord) error {
	return tx.Save(record).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error { return r.db.Transaction(run) }
