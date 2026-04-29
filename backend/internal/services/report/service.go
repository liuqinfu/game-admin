package report

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type AgentPerformanceItem struct {
	AgentID            uint64
	AgentName          string
	Currency           string
	Balance            float64
	FrozenBalance      float64
	WithdrawableAmount float64
	TotalEntries       int64
	IncomeAmount       float64
	FreezeAmount       float64
	UnfreezeAmount     float64
	DebitAmount        float64
	ReverseAmount      float64
	AdjustAmount       float64
}

type GameSettlementItem struct {
	Currency              string
	TotalBills            int64
	ConfirmedBills        int64
	PendingBills          int64
	GeneratedBills        int64
	CancelledBills        int64
	TotalCommissionAmount float64
	TotalAdjustmentAmount float64
	TotalPayableAmount    float64
}

type TeamPerformanceItem struct {
	AgentID                 uint64
	AgentName               string
	Level                   uint32
	Currency                string
	DirectDescendants       int64
	TotalDescendants        int64
	ActiveDescendants       int64
	LeafDescendants         int64
	BoundPlayers            int64
	TotalTeamBalance        float64
	TotalTeamFrozenBalance  float64
	TotalWithdrawableAmount float64
	PendingWithdrawals      int64
	PendingWithdrawalAmount float64
	ConfirmedBills          int64
	ConfirmedBillAmount     float64
}

type SettlementProgressItem struct {
	Currency                string
	TotalBills              int64
	GeneratedBills          int64
	ConfirmedBills          int64
	CancelledBills          int64
	ProgressPercent         float64
	PendingCommissionAmount float64
	PendingPayableAmount    float64
	CompletedPayableAmount  float64
	LastGeneratedAt         string
	LastConfirmedAt         string
}

type DataPlatformLayerItem struct {
	Layer        string
	Status       string
	SyncMode     string
	Description  string
	TableCount   int
	RecordCount  int64
	LastSyncedAt string
}

type DataPlatformMetricItem struct {
	Key         string
	Value       float64
	Unit        string
	Description string
}

type profitabilityConfig struct {
	GrossMarginRate        float64 `json:"grossMarginRate"`
	PaymentChannelCostRate float64 `json:"paymentChannelCostRate"`
	TargetNetProfitRate    float64 `json:"targetNetProfitRate"`
}

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) AgentPerformance(scope sharedsvc.Scope, agentID string) ([]AgentPerformanceItem, error) {
	query := s.db.Model(&model.AgentAccount{}).
		Select("agent_account.agent_id, agent.name as agent_name, agent_account.currency, agent_account.balance, agent_account.frozen_balance, agent_account.withdrawable_amount").
		Joins("JOIN agent ON agent.id = agent_account.agent_id").
		Order("agent_account.id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "agent_account.tenant_id", "agent_account.brand_id", "TRIM(agent.remark)")
	if value := strings.TrimSpace(agentID); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, errors.New("invalid agentID")
		}
		query = query.Where("agent_account.agent_id = ?", parsed)
	}
	type row struct {
		AgentID            uint64
		AgentName          string
		Currency           string
		Balance            float64
		FrozenBalance      float64
		WithdrawableAmount float64
	}
	var rows []row
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AgentPerformanceItem, 0, len(rows))
	for _, row := range rows {
		item := AgentPerformanceItem{
			AgentID:            row.AgentID,
			AgentName:          row.AgentName,
			Currency:           sharedsvc.FirstNonEmpty(strings.TrimSpace(row.Currency), "CNY"),
			Balance:            row.Balance,
			FrozenBalance:      row.FrozenBalance,
			WithdrawableAmount: row.WithdrawableAmount,
		}
		var totals []struct {
			LedgerType    model.LedgerType
			ReferenceType string
			Amount        float64
			Count         int64
		}
		if err := s.db.Model(&model.AgentAccountLedger{}).
			Select("ledger_type, reference_type, sum(amount) as amount, count(*) as count").
			Where("agent_id = ?", row.AgentID).
			Group("ledger_type, reference_type").
			Find(&totals).Error; err != nil {
			return nil, err
		}
		for _, total := range totals {
			item.TotalEntries += total.Count
			switch total.LedgerType {
			case model.LedgerTypeIncome:
				item.IncomeAmount += total.Amount
			case model.LedgerTypeReverse:
				item.ReverseAmount += total.Amount
				if strings.TrimSpace(total.ReferenceType) != "withdrawal_request" {
					item.DebitAmount += total.Amount
				}
			case model.LedgerTypeFreeze:
				item.FreezeAmount += total.Amount
				if strings.TrimSpace(total.ReferenceType) == "risk_case" {
					item.DebitAmount += total.Amount
				}
			case model.LedgerTypeUnfreeze:
				item.UnfreezeAmount += total.Amount
			case model.LedgerTypeDebit:
				item.DebitAmount += total.Amount
			case model.LedgerTypeAdjust:
				item.AdjustAmount += total.Amount
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) GameSettlement(scope sharedsvc.Scope, currency string) ([]GameSettlementItem, error) {
	value, err := sharedsvc.NormalizeCurrencyFilter(currency)
	if err != nil {
		return nil, err
	}
	type row struct {
		Currency              string
		TotalBills            int64
		ConfirmedBills        int64
		PendingBills          int64
		GeneratedBills        int64
		CancelledBills        int64
		TotalCommissionAmount float64
		TotalAdjustmentAmount float64
		TotalPayableAmount    float64
	}
	query := s.db.Model(&model.SettlementBill{}).
		Select(strings.Join([]string{
			"settlement_bill.currency",
			"count(*) as total_bills",
			"sum(case when settlement_bill.status = 'confirmed' then 1 else 0 end) as confirmed_bills",
			"sum(case when settlement_bill.status = 'pending' then 1 else 0 end) as pending_bills",
			"sum(case when settlement_bill.status = 'generated' then 1 else 0 end) as generated_bills",
			"sum(case when settlement_bill.status = 'cancelled' then 1 else 0 end) as cancelled_bills",
			"sum(settlement_bill.commission_amount) as total_commission_amount",
			"sum(settlement_bill.adjustment_amount) as total_adjustment_amount",
			"sum(settlement_bill.payable_amount) as total_payable_amount",
		}, ", ")).
		Joins("JOIN agent ON agent.id = settlement_bill.agent_id").
		Group("settlement_bill.currency").
		Order("settlement_bill.currency asc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "settlement_bill.tenant_id", "settlement_bill.brand_id", "TRIM(agent.remark)")
	if value != "" {
		query = query.Where("settlement_bill.currency = ?", value)
	}
	var rows []row
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]GameSettlementItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, GameSettlementItem{
			Currency:              sharedsvc.FirstNonEmpty(strings.ToUpper(strings.TrimSpace(row.Currency)), value),
			TotalBills:            row.TotalBills,
			ConfirmedBills:        row.ConfirmedBills,
			PendingBills:          row.PendingBills,
			GeneratedBills:        row.GeneratedBills,
			CancelledBills:        row.CancelledBills,
			TotalCommissionAmount: row.TotalCommissionAmount,
			TotalAdjustmentAmount: row.TotalAdjustmentAmount,
			TotalPayableAmount:    row.TotalPayableAmount,
		})
	}
	if len(items) == 0 {
		items = []GameSettlementItem{{Currency: sharedsvc.FirstNonEmpty(value, "CNY")}}
	}
	return items, nil
}

func (s *Service) TeamPerformance(scope sharedsvc.Scope, agentID, currency string) ([]TeamPerformanceItem, error) {
	agentValue := strings.TrimSpace(agentID)
	if agentValue != "" {
		if _, err := strconv.ParseUint(agentValue, 10, 64); err != nil {
			return nil, errors.New("invalid agentID")
		}
	}
	currencyValue, err := sharedsvc.NormalizeCurrencyFilter(currency)
	if err != nil {
		return nil, err
	}
	type agentRow struct {
		ID       uint64
		Name     string
		Level    uint32
		Currency string
	}
	query := s.db.Model(&model.Agent{}).Select("id, name, level, currency").Order("id desc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "tenant_id", "brand_id", "TRIM(remark)")
	if agentValue != "" {
		query = query.Where("id = ?", agentValue)
	}
	if currencyValue != "" {
		query = query.Where("currency = ?", currencyValue)
	}
	var agents []agentRow
	if err := query.Find(&agents).Error; err != nil {
		return nil, err
	}
	items := make([]TeamPerformanceItem, 0, len(agents))
	for _, agent := range agents {
		item := TeamPerformanceItem{
			AgentID:   agent.ID,
			AgentName: agent.Name,
			Level:     agent.Level,
			Currency:  sharedsvc.FirstNonEmpty(strings.ToUpper(strings.TrimSpace(agent.Currency)), sharedsvc.FirstNonEmpty(currencyValue, "CNY")),
		}
		stats, err := s.loadAgentTeamStats(agent.ID)
		if err != nil {
			return nil, err
		}
		item.DirectDescendants = stats.DirectDescendants
		item.TotalDescendants = stats.TotalDescendants
		item.LeafDescendants = stats.LeafDescendants

		if err := s.db.Model(&model.AgentRelationClosure{}).
			Joins("JOIN agent ON agent.id = agent_relation_closure.descendant_agent_id").
			Where("agent_relation_closure.ancestor_agent_id = ? AND agent_relation_closure.depth > ? AND agent_relation_closure.status = ? AND agent.status = ?", agent.ID, 0, model.RelationStatusActive, model.AgentStatusActive).
			Count(&item.ActiveDescendants).Error; err != nil {
			return nil, err
		}

		var descendantIDs []uint64
		if err := s.db.Model(&model.AgentRelationClosure{}).
			Where("ancestor_agent_id = ? AND depth > ? AND status = ?", agent.ID, 0, model.RelationStatusActive).
			Distinct("descendant_agent_id").
			Pluck("descendant_agent_id", &descendantIDs).Error; err != nil {
			return nil, err
		}
		teamAgentIDs := append([]uint64{agent.ID}, descendantIDs...)

		if err := s.db.Model(&model.Binding{}).
			Where("agent_id IN ? AND status = ?", teamAgentIDs, model.BindingStatusBound).
			Count(&item.BoundPlayers).Error; err != nil {
			return nil, err
		}
		var accounts []model.AgentAccount
		if err := s.db.Where("agent_id IN ?", teamAgentIDs).Find(&accounts).Error; err != nil {
			return nil, err
		}
		for _, account := range accounts {
			effectiveFrozen, effectiveBalance := s.computeAgentAccountRiskMetrics(account)
			item.TotalTeamBalance += effectiveBalance
			item.TotalTeamFrozenBalance += effectiveFrozen
			item.TotalWithdrawableAmount += effectiveBalance - effectiveFrozen
		}
		type pendingWithdrawalRow struct {
			Count  int64
			Amount float64
		}
		var pendingWithdrawal pendingWithdrawalRow
		if err := s.db.Model(&model.WithdrawalRequest{}).
			Select("COUNT(*) AS count, COALESCE(SUM(amount), 0) AS amount").
			Where("agent_id IN ? AND status = ?", teamAgentIDs, model.WithdrawalStatusPending).
			Scan(&pendingWithdrawal).Error; err != nil {
			return nil, err
		}
		item.PendingWithdrawals = pendingWithdrawal.Count
		item.PendingWithdrawalAmount = pendingWithdrawal.Amount

		type confirmedBillRow struct {
			Count  int64
			Amount float64
		}
		var confirmedBills confirmedBillRow
		if err := s.db.Model(&model.SettlementBill{}).
			Select("COUNT(*) AS count, COALESCE(SUM(payable_amount), 0) AS amount").
			Where("agent_id IN ? AND status = ?", teamAgentIDs, model.SettlementBillStatusConfirmed).
			Scan(&confirmedBills).Error; err != nil {
			return nil, err
		}
		item.ConfirmedBills = confirmedBills.Count
		item.ConfirmedBillAmount = confirmedBills.Amount
		items = append(items, item)
	}
	if len(items) == 0 {
		items = []TeamPerformanceItem{}
	}
	return items, nil
}

func (s *Service) SettlementProgress(scope sharedsvc.Scope, currency string) ([]SettlementProgressItem, error) {
	value, err := sharedsvc.NormalizeCurrencyFilter(currency)
	if err != nil {
		return nil, err
	}
	type row struct {
		Currency                string
		TotalBills              int64
		GeneratedBills          int64
		ConfirmedBills          int64
		CancelledBills          int64
		PendingCommissionAmount float64
		PendingPayableAmount    float64
		CompletedPayableAmount  float64
		LastGeneratedAt         string
		LastConfirmedAt         string
	}
	query := s.db.Model(&model.SettlementBill{}).
		Select(strings.Join([]string{
			"settlement_bill.currency",
			"count(*) as total_bills",
			"sum(case when settlement_bill.status = 'generated' then 1 else 0 end) as generated_bills",
			"sum(case when settlement_bill.status = 'confirmed' then 1 else 0 end) as confirmed_bills",
			"sum(case when settlement_bill.status = 'cancelled' then 1 else 0 end) as cancelled_bills",
			"sum(case when settlement_bill.status in (?, ?) then settlement_bill.commission_amount else 0 end) as pending_commission_amount",
			"sum(case when settlement_bill.status in (?, ?) then settlement_bill.payable_amount else 0 end) as pending_payable_amount",
			"sum(case when settlement_bill.status = ? then settlement_bill.payable_amount else 0 end) as completed_payable_amount",
			"max(case when settlement_bill.status in (?, ?) then settlement_bill.generated_at end) as last_generated_at",
			"max(case when settlement_bill.status = ? then settlement_bill.confirmed_at end) as last_confirmed_at",
		}, ", "),
			model.SettlementBillStatusPending,
			model.SettlementBillStatusGenerated,
			model.SettlementBillStatusPending,
			model.SettlementBillStatusGenerated,
			model.SettlementBillStatusConfirmed,
			model.SettlementBillStatusPending,
			model.SettlementBillStatusGenerated,
			model.SettlementBillStatusConfirmed,
		).
		Joins("JOIN agent ON agent.id = settlement_bill.agent_id").
		Group("settlement_bill.currency").
		Order("settlement_bill.currency asc")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "settlement_bill.tenant_id", "settlement_bill.brand_id", "TRIM(agent.remark)")
	if value != "" {
		query = query.Where("settlement_bill.currency = ?", value)
	}
	var rows []row
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]SettlementProgressItem, 0, len(rows))
	for _, row := range rows {
		progressPercent := 0.0
		if row.TotalBills > 0 {
			progressPercent = sharedsvc.Round2(float64(row.ConfirmedBills) * 100 / float64(row.TotalBills))
		}
		item := SettlementProgressItem{
			Currency:                sharedsvc.FirstNonEmpty(strings.TrimSpace(row.Currency), value),
			TotalBills:              row.TotalBills,
			GeneratedBills:          row.GeneratedBills,
			ConfirmedBills:          row.ConfirmedBills,
			CancelledBills:          row.CancelledBills,
			ProgressPercent:         progressPercent,
			PendingCommissionAmount: row.PendingCommissionAmount,
			PendingPayableAmount:    row.PendingPayableAmount,
			CompletedPayableAmount:  row.CompletedPayableAmount,
		}
		if parsed := parseSQLiteAggregateTime(row.LastGeneratedAt); parsed != nil {
			item.LastGeneratedAt = parsed.UTC().Format(time.RFC3339)
		}
		if parsed := parseSQLiteAggregateTime(row.LastConfirmedAt); parsed != nil {
			item.LastConfirmedAt = parsed.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		items = []SettlementProgressItem{{Currency: sharedsvc.FirstNonEmpty(value, "CNY")}}
	}
	return items, nil
}

func (s *Service) DataPlatformLayers(scope sharedsvc.Scope, tenantIDText, brandIDText string) ([]DataPlatformLayerItem, error) {
	now := time.Now().UTC()
	type scopedTable struct {
		model       any
		tenantField string
		brandField  string
	}
	type layerDefinition struct {
		layer       string
		description string
		tables      []scopedTable
	}
	layers := []layerDefinition{
		{
			layer:       "ODS",
			description: "Raw business ingestion for recharge callbacks and source orders.",
			tables: []scopedTable{
				{model: &model.RechargeOrder{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.RechargeCallbackLog{}, tenantField: "", brandField: ""},
			},
		},
		{
			layer:       "DWD",
			description: "Detailed business facts for binding, commission, ledger, withdrawal, and activity reward.",
			tables: []scopedTable{
				{model: &model.Binding{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.CommissionRecord{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.AgentAccountLedger{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.WithdrawalRequest{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.ActivityRewardRecord{}, tenantField: "tenant_id", brandField: "brand_id"},
			},
		},
		{
			layer:       "DWS",
			description: "Subject-area aggregates grouped by agent, team, settlement, and financial progress.",
			tables: []scopedTable{
				{model: &model.AgentAccount{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.SettlementBill{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.AgentRelationClosure{}, tenantField: "tenant_id", brandField: "brand_id"},
			},
		},
		{
			layer:       "ADS",
			description: "Operational metrics for growth, reward ROI, withdrawals, and risk interception.",
			tables: []scopedTable{
				{model: &model.Player{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.RechargeOrder{}, tenantField: "tenant_id", brandField: "brand_id"},
				{model: &model.RiskCase{}, tenantField: "tenant_id", brandField: "brand_id"},
			},
		},
	}

	items := make([]DataPlatformLayerItem, 0, len(layers))
	for _, layer := range layers {
		item := DataPlatformLayerItem{
			Layer:       layer.layer,
			Status:      "ready",
			SyncMode:    "on_demand",
			Description: layer.description,
			TableCount:  len(layer.tables),
		}
		var lastSyncedAt *time.Time
		for _, table := range layer.tables {
			countQuery := s.db.Model(table.model)
			if table.tenantField != "" || table.brandField != "" {
				countQuery = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(countQuery, scope, table.tenantField, table.brandField, "")
			}
			var err error
			countQuery, err = sharedsvc.ApplyScopedQueryFilters(countQuery, table.tenantField, table.brandField, tenantIDText, brandIDText)
			if err != nil {
				return nil, err
			}
			var count int64
			if err := countQuery.Count(&count).Error; err != nil {
				return nil, err
			}
			item.RecordCount += count

			latestQuery := s.db.Model(table.model)
			if table.tenantField != "" || table.brandField != "" {
				latestQuery = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(latestQuery, scope, table.tenantField, table.brandField, "")
			}
			latestQuery, err = sharedsvc.ApplyScopedQueryFilters(latestQuery, table.tenantField, table.brandField, tenantIDText, brandIDText)
			if err != nil {
				return nil, err
			}
			type latestRow struct{ Value string }
			var row latestRow
			if err := latestQuery.Select("MAX(updated_at) AS value").Scan(&row).Error; err != nil {
				return nil, err
			}
			latest := parseSQLiteAggregateTime(row.Value)
			if latest != nil && (lastSyncedAt == nil || latest.After(*lastSyncedAt)) {
				lastSyncedAt = latest
			}
		}
		if lastSyncedAt != nil {
			item.LastSyncedAt = lastSyncedAt.UTC().Format(time.RFC3339)
		} else {
			item.LastSyncedAt = now.Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) DataPlatformMetrics(scope sharedsvc.Scope, tenantIDText, brandIDText string) ([]DataPlatformMetricItem, error) {
	now := time.Now().UTC()
	last7 := now.Add(-7 * 24 * time.Hour)
	prev7 := now.Add(-14 * 24 * time.Hour)

	paidOrders7d, err := s.repo.CountBetween(scope, &model.RechargeOrder{}, "tenant_id", "brand_id", "created_at", tenantIDText, brandIDText, last7, now)
	if err != nil {
		return nil, err
	}
	newPlayers7d, err := s.repo.CountBetween(scope, &model.Player{}, "tenant_id", "brand_id", "registered_at", tenantIDText, brandIDText, last7, now)
	if err != nil {
		return nil, err
	}
	prevNewPlayers7d, err := s.repo.CountBetween(scope, &model.Player{}, "tenant_id", "brand_id", "registered_at", tenantIDText, brandIDText, prev7, last7)
	if err != nil {
		return nil, err
	}
	rechargeAmount7d, err := s.repo.SumBetween(scope, &model.RechargeOrder{}, "tenant_id", "brand_id", "created_at", "paid_amount", tenantIDText, brandIDText, last7, now, "status = ?", model.OrderStatusPaid)
	if err != nil {
		return nil, err
	}
	commissionCost7d, err := s.repo.SumBetween(scope, &model.CommissionRecord{}, "tenant_id", "brand_id", "estimated_at", "commission_amount", tenantIDText, brandIDText, last7, now, "commission_amount > ?", 0)
	if err != nil {
		return nil, err
	}
	rewardCost7d, err := s.repo.SumBetween(scope, &model.ActivityRewardRecord{}, "tenant_id", "brand_id", "created_at", "reward_value", tenantIDText, brandIDText, last7, now, "status = ?", model.ActivityRewardRecordStatusGranted)
	if err != nil {
		return nil, err
	}
	actualPaymentChannelCost7d, err := s.repo.SumBetween(scope, &model.RechargeOrder{}, "tenant_id", "brand_id", "created_at", "payment_channel_cost", tenantIDText, brandIDText, last7, now, "status = ?", model.OrderStatusPaid)
	if err != nil {
		return nil, err
	}
	actualPaymentChannelCostCount7d, err := s.repo.CountNonNullBetween(scope, &model.RechargeOrder{}, "tenant_id", "brand_id", "created_at", "payment_channel_cost", tenantIDText, brandIDText, last7, now, "status = ?", model.OrderStatusPaid)
	if err != nil {
		return nil, err
	}
	actualGrossProfit7d, err := s.repo.SumBetween(scope, &model.RechargeOrder{}, "tenant_id", "brand_id", "created_at", "gross_profit_amount", tenantIDText, brandIDText, last7, now, "status = ?", model.OrderStatusPaid)
	if err != nil {
		return nil, err
	}
	actualGrossProfitCount7d, err := s.repo.CountNonNullBetween(scope, &model.RechargeOrder{}, "tenant_id", "brand_id", "created_at", "gross_profit_amount", tenantIDText, brandIDText, last7, now, "status = ?", model.OrderStatusPaid)
	if err != nil {
		return nil, err
	}
	firstRechargePlayers7d, err := s.repo.CountFirstRechargePlayersScoped(scope, last7, now)
	if err != nil {
		return nil, err
	}
	withdrawalSuccessRate, err := s.repo.CalculateWithdrawalSuccessRateScoped(scope)
	if err != nil {
		return nil, err
	}
	riskInterceptRate, err := s.repo.CalculateRiskInterceptRateScoped(scope)
	if err != nil {
		return nil, err
	}
	profitConfig, hasProfitConfig, err := s.loadScopedProfitabilityConfig(scope, tenantIDText, brandIDText)
	if err != nil {
		return nil, err
	}
	withdrawalCost7d, err := s.repo.SumBetween(scope, &model.WithdrawalRequest{}, "tenant_id", "brand_id", "updated_at", "(fee_amount + tax_amount)", tenantIDText, brandIDText, last7, now, "status IN ?", []model.WithdrawalStatus{model.WithdrawalStatusPaid, model.WithdrawalStatusReturned, model.WithdrawalStatusClosed})
	if err != nil {
		return nil, err
	}

	profitDataCoverage := 0.0
	if paidOrders7d > 0 {
		coverageCount := minInt64(actualPaymentChannelCostCount7d, actualGrossProfitCount7d)
		profitDataCoverage = sharedsvc.Round2(float64(coverageCount) * 100 / float64(paidOrders7d))
	}
	teamGrowthRate := 0.0
	if prevNewPlayers7d > 0 {
		teamGrowthRate = sharedsvc.Round2((float64(newPlayers7d-prevNewPlayers7d) / float64(prevNewPlayers7d)) * 100)
	} else if newPlayers7d > 0 {
		teamGrowthRate = 100
	}
	activityROI := 0.0
	if rewardCost7d > 0 {
		activityROI = sharedsvc.Round2(((rechargeAmount7d - rewardCost7d) / rewardCost7d) * 100)
	}

	items := []DataPlatformMetricItem{
		{Key: "newPlayers", Value: float64(newPlayers7d), Unit: "count", Description: "New players registered within the last 7 days."},
		{Key: "firstRechargePlayers", Value: float64(firstRechargePlayers7d), Unit: "count", Description: "Players whose first successful recharge happened within the last 7 days."},
		{Key: "rechargeAmount", Value: sharedsvc.Round2(rechargeAmount7d), Unit: "amount", Description: "Total paid recharge amount within the last 7 days."},
		{Key: "commissionCost", Value: sharedsvc.Round2(commissionCost7d), Unit: "amount", Description: "Commission cost generated within the last 7 days."},
		{Key: "teamGrowthRate", Value: teamGrowthRate, Unit: "percent", Description: "New-player growth compared with the previous 7-day window."},
		{Key: "activityROI", Value: activityROI, Unit: "percent", Description: "Recharge return relative to granted activity reward cost."},
		{Key: "withdrawalSuccessRate", Value: withdrawalSuccessRate, Unit: "percent", Description: "Share of paid withdrawals among completed payout outcomes."},
		{Key: "riskInterceptRate", Value: riskInterceptRate, Unit: "percent", Description: "Share of freeze-requested risk cases among all risk cases."},
		{Key: "withdrawalCost", Value: sharedsvc.Round2(withdrawalCost7d), Unit: "amount", Description: "Withdrawal fee and tax cost recorded within the last 7 days."},
		{Key: "profitDataCoverage", Value: profitDataCoverage, Unit: "percent", Description: "Share of paid recharge orders that already contain both actual payment cost and gross profit facts."},
	}
	grossProfitValue, grossProfitDescription, hasGrossProfitValue := 0.0, "", false
	if paidOrders7d > 0 && actualGrossProfitCount7d == paidOrders7d {
		grossProfitValue = sharedsvc.Round2(actualGrossProfit7d)
		grossProfitDescription = "Actual gross profit aggregated from recharge orders within the last 7 days."
		hasGrossProfitValue = true
	} else if hasProfitConfig {
		grossProfitValue = sharedsvc.Round2(rechargeAmount7d * profitConfig.GrossMarginRate / 100)
		grossProfitDescription = "Estimated gross profit based on configured gross margin rate."
		hasGrossProfitValue = true
	}
	paymentChannelCostValue, paymentChannelCostDescription, hasPaymentChannelCostValue := 0.0, "", false
	if paidOrders7d > 0 && actualPaymentChannelCostCount7d == paidOrders7d {
		paymentChannelCostValue = sharedsvc.Round2(actualPaymentChannelCost7d)
		paymentChannelCostDescription = "Actual payment channel cost aggregated from recharge orders within the last 7 days."
		hasPaymentChannelCostValue = true
	} else if hasProfitConfig {
		paymentChannelCostValue = sharedsvc.Round2(rechargeAmount7d * profitConfig.PaymentChannelCostRate / 100)
		paymentChannelCostDescription = "Estimated payment channel cost based on configured payment cost rate."
		hasPaymentChannelCostValue = true
	}
	if hasGrossProfitValue {
		items = append(items, DataPlatformMetricItem{Key: "estimatedGrossProfit", Value: grossProfitValue, Unit: "amount", Description: grossProfitDescription})
	}
	if hasPaymentChannelCostValue {
		items = append(items, DataPlatformMetricItem{Key: "paymentChannelCost", Value: paymentChannelCostValue, Unit: "amount", Description: paymentChannelCostDescription})
	}
	if hasGrossProfitValue && hasPaymentChannelCostValue {
		estimatedNetProfit := sharedsvc.Round2(grossProfitValue - paymentChannelCostValue - commissionCost7d - rewardCost7d - withdrawalCost7d)
		estimatedNetMargin := 0.0
		if rechargeAmount7d > 0 {
			estimatedNetMargin = sharedsvc.Round2(estimatedNetProfit * 100 / rechargeAmount7d)
		}
		items = append(items,
			DataPlatformMetricItem{Key: "estimatedNetProfit", Value: estimatedNetProfit, Unit: "amount", Description: "Net profit after payment cost, commission, reward, and withdrawal costs. Uses actual recharge facts when fully available, otherwise falls back to configured estimates."},
			DataPlatformMetricItem{Key: "estimatedNetMargin", Value: estimatedNetMargin, Unit: "percent", Description: "Net profit divided by recharge amount. Uses actual recharge facts when fully available, otherwise falls back to configured estimates."},
		)
		if hasProfitConfig && profitConfig.TargetNetProfitRate > 0 {
			items = append(items, DataPlatformMetricItem{Key: "targetNetProfitRate", Value: sharedsvc.Round2(profitConfig.TargetNetProfitRate), Unit: "percent", Description: "Configured net profit rate target for the current scope."})
		}
	}
	return items, nil
}

type teamStats struct {
	DirectDescendants int64
	TotalDescendants  int64
	LeafDescendants   int64
}

func (s *Service) loadAgentTeamStats(agentID uint64) (teamStats, error) {
	return s.repo.LoadAgentTeamStats(agentID)
}

func (s *Service) computeAgentAccountRiskMetrics(account model.AgentAccount) (float64, float64) {
	effectiveFrozen := account.FrozenBalance
	effectiveBalance := account.Balance
	if effectiveFrozen > 0 {
		return effectiveFrozen, effectiveBalance
	}
	latestWithdrawal, err := s.repo.LoadLatestWithdrawal(account.AgentID)
	if err != nil {
		return effectiveFrozen, effectiveBalance
	}
	if latestWithdrawal.Amount <= 0 {
		return effectiveFrozen, effectiveBalance
	}
	incomeAmount, manualReverseAmount, err := s.repo.LoadLedgerRiskMetrics(account.AgentID)
	if err == nil {
		effectiveFrozen = latestWithdrawal.Amount
		if incomeAmount > 0 {
			effectiveBalance = incomeAmount - manualReverseAmount
		}
	}
	return effectiveFrozen, effectiveBalance
}

func parseSQLiteAggregateTime(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return &parsed
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func uniqueUint64(values []uint64) []uint64 {
	seen := map[uint64]struct{}{}
	items := make([]uint64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func (s *Service) loadScopedProfitabilityConfig(scope sharedsvc.Scope, tenantIDText, brandIDText string) (profitabilityConfig, bool, error) {
	var config profitabilityConfig
	tenantID, err := sharedsvc.ParseScopeUint64(tenantIDText)
	if err != nil {
		return config, false, err
	}
	brandID, err := sharedsvc.ParseScopeUint64(brandIDText)
	if err != nil {
		return config, false, err
	}

	query := sharedsvc.ApplyTenantBrandScopeWithLegacyCode(s.db.Model(&model.PlatformConfig{}), scope, "platform_config.tenant_id", "platform_config.brand_id", "").
		Where("platform_config.key = ?", "operations.profit_model")
	if tenantID != nil {
		query = query.Where("(platform_config.tenant_id = ? OR platform_config.tenant_id IS NULL)", *tenantID)
	}
	if brandID != nil {
		query = query.Where("(platform_config.brand_id = ? OR platform_config.brand_id IS NULL)", *brandID)
	}

	var configs []model.PlatformConfig
	if err := query.Order("platform_config.id desc").Find(&configs).Error; err != nil {
		return config, false, err
	}
	if len(configs) == 0 {
		return config, false, nil
	}

	scoreConfig := func(item model.PlatformConfig) int {
		score := 0
		if brandID != nil && item.BrandID != nil && *item.BrandID == *brandID {
			score += 4
		}
		if tenantID != nil && item.TenantID != nil && *item.TenantID == *tenantID {
			score += 2
		}
		if item.BrandID != nil {
			score++
		}
		if item.TenantID != nil {
			score++
		}
		return score
	}

	best := configs[0]
	bestScore := scoreConfig(best)
	for _, item := range configs[1:] {
		if currentScore := scoreConfig(item); currentScore > bestScore {
			best = item
			bestScore = currentScore
		}
	}
	if err := json.Unmarshal(best.Value, &config); err != nil {
		return profitabilityConfig{}, false, err
	}
	return config, true, nil
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
