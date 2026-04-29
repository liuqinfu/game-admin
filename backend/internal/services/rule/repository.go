package rule

import (
	"strings"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListRules(scope sharedsvc.Scope, filter ListFilter) ([]model.CommissionRule, error)
	ListRulesForGame(game model.Game, filter OpenAPIListFilter) ([]model.CommissionRule, error)
	FindAgent(id uint64) (model.Agent, error)
	FindGame(id uint64) (model.Game, error)
	CreateRule(rule *model.CommissionRule) error
	FindRule(id uint64) (model.CommissionRule, error)
	DisablePublishedRulesTx(tx *gorm.DB, uniqueKey string, ruleID uint64, tenantID, brandID *uint64) error
	SaveRuleTx(tx *gorm.DB, rule *model.CommissionRule) error
	CreateRuleSnapshotTx(tx *gorm.DB, snapshot *model.RuleSnapshot) error
	ResolveEnumDictionary(code string) (sharedsvc.EnumDictionaryResponse, error)
	ResolveEnumDictionaryCached(code string) (sharedsvc.EnumDictionaryResponse, error)
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListRules(scope sharedsvc.Scope, filter ListFilter) ([]model.CommissionRule, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if value := strings.TrimSpace(filter.Scope); value != "" {
		query = query.Where("scope = ?", value)
	}
	if value := strings.TrimSpace(filter.GameID); value != "" {
		query = query.Where("game_id = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("agent_id = ?", value)
	}
	var items []model.CommissionRule
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListRulesForGame(game model.Game, filter OpenAPIListFilter) ([]model.CommissionRule, error) {
	query := r.db.Order("id desc").Where("game_id = ? OR scope IN ?", game.ID, []model.RuleScope{model.RuleScopePlatform, model.RuleScopeAgent})
	if game.TenantID != nil {
		query = query.Where("tenant_id IS NULL OR tenant_id = ?", *game.TenantID)
	}
	if game.BrandID != nil {
		query = query.Where("brand_id IS NULL OR brand_id = ?", *game.BrandID)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.Scope); value != "" {
		query = query.Where("scope = ?", value)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("agent_id = ? OR agent_id IS NULL", value)
	}
	var items []model.CommissionRule
	return items, query.Find(&items).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) FindGame(id uint64) (model.Game, error) {
	var game model.Game
	return game, r.db.First(&game, id).Error
}

func (r *gormRepository) CreateRule(rule *model.CommissionRule) error { return r.db.Create(rule).Error }

func (r *gormRepository) FindRule(id uint64) (model.CommissionRule, error) {
	var rule model.CommissionRule
	return rule, r.db.First(&rule, id).Error
}

func (r *gormRepository) DisablePublishedRulesTx(tx *gorm.DB, uniqueKey string, ruleID uint64, tenantID, brandID *uint64) error {
	query := tx.Model(&model.CommissionRule{}).Where("unique_key = ? AND id <> ? AND status = ?", uniqueKey, ruleID, model.RuleStatusPublished)
	query = sharedsvc.ApplyExactTenantBrandScope(query, tenantID, brandID, "tenant_id", "brand_id")
	return query.Updates(map[string]any{"status": model.RuleStatusDisabled}).Error
}

func (r *gormRepository) SaveRuleTx(tx *gorm.DB, rule *model.CommissionRule) error {
	return tx.Save(rule).Error
}

func (r *gormRepository) CreateRuleSnapshotTx(tx *gorm.DB, snapshot *model.RuleSnapshot) error {
	return tx.Create(snapshot).Error
}

func (r *gormRepository) ResolveEnumDictionary(code string) (sharedsvc.EnumDictionaryResponse, error) {
	return sharedsvc.ResolveEnumDictionary(r.db, code)
}

func (r *gormRepository) ResolveEnumDictionaryCached(code string) (sharedsvc.EnumDictionaryResponse, error) {
	return sharedsvc.ResolveEnumDictionaryCached(r.db, code)
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error { return r.db.Transaction(run) }
