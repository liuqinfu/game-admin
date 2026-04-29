package risk

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	accountcontract "game-admin/backend/internal/contract/account"
	reportcontract "game-admin/backend/internal/contract/report"
	withdrawalcontract "game-admin/backend/internal/contract/withdrawal"
	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db               *gorm.DB
	repo             Repository
	accountClient    accountcontract.Client
	reportClient     reportcontract.Client
	withdrawalClient withdrawalcontract.Client
}

type AgentAccountFilter struct {
	MinFrozenAmount string
	RiskLevel       string
	AgentID         string
}

type RiskCaseFilter struct {
	MinFrozenRatio string
	RiskLevel      string
	Status         string
	AgentID        string
}

type CreateCaseInput struct {
	CaseNo            string
	TenantID          *uint64
	BrandID           *uint64
	AgentID           uint64
	Amount            float64
	Currency          string
	Reason            string
	Freeze            bool
	FreezeIdempotency string
	Remark            string
}

type ReviewCaseInput struct {
	Action string
	Remark string
}

type AgentAccountRiskItem struct {
	model.AgentAccount
	AgentName   string
	RiskLevel   string
	FrozenRatio float64
}

type RiskCaseItem struct {
	CaseNo                    string
	Status                    string
	AgentID                   uint64
	AgentName                 string
	Currency                  string
	RiskLevel                 string
	Reason                    string
	FreezeRequested           bool
	FrozenBalance             float64
	WithdrawableAmount        float64
	FrozenRatio               float64
	LatestWithdrawalRequestID *uint64
	LatestWithdrawalRequestNo string
	LatestWithdrawalAmount    *float64
	RiskNote                  string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type IntelligenceItem struct {
	SourceType        string
	SourceID          string
	AgentID           uint64
	AgentName         string
	RiskLevel         string
	Score             float64
	Reason            string
	RecommendedAction string
	Intercepted       bool
	Status            string
	CreatedAt         string
}

func NewService(db *gorm.DB) *Service {
	return NewServiceWithClients(
		db,
		reportcontract.NewLocalDBClient(db),
		accountcontract.NewLocalDBClient(db),
		withdrawalcontract.NewLocalDBClient(db),
	)
}

func NewServiceWithReportClient(db *gorm.DB, reportClient reportcontract.Client) *Service {
	return NewServiceWithClients(db, reportClient, accountcontract.NewLocalDBClient(db), withdrawalcontract.NewLocalDBClient(db))
}

func NewServiceWithClients(db *gorm.DB, reportClient reportcontract.Client, accountClient accountcontract.Client, withdrawalClient withdrawalcontract.Client) *Service {
	return &Service{
		db:               db,
		repo:             newRepository(db),
		accountClient:    accountClient,
		reportClient:     reportClient,
		withdrawalClient: withdrawalClient,
	}
}

func (s *Service) ReleasePendingCases(ctx context.Context, scope sharedsvc.Scope, agentID uint64, remark, reviewer string) ([]string, error) {
	if agentID == 0 {
		return nil, errors.New("agentID is required")
	}
	rows, err := s.repo.ListRiskCases(scope, RiskCaseFilter{
		Status:  string(model.RiskCaseStatusPending),
		AgentID: strconv.FormatUint(agentID, 10),
	})
	if err != nil {
		return nil, err
	}
	released := make([]string, 0, len(rows))
	for _, row := range rows {
		updated, _, err := s.ReviewCase(ctx, scope, row.CaseNo, ReviewCaseInput{
			Action: "release",
			Remark: remark,
		}, reviewer)
		if err != nil {
			if err.Error() == "risk case already reviewed" {
				continue
			}
			return released, err
		}
		if updated.Status == model.RiskCaseStatusReleased {
			released = append(released, updated.CaseNo)
		}
	}
	return released, nil
}

func (s *Service) RestorePendingCases(ctx context.Context, scope sharedsvc.Scope, caseNos []string, remark, reviewer string) (int, error) {
	restored := 0
	for _, caseNo := range uniqueStrings(caseNos) {
		query := s.db.Model(&model.RiskCase{}).Joins("JOIN agent ON agent.id = risk_case.agent_id")
		query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "risk_case.tenant_id", "risk_case.brand_id", "TRIM(agent.remark)")
		var item model.RiskCase
		if err := query.Where("risk_case.case_no = ?", caseNo).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return restored, err
		}
		if item.Status == model.RiskCaseStatusPending {
			restored++
			continue
		}
		if item.Status != model.RiskCaseStatusReleased {
			continue
		}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			ledger, err := s.accountClient.CreateLedger(accountcontract.WithDBTx(ctx, tx), scope, accountcontract.LedgerCreateInput{
				AgentID:        item.AgentID,
				ReferenceType:  "risk_case",
				ReferenceID:    item.CaseNo,
				LedgerType:     model.LedgerTypeFreeze,
				Amount:         sharedsvc.Round2(item.Amount),
				Currency:       sharedsvc.FirstNonEmpty(strings.TrimSpace(item.Currency), "CNY"),
				IdempotencyKey: "risk-case-restore:" + item.CaseNo,
				Remark:         sharedsvc.FirstNonEmpty(strings.TrimSpace(remark), "restore released risk case"),
			})
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			item.Status = model.RiskCaseStatusPending
			item.ReviewedAt = &now
			item.ReviewedBy = sharedsvc.FirstNonEmpty(strings.TrimSpace(reviewer), item.ReviewedBy, "system")
			item.ReviewRemark = strings.TrimSpace(remark)
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
			_, err = eventbus.Publish(tx, eventbus.PublishInput{
				EventType:      eventbus.EventRiskCaseRestored,
				AggregateType:  "risk_case",
				AggregateID:    strconv.FormatUint(item.ID, 10),
				TenantID:       item.TenantID,
				BrandID:        item.BrandID,
				OccurredAt:     now,
				Producer:       "risk-service",
				IdempotencyKey: "risk.restore:" + item.CaseNo + ":" + string(item.Status),
				Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
				Payload:        riskCaseReviewedPayload(item, &ledger.ID),
			})
			return err
		}); err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}

func (s *Service) AgentAccounts(ctx context.Context, scope sharedsvc.Scope, filter AgentAccountFilter) ([]AgentAccountRiskItem, error) {
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		agentID, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, errors.New("invalid agentID")
		}
		_ = agentID
	}
	rows, err := s.accountClient.ListRiskAccounts(ctx, scope, filter.AgentID)
	if err != nil {
		return nil, err
	}
	filterRiskLevel := strings.TrimSpace(strings.ToLower(filter.RiskLevel))
	filterMinFrozen := 0.0
	if value := strings.TrimSpace(filter.MinFrozenAmount); value != "" {
		amount, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("invalid minFrozenAmount")
		}
		filterMinFrozen = amount
	}
	items := make([]AgentAccountRiskItem, 0, len(rows))
	for _, row := range rows {
		agentID := row.AgentID
		if row.Account != nil && row.Account.AgentID != 0 {
			agentID = row.Account.AgentID
		}
		effectiveFrozen, effectiveBalance, err := s.computeAgentAccountRiskMetrics(ctx, agentID, row.Account, row.IncomeAmount, row.ManualReverseAmount)
		if err != nil {
			return nil, err
		}
		if effectiveFrozen < filterMinFrozen {
			continue
		}
		ratio := 0.0
		if effectiveBalance > 0 {
			ratio = effectiveFrozen / effectiveBalance
		}
		riskLevel := sharedsvc.ClassifyFrozenRisk(ratio)
		if filterRiskLevel != "" && riskLevel != filterRiskLevel {
			continue
		}
		items = append(items, AgentAccountRiskItem{
			AgentAccount: derefRiskAccount(row.Account),
			AgentName:    row.AgentName,
			RiskLevel:    riskLevel,
			FrozenRatio:  ratio,
		})
	}
	return items, nil
}

func (s *Service) Cases(ctx context.Context, scope sharedsvc.Scope, filter RiskCaseFilter) ([]RiskCaseItem, error) {
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		agentID, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, errors.New("invalid agentID")
		}
		_ = agentID
	}
	filterRiskLevel := strings.TrimSpace(strings.ToLower(filter.RiskLevel))
	filterMinFrozenRatio := 0.0
	if value := strings.TrimSpace(filter.MinFrozenRatio); value != "" {
		ratio, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("invalid minFrozenRatio")
		}
		filterMinFrozenRatio = ratio
	}
	rows, err := s.repo.ListRiskCases(scope, filter)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.loadRiskAccountSnapshots(ctx, scope, rows)
	if err != nil {
		return nil, err
	}
	items := make([]RiskCaseItem, 0, len(rows))
	for _, row := range rows {
		frozenBalance, withdrawableAmount, err := s.loadRiskCaseAccountMetrics(ctx, row.RiskCase, snapshots[row.AgentID])
		if err != nil {
			return nil, err
		}
		totalBalance := frozenBalance + withdrawableAmount
		frozenRatio := 0.0
		if totalBalance > 0 {
			frozenRatio = sharedsvc.Round2(frozenBalance / totalBalance)
		}
		riskLevel := sharedsvc.ClassifyFrozenRisk(frozenRatio)
		if filterRiskLevel != "" && riskLevel != filterRiskLevel {
			continue
		}
		if frozenRatio < filterMinFrozenRatio {
			continue
		}
		latestWithdrawal, err := s.loadLatestWithdrawalForRisk(ctx, row.RiskCase)
		if err != nil {
			return nil, err
		}
		var latestWithdrawalID *uint64
		var latestWithdrawalAmount *float64
		latestWithdrawalNo := ""
		if latestWithdrawal != nil {
			latestWithdrawalID = uint64Ptr(latestWithdrawal.ID)
			amount := sharedsvc.Round2(latestWithdrawal.Amount)
			latestWithdrawalAmount = &amount
			latestWithdrawalNo = latestWithdrawal.RequestNo
		}
		items = append(items, RiskCaseItem{
			CaseNo:                    row.CaseNo,
			Status:                    string(row.Status),
			AgentID:                   row.AgentID,
			AgentName:                 row.AgentName,
			Currency:                  sharedsvc.FirstNonEmpty(strings.TrimSpace(row.Currency), "CNY"),
			RiskLevel:                 riskLevel,
			Reason:                    strings.TrimSpace(row.Reason),
			FreezeRequested:           row.FreezeRequested,
			FrozenBalance:             frozenBalance,
			WithdrawableAmount:        withdrawableAmount,
			FrozenRatio:               frozenRatio,
			LatestWithdrawalRequestID: latestWithdrawalID,
			LatestWithdrawalRequestNo: latestWithdrawalNo,
			LatestWithdrawalAmount:    latestWithdrawalAmount,
			RiskNote:                  sharedsvc.FirstNonEmpty(strings.TrimSpace(row.ReviewRemark), strings.TrimSpace(row.Remark)),
			CreatedAt:                 row.CreatedAt,
			UpdatedAt:                 row.UpdatedAt,
		})
	}
	return items, nil
}

func (s *Service) Intelligence(ctx context.Context, scope sharedsvc.Scope) ([]IntelligenceItem, error) {
	now := time.Now().UTC()
	items := make([]IntelligenceItem, 0)
	riskCases, err := s.Cases(ctx, scope, RiskCaseFilter{})
	if err != nil {
		return nil, err
	}
	for _, riskCase := range riskCases {
		score := riskBaseScore(riskCase.RiskLevel)
		if riskCase.FreezeRequested {
			score += 6
		}
		if riskCase.LatestWithdrawalAmount != nil && *riskCase.LatestWithdrawalAmount > 0 {
			score += 4
		}
		items = append(items, IntelligenceItem{
			SourceType:        "risk_case",
			SourceID:          riskCase.CaseNo,
			AgentID:           riskCase.AgentID,
			AgentName:         riskCase.AgentName,
			RiskLevel:         riskCase.RiskLevel,
			Score:             minFloat(sharedsvc.Round2(score), 99),
			Reason:            sharedsvc.FirstNonEmpty(strings.TrimSpace(riskCase.Reason), "High frozen ratio with pending manual review."),
			RecommendedAction: "confirm_or_release",
			Intercepted:       riskCase.FreezeRequested,
			Status:            riskCase.Status,
			CreatedAt:         riskCase.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	withdrawals, err := s.withdrawalClient.ListRiskSignals(ctx, scope, 20)
	if err != nil {
		return nil, err
	}
	for _, withdrawal := range withdrawals {
		score := 0.0
		riskLevel := "medium"
		recommendedAction := "review_withdrawal"
		intercepted := false
		switch withdrawal.Status {
		case model.WithdrawalStatusFailed, model.WithdrawalStatusReturned:
			score = 88
			riskLevel = "high"
			recommendedAction = "inspect_payout_trace"
			intercepted = true
		case model.WithdrawalStatusApproved:
			score = 74
		default:
			score = 68
		}
		if withdrawal.Amount >= 100 {
			score += 6
			riskLevel = "high"
		}
		items = append(items, IntelligenceItem{
			SourceType:        "withdrawal_request",
			SourceID:          withdrawal.RequestNo,
			AgentID:           withdrawal.AgentID,
			AgentName:         withdrawal.AgentName,
			RiskLevel:         riskLevel,
			Score:             minFloat(sharedsvc.Round2(score), 99),
			Reason:            fmt.Sprintf("Withdrawal %s requires additional review under stage-three payout controls.", withdrawal.Status),
			RecommendedAction: recommendedAction,
			Intercepted:       intercepted,
			Status:            string(withdrawal.Status),
			CreatedAt:         sharedsvc.FirstNonEmpty(formatOptionalTime(withdrawal.CreatedAt), now.Format(time.RFC3339)),
		})
	}
	teamSignals, err := s.reportClient.TeamPerformance(ctx, scope, "", "")
	if err != nil {
		return nil, err
	}
	for _, team := range teamSignals {
		if team.TotalDescendants < 3 {
			continue
		}
		activeRatio := 0.0
		if team.TotalDescendants > 0 {
			activeRatio = float64(team.ActiveDescendants) / float64(team.TotalDescendants)
		}
		if activeRatio >= 0.4 {
			continue
		}
		score := sharedsvc.Round2(70 + float64(team.TotalDescendants-team.ActiveDescendants))
		riskLevel := "medium"
		if activeRatio < 0.25 || team.TotalDescendants >= 6 {
			riskLevel = "high"
			score += 8
		}
		items = append(items, IntelligenceItem{
			SourceType:        "agent_network",
			SourceID:          strconv.FormatUint(team.AgentID, 10),
			AgentID:           team.AgentID,
			AgentName:         team.AgentName,
			RiskLevel:         riskLevel,
			Score:             minFloat(score, 99),
			Reason:            fmt.Sprintf("Only %d of %d descendants are active, indicating possible abnormal growth quality.", team.ActiveDescendants, team.TotalDescendants),
			RecommendedAction: "inspect_network_growth",
			Intercepted:       false,
			Status:            "observed",
			CreatedAt:         now.Format(time.RFC3339),
		})
	}
	slices.SortFunc(items, func(left, right IntelligenceItem) int {
		if left.Score == right.Score {
			return strings.Compare(left.SourceID, right.SourceID)
		}
		if left.Score > right.Score {
			return -1
		}
		return 1
	})
	if len(items) > 20 {
		items = items[:20]
	}
	return items, nil
}

func (s *Service) CreateCase(ctx context.Context, scope sharedsvc.Scope, input CreateCaseInput, reviewer string) (model.RiskCase, *model.AgentAccountLedger, error) {
	if err := s.ensureScopeReferences(scope, input.TenantID, input.BrandID); err != nil {
		return model.RiskCase{}, nil, err
	}
	caseNo := strings.TrimSpace(input.CaseNo)
	if caseNo == "" {
		return model.RiskCase{}, nil, errors.New("caseNo is required")
	}
	if input.AgentID == 0 {
		return model.RiskCase{}, nil, errors.New("agentID is required")
	}
	if err := s.ensureAgentExists(input.AgentID); err != nil {
		return model.RiskCase{}, nil, err
	}
	if input.Freeze && input.Amount <= 0 {
		return model.RiskCase{}, nil, errors.New("amount must be greater than 0")
	}

	var created model.RiskCase
	var ledger *model.AgentAccountLedger
	err := s.db.Transaction(func(tx *gorm.DB) error {
		current := model.RiskCase{
			CaseNo:          caseNo,
			TenantID:        input.TenantID,
			BrandID:         input.BrandID,
			AgentID:         input.AgentID,
			Amount:          sharedsvc.Round2(input.Amount),
			Currency:        sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), "CNY"),
			Reason:          strings.TrimSpace(input.Reason),
			Status:          model.RiskCaseStatusPending,
			FreezeRequested: input.Freeze,
			Remark:          strings.TrimSpace(input.Remark),
			ReviewedBy:      sharedsvc.FirstNonEmpty(strings.TrimSpace(reviewer), "finance"),
			ReviewRemark:    strings.TrimSpace(input.Remark),
		}
		if err := tx.Create(&current).Error; err != nil {
			return err
		}
		created = current
		if !input.Freeze {
			_, err := eventbus.Publish(tx, eventbus.PublishInput{
				EventType:      eventbus.EventRiskCaseCreated,
				AggregateType:  "risk_case",
				AggregateID:    strconv.FormatUint(created.ID, 10),
				TenantID:       created.TenantID,
				BrandID:        created.BrandID,
				OccurredAt:     created.CreatedAt,
				Producer:       "risk-service",
				IdempotencyKey: "risk.created:" + created.CaseNo,
				Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
				Payload:        riskCaseCreatedPayload(created, nil),
			})
			return err
		}
		idempotencyKey := strings.TrimSpace(input.FreezeIdempotency)
		if idempotencyKey == "" {
			idempotencyKey = "risk-case-freeze:" + caseNo
		}
		createdLedger, err := s.accountClient.CreateLedger(accountcontract.WithDBTx(ctx, tx), scope, accountcontract.LedgerCreateInput{
			AgentID:        input.AgentID,
			ReferenceType:  "risk_case",
			ReferenceID:    caseNo,
			LedgerType:     model.LedgerTypeFreeze,
			Amount:         sharedsvc.Round2(input.Amount),
			Currency:       sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), "CNY"),
			IdempotencyKey: idempotencyKey,
			Remark:         sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Remark), strings.TrimSpace(input.Reason)),
		})
		if err != nil {
			return err
		}
		ledger = &createdLedger
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRiskCaseCreated,
			AggregateType:  "risk_case",
			AggregateID:    strconv.FormatUint(created.ID, 10),
			TenantID:       created.TenantID,
			BrandID:        created.BrandID,
			OccurredAt:     created.CreatedAt,
			Producer:       "risk-service",
			IdempotencyKey: "risk.created:" + created.CaseNo,
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        riskCaseCreatedPayload(created, &createdLedger.ID),
		})
		return err
	})
	return created, ledger, err
}

func (s *Service) ReviewCase(ctx context.Context, scope sharedsvc.Scope, caseNo string, input ReviewCaseInput, reviewer string) (model.RiskCase, *model.AgentAccountLedger, error) {
	caseNo = strings.TrimSpace(caseNo)
	if caseNo == "" {
		return model.RiskCase{}, nil, errors.New("caseNo is required")
	}
	query := s.db.Model(&model.RiskCase{}).Joins("JOIN agent ON agent.id = risk_case.agent_id")
	query = sharedsvc.ApplyTenantBrandScopeWithLegacyCode(query, scope, "risk_case.tenant_id", "risk_case.brand_id", "TRIM(agent.remark)")
	var scopedCase model.RiskCase
	if err := query.Where("risk_case.case_no = ?", caseNo).First(&scopedCase).Error; err != nil {
		return model.RiskCase{}, nil, err
	}

	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "release" && action != "confirm" {
		return model.RiskCase{}, nil, errors.New("action must be release or confirm")
	}
	if scopedCase.Status != model.RiskCaseStatusPending {
		return model.RiskCase{}, nil, errors.New("risk case already reviewed")
	}

	if action == "confirm" {
		now := time.Now().UTC()
		scopedCase.Status = model.RiskCaseStatusConfirmed
		scopedCase.ReviewedAt = &now
		scopedCase.ReviewedBy = sharedsvc.FirstNonEmpty(strings.TrimSpace(reviewer), "finance")
		scopedCase.ReviewRemark = strings.TrimSpace(input.Remark)
		if err := s.db.Save(&scopedCase).Error; err != nil {
			return model.RiskCase{}, nil, err
		}
		if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
			EventType:      eventbus.EventRiskCaseConfirmed,
			AggregateType:  "risk_case",
			AggregateID:    strconv.FormatUint(scopedCase.ID, 10),
			TenantID:       scopedCase.TenantID,
			BrandID:        scopedCase.BrandID,
			OccurredAt:     now,
			Producer:       "risk-service",
			IdempotencyKey: "risk.review:" + scopedCase.CaseNo + ":" + string(scopedCase.Status),
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        riskCaseReviewedPayload(scopedCase, nil),
		}); err != nil {
			return model.RiskCase{}, nil, err
		}
		return scopedCase, nil, nil
	}

	ledger, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
		AgentID:        scopedCase.AgentID,
		ReferenceType:  "risk_case",
		ReferenceID:    caseNo,
		LedgerType:     model.LedgerTypeUnfreeze,
		Amount:         sharedsvc.Round2(scopedCase.Amount),
		Currency:       sharedsvc.FirstNonEmpty(strings.TrimSpace(scopedCase.Currency), "CNY"),
		IdempotencyKey: "risk-case-release:" + caseNo,
		Remark:         strings.TrimSpace(input.Remark),
	})
	if err != nil {
		return model.RiskCase{}, nil, err
	}
	now := time.Now().UTC()
	scopedCase.Status = model.RiskCaseStatusReleased
	scopedCase.ReviewedAt = &now
	scopedCase.ReviewedBy = sharedsvc.FirstNonEmpty(strings.TrimSpace(reviewer), "finance")
	scopedCase.ReviewRemark = strings.TrimSpace(input.Remark)
	if err := s.db.Save(&scopedCase).Error; err != nil {
		return model.RiskCase{}, nil, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventRiskCaseReleased,
		AggregateType:  "risk_case",
		AggregateID:    strconv.FormatUint(scopedCase.ID, 10),
		TenantID:       scopedCase.TenantID,
		BrandID:        scopedCase.BrandID,
		OccurredAt:     now,
		Producer:       "risk-service",
		IdempotencyKey: "risk.review:" + scopedCase.CaseNo + ":" + string(scopedCase.Status),
		Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
		Payload:        riskCaseReviewedPayload(scopedCase, &ledger.ID),
	}); err != nil {
		return model.RiskCase{}, nil, err
	}
	return scopedCase, &ledger, nil
}

func (s *Service) computeAgentAccountRiskMetrics(ctx context.Context, agentID uint64, account *model.AgentAccount, incomeAmount, manualReverseAmount float64) (float64, float64, error) {
	if account == nil {
		return 0, 0, nil
	}
	effectiveFrozen := account.FrozenBalance
	effectiveBalance := account.Balance
	if effectiveFrozen > 0 {
		return effectiveFrozen, effectiveBalance, nil
	}
	latestWithdrawal, err := s.withdrawalClient.FindLatestRiskWithdrawal(ctx, withdrawalcontract.LatestRiskWithdrawalRequest{AgentID: agentID})
	if err != nil {
		return 0, 0, err
	}
	if latestWithdrawal == nil {
		return effectiveFrozen, effectiveBalance, nil
	}
	if latestWithdrawal.Amount <= 0 {
		return effectiveFrozen, effectiveBalance, nil
	}
	effectiveFrozen = latestWithdrawal.Amount
	if incomeAmount > 0 {
		effectiveBalance = incomeAmount - manualReverseAmount
	}
	return effectiveFrozen, effectiveBalance, nil
}

func (s *Service) loadLatestWithdrawalForRisk(ctx context.Context, riskCase model.RiskCase) (*model.WithdrawalRequest, error) {
	return s.withdrawalClient.FindLatestRiskWithdrawal(ctx, withdrawalcontract.LatestRiskWithdrawalRequest{
		AgentID:  riskCase.AgentID,
		Currency: riskCase.Currency,
		TenantID: riskCase.TenantID,
		BrandID:  riskCase.BrandID,
	})
}

func (s *Service) loadRiskCaseAccountMetrics(ctx context.Context, riskCase model.RiskCase, snapshot accountcontract.RiskAccountSnapshot) (float64, float64, error) {
	if snapshot.Account == nil {
		frozen := sharedsvc.Round2(riskCase.Amount)
		return frozen, 0, nil
	}
	effectiveFrozen, effectiveBalance, err := s.computeAgentAccountRiskMetrics(ctx, riskCase.AgentID, snapshot.Account, snapshot.IncomeAmount, snapshot.ManualReverseAmount)
	if err != nil {
		return 0, 0, err
	}
	withdrawable := effectiveBalance - effectiveFrozen
	if withdrawable < 0 {
		withdrawable = 0
	}
	return sharedsvc.Round2(effectiveFrozen), sharedsvc.Round2(withdrawable), nil
}

func (s *Service) loadRiskAccountSnapshots(ctx context.Context, scope sharedsvc.Scope, rows []riskCaseRow) (map[uint64]accountcontract.RiskAccountSnapshot, error) {
	snapshots := make(map[uint64]accountcontract.RiskAccountSnapshot, len(rows))
	for _, row := range rows {
		if _, ok := snapshots[row.AgentID]; ok {
			continue
		}
		snapshot, err := s.accountClient.GetRiskAccountSnapshot(ctx, scope, row.AgentID)
		if err != nil {
			return nil, err
		}
		snapshots[row.AgentID] = snapshot
	}
	return snapshots, nil
}

func derefRiskAccount(account *model.AgentAccount) model.AgentAccount {
	if account == nil {
		return model.AgentAccount{}
	}
	return *account
}

func classifyFrozenRisk(ratio float64) string { return sharedsvc.ClassifyFrozenRisk(ratio) }

func riskBaseScore(level string) float64 {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high":
		return 88
	case "medium":
		return 72
	default:
		return 58
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
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

func uint64Ptr(value uint64) *uint64 { return &value }

func (s *Service) ensureAgentExists(agentID uint64) error {
	return s.repo.EnsureAgentExists(agentID)
}

func (s *Service) ensureScopeReferences(scope sharedsvc.Scope, tenantID, brandID *uint64) error {
	scope = sharedsvc.NormalizeScope(scope)
	if tenantID != nil {
		if len(scope.TenantIDs) > 0 && !slices.Contains(scope.TenantIDs, *tenantID) {
			return gorm.ErrRecordNotFound
		}
		if _, err := sharedsvc.LoadTenant(s.db, *tenantID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("tenant not found")
			}
			return err
		}
	}
	if brandID != nil {
		brand, err := sharedsvc.LoadBrand(s.db, *brandID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("brand not found")
			}
			return err
		}
		if tenantID != nil && brand.TenantID != *tenantID {
			return errors.New("brand does not belong to tenant")
		}
		if !sharedsvc.ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
			return gorm.ErrRecordNotFound
		}
		if !sharedsvc.ScopeAllowsTenant(scope, brand.TenantID) && !sharedsvc.ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func validateAgentLedgerType(ledgerType model.LedgerType) error {
	return sharedsvc.ValidateAgentLedgerType(ledgerType)
}
