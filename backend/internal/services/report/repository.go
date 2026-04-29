package report

import (
	"fmt"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	CountBetween(scope sharedsvc.Scope, modelRef any, tenantField, brandField, timeField, tenantIDText, brandIDText string, start, end time.Time) (int64, error)
	SumBetween(scope sharedsvc.Scope, modelRef any, tenantField, brandField, timeField, amountField, tenantIDText, brandIDText string, start, end time.Time, filters ...any) (float64, error)
	CountNonNullBetween(scope sharedsvc.Scope, modelRef any, tenantField, brandField, timeField, amountField, tenantIDText, brandIDText string, start, end time.Time, filters ...any) (int64, error)
	CountFirstRechargePlayersScoped(scope sharedsvc.Scope, start, end time.Time) (int64, error)
	CalculateWithdrawalSuccessRateScoped(scope sharedsvc.Scope) (float64, error)
	CalculateRiskInterceptRateScoped(scope sharedsvc.Scope) (float64, error)
	LoadAgentTeamStats(agentID uint64) (teamStats, error)
	LoadLatestWithdrawal(agentID uint64) (model.WithdrawalRequest, error)
	LoadLedgerRiskMetrics(agentID uint64) (float64, float64, error)
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) baseScopedQuery(scope sharedsvc.Scope, modelRef any, tenantField, brandField, tenantIDText, brandIDText string) (*gorm.DB, error) {
	query := r.db.Model(modelRef)
	if tenantField != "" || brandField != "" {
		query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, tenantField, brandField, "")
	}
	return sharedsvc.ApplyScopedQueryFilters(query, tenantField, brandField, tenantIDText, brandIDText)
}

func (r *gormRepository) CountBetween(scope sharedsvc.Scope, modelRef any, tenantField, brandField, timeField, tenantIDText, brandIDText string, start, end time.Time) (int64, error) {
	query, err := r.baseScopedQuery(scope, modelRef, tenantField, brandField, tenantIDText, brandIDText)
	if err != nil {
		return 0, err
	}
	var total int64
	if err := query.Where(fmt.Sprintf("%s >= ? AND %s < ?", timeField, timeField), start, end).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *gormRepository) SumBetween(scope sharedsvc.Scope, modelRef any, tenantField, brandField, timeField, amountField, tenantIDText, brandIDText string, start, end time.Time, filters ...any) (float64, error) {
	query, err := r.baseScopedQuery(scope, modelRef, tenantField, brandField, tenantIDText, brandIDText)
	if err != nil {
		return 0, err
	}
	if len(filters) > 0 {
		query = query.Where(filters[0].(string), filters[1:]...)
	}
	type amountRow struct{ Amount float64 }
	var row amountRow
	if err := query.Select(fmt.Sprintf("COALESCE(SUM(%s), 0) AS amount", amountField)).
		Where(fmt.Sprintf("%s >= ? AND %s < ?", timeField, timeField), start, end).
		Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.Amount, nil
}

func (r *gormRepository) CountNonNullBetween(scope sharedsvc.Scope, modelRef any, tenantField, brandField, timeField, amountField, tenantIDText, brandIDText string, start, end time.Time, filters ...any) (int64, error) {
	query, err := r.baseScopedQuery(scope, modelRef, tenantField, brandField, tenantIDText, brandIDText)
	if err != nil {
		return 0, err
	}
	if len(filters) > 0 {
		query = query.Where(filters[0].(string), filters[1:]...)
	}
	var total int64
	if err := query.Where(fmt.Sprintf("%s >= ? AND %s < ?", timeField, timeField), start, end).
		Where(amountField + " IS NOT NULL").Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *gormRepository) CountFirstRechargePlayersScoped(scope sharedsvc.Scope, start, end time.Time) (int64, error) {
	type paidOrderRow struct {
		PlayerID uint64
		PaidAt   time.Time
	}
	query := r.db.Model(&model.RechargeOrder{}).
		Select("player_id, created_at AS paid_at").
		Where("status = ?", model.OrderStatusPaid).
		Order("player_id asc, created_at asc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "tenant_id", "brand_id", "")
	var rows []paidOrderRow
	if err := query.Find(&rows).Error; err != nil {
		return 0, err
	}
	seen := make(map[uint64]struct{}, len(rows))
	var total int64
	for _, row := range rows {
		if _, ok := seen[row.PlayerID]; ok {
			continue
		}
		seen[row.PlayerID] = struct{}{}
		if !row.PaidAt.Before(start) && row.PaidAt.Before(end) {
			total++
		}
	}
	return total, nil
}

func (r *gormRepository) CalculateWithdrawalSuccessRateScoped(scope sharedsvc.Scope) (float64, error) {
	type row struct {
		Paid     int64
		Failed   int64
		Returned int64
	}
	var result row
	query := sharedsvc.ApplyTenantBrandScopeWithLegacyCode(r.db.Model(&model.WithdrawalRequest{}), scope, "tenant_id", "brand_id", "")
	if err := query.Select(strings.Join([]string{
		"SUM(CASE WHEN status = 'paid' THEN 1 ELSE 0 END) AS paid",
		"SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed",
		"SUM(CASE WHEN status = 'returned' THEN 1 ELSE 0 END) AS returned",
	}, ", ")).Scan(&result).Error; err != nil {
		return 0, err
	}
	total := result.Paid + result.Failed + result.Returned
	if total == 0 {
		return 0, nil
	}
	return sharedsvc.Round2(float64(result.Paid) * 100 / float64(total)), nil
}

func (r *gormRepository) CalculateRiskInterceptRateScoped(scope sharedsvc.Scope) (float64, error) {
	type row struct {
		Total       int64
		Intercepted int64
	}
	var result row
	query := sharedsvc.ApplyTenantBrandScopeWithLegacyCode(r.db.Model(&model.RiskCase{}), scope, "tenant_id", "brand_id", "")
	if err := query.Select(strings.Join([]string{
		"COUNT(*) AS total",
		"SUM(CASE WHEN freeze_requested = 1 THEN 1 ELSE 0 END) AS intercepted",
	}, ", ")).Scan(&result).Error; err != nil {
		return 0, err
	}
	if result.Total == 0 {
		return 0, nil
	}
	return sharedsvc.Round2(float64(result.Intercepted) * 100 / float64(result.Total)), nil
}

func (r *gormRepository) LoadAgentTeamStats(agentID uint64) (teamStats, error) {
	stats := teamStats{}
	if err := r.db.Model(&model.AgentRelationClosure{}).
		Where("ancestor_agent_id = ? AND depth = ? AND status = ?", agentID, 1, model.RelationStatusActive).
		Count(&stats.DirectDescendants).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&model.AgentRelationClosure{}).
		Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive).
		Count(&stats.TotalDescendants).Error; err != nil {
		return stats, err
	}
	var leafRows []struct{ DescendantAgentID uint64 }
	if err := r.db.Model(&model.AgentRelationClosure{}).
		Select("descendant_agent_id").
		Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive).
		Group("descendant_agent_id").
		Find(&leafRows).Error; err != nil {
		return stats, err
	}
	for _, row := range leafRows {
		var childCount int64
		if err := r.db.Model(&model.AgentRelationClosure{}).
			Where("ancestor_agent_id = ? AND depth = ? AND status = ?", row.DescendantAgentID, 1, model.RelationStatusActive).
			Count(&childCount).Error; err != nil {
			return stats, err
		}
		if childCount == 0 {
			stats.LeafDescendants++
		}
	}
	return stats, nil
}

func (r *gormRepository) LoadLatestWithdrawal(agentID uint64) (model.WithdrawalRequest, error) {
	var item model.WithdrawalRequest
	return item, r.db.Where("agent_id = ?", agentID).Order("id desc").First(&item).Error
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
