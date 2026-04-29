package settlement

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type BillFilter struct {
	AgentID string
	Status  string
	BillNo  string
}

type CreateBillInput struct {
	AgentID     uint64
	PeriodStart string
	PeriodEnd   string
	Currency    string
	Remark      string
	Adjustment  float64
	Cycle       string
}

type BillDetailItem struct {
	model.SettlementBillDetail
	RecordNo string
	OrderNo  string
}

type BillListItem struct {
	model.SettlementBill
	AgentName string
	Details   []BillDetailItem
}

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) ListBills(scope sharedsvc.Scope, filter BillFilter) ([]model.SettlementBill, error) {
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, err
		}
	}
	return s.repo.ListBills(scope, filter)
}

func (s *Service) ListBillItems(scope sharedsvc.Scope, filter BillFilter) ([]BillListItem, error) {
	bills, err := s.ListBills(scope, filter)
	if err != nil {
		return nil, err
	}
	items := make([]BillListItem, 0, len(bills))
	for _, bill := range bills {
		item, err := s.BuildBillListItem(bill)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) GenerateBill(scope sharedsvc.Scope, input CreateBillInput) (model.SettlementBill, error) {
	bill, _, err := s.generateBill(scope, input, nil)
	return bill, err
}

func (s *Service) GenerateBillItem(scope sharedsvc.Scope, input CreateBillInput) (BillListItem, error) {
	bill, err := s.GenerateBill(scope, input)
	if err != nil {
		return BillListItem{}, err
	}
	return s.BuildBillListItem(bill)
}

func (s *Service) GenerateBillWithSourceTask(scope sharedsvc.Scope, input CreateBillInput, sourceTaskID *uint64) (model.SettlementBill, int, error) {
	bill, detailCount, err := s.generateBill(scope, input, sourceTaskID)
	return bill, detailCount, err
}

func (s *Service) generateBill(scope sharedsvc.Scope, input CreateBillInput, sourceTaskID *uint64) (model.SettlementBill, int, error) {
	agent, err := s.repo.FindAgent(input.AgentID)
	if err != nil {
		return model.SettlementBill{}, 0, err
	}
	if !sharedsvc.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
		return model.SettlementBill{}, 0, gorm.ErrRecordNotFound
	}
	periodStart, periodEnd, cycleLabel, err := resolveSettlementPeriod(input)
	if err != nil {
		return model.SettlementBill{}, 0, err
	}
	currency := sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), "CNY")

	var bill model.SettlementBill
	detailCount := 0
	err = s.repo.WithTx(func(tx *gorm.DB) error {
		records, err := s.repo.ListCommissionRecordsForBill(input.AgentID, periodStart, periodEnd)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		bill = model.SettlementBill{
			TenantID:         agent.TenantID,
			BrandID:          agent.BrandID,
			BillNo:           fmt.Sprintf("SB-%d-%d", input.AgentID, now.UnixNano()),
			AgentID:          input.AgentID,
			PeriodStart:      periodStart,
			PeriodEnd:        periodEnd,
			Currency:         currency,
			Status:           model.SettlementBillStatusGenerated,
			AdjustmentAmount: sharedsvc.Round2(input.Adjustment),
			GeneratedAt:      &now,
			SourceTaskID:     sourceTaskID,
			Remark:           strings.TrimSpace(input.Remark),
			SummaryPayload:   sharedsvc.MustJSONBytes(map[string]any{"recordCount": len(records), "cycle": cycleLabel}),
		}

		detailRows := make([]model.SettlementBillDetail, 0, len(records))
		commissionAmount := 0.0
		for _, record := range records {
			amount := sharedsvc.Round2(record.CommissionAmount)
			commissionAmount += amount
			orderID := record.RechargeOrderID
			recordID := record.ID
			detailRows = append(detailRows, model.SettlementBillDetail{
				TenantID:           record.TenantID,
				BrandID:            record.BrandID,
				AgentID:            input.AgentID,
				CommissionRecordID: &recordID,
				RechargeOrderID:    &orderID,
				ReferenceType:      "commission_record",
				ReferenceID:        record.RecordNo,
				CommissionAmount:   amount,
				AdjustmentAmount:   0,
				Amount:             amount,
				Currency:           sharedsvc.FirstNonEmpty(strings.TrimSpace(record.Currency), currency),
				OccurredAt:         record.EstimatedAt,
				SnapshotPayload:    sharedsvc.MustJSONBytes(record),
				Remark:             strings.TrimSpace(record.Remark),
			})
		}
		bill.CommissionAmount = sharedsvc.Round2(commissionAmount)
		bill.PayableAmount = sharedsvc.Round2(bill.CommissionAmount + bill.AdjustmentAmount)
		if err := s.repo.CreateBillTx(tx, &bill); err != nil {
			return err
		}
		if len(detailRows) > 0 {
			for i := range detailRows {
				detailRows[i].SettlementBillID = bill.ID
			}
			detailCount = len(detailRows)
			if err := s.repo.CreateBillDetailsTx(tx, detailRows); err != nil {
				return err
			}
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventSettlementBillGenerated,
			AggregateType:  "settlement_bill",
			AggregateID:    strconv.FormatUint(bill.ID, 10),
			TenantID:       bill.TenantID,
			BrandID:        bill.BrandID,
			OccurredAt:     now,
			Producer:       "settlement-service",
			IdempotencyKey: "settlement.generated:" + bill.BillNo,
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        settlementBillGeneratedPayload(bill, detailCount),
		})
		return err
	})
	return bill, detailCount, err
}

func (s *Service) ConfirmBill(scope sharedsvc.Scope, id uint64, confirmer string) (model.SettlementBill, model.SettlementBill, error) {
	bill, err := s.loadScopedBill(scope, id)
	if err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, err
	}
	if bill.Status != model.SettlementBillStatusGenerated {
		return model.SettlementBill{}, model.SettlementBill{}, fmt.Errorf("settlement bill %d cannot be confirmed in status %s", id, bill.Status)
	}
	before := bill
	confirmed := bill
	now := time.Now().UTC()
	confirmed.Status = model.SettlementBillStatusConfirmed
	confirmed.ConfirmedAt = &now
	confirmed.ConfirmedBy = sharedsvc.FirstNonEmpty(strings.TrimSpace(confirmer), "system")
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		if err := s.repo.UpdateBillConfirmationTx(tx, id, confirmed.Status, confirmed.ConfirmedAt, confirmed.ConfirmedBy, now); err != nil {
			return err
		}
		if confirmed, err = s.repo.ReloadBillTx(tx, id); err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventSettlementBillConfirmed,
			AggregateType:  "settlement_bill",
			AggregateID:    strconv.FormatUint(confirmed.ID, 10),
			TenantID:       confirmed.TenantID,
			BrandID:        confirmed.BrandID,
			OccurredAt:     now,
			Producer:       "settlement-service",
			IdempotencyKey: "settlement.confirmed:" + confirmed.BillNo,
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        settlementBillConfirmedPayload(confirmed),
		})
		return err
	}); err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, err
	}
	return before, confirmed, nil
}

func (s *Service) ConfirmBillItem(scope sharedsvc.Scope, id uint64, confirmer string) (model.SettlementBill, model.SettlementBill, BillListItem, error) {
	before, confirmed, err := s.ConfirmBill(scope, id, confirmer)
	if err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, BillListItem{}, err
	}
	item, err := s.BuildBillListItem(confirmed)
	if err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, BillListItem{}, err
	}
	return before, confirmed, item, nil
}

func (s *Service) ExportBill(scope sharedsvc.Scope, id uint64) (model.SettlementBill, model.SettlementBill, error) {
	bill, err := s.loadScopedBill(scope, id)
	if err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, err
	}
	if bill.Status != model.SettlementBillStatusGenerated && bill.Status != model.SettlementBillStatusConfirmed {
		return model.SettlementBill{}, model.SettlementBill{}, fmt.Errorf("settlement bill %d cannot be exported in status %s", id, bill.Status)
	}
	before := bill
	exported := bill
	now := time.Now().UTC()
	summary := map[string]any{}
	if len(before.SummaryPayload) > 0 {
		_ = json.Unmarshal(before.SummaryPayload, &summary)
	}
	summary["exportedAt"] = now.Format(time.RFC3339)
	exported.SummaryPayload = sharedsvc.MustJSONBytes(summary)
	if err := s.repo.UpdateBillSummaryPayload(id, exported.SummaryPayload, now); err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, err
	}
	if exported, err = s.repo.ReloadBill(id); err != nil {
		return model.SettlementBill{}, model.SettlementBill{}, err
	}
	return before, exported, nil
}

func (s *Service) loadAgent(id uint64) (model.Agent, error) {
	return s.repo.FindAgent(id)
}

func (s *Service) BuildBillListItem(bill model.SettlementBill) (BillListItem, error) {
	return BillListItem{
		SettlementBill: bill,
		AgentName:      s.loadAgentName(bill.AgentID),
		Details:        s.loadSettlementBillDetails(bill.ID),
	}, nil
}

func (s *Service) loadScopedBill(scope sharedsvc.Scope, id uint64) (model.SettlementBill, error) {
	return s.repo.FindScopedBill(scope, id)
}

func (s *Service) loadAgentName(agentID uint64) string {
	return s.repo.LoadAgentName(agentID)
}

func (s *Service) loadSettlementBillDetails(billID uint64) []BillDetailItem {
	rows, err := s.repo.ListBillDetails(billID)
	if err != nil {
		return nil
	}
	items := make([]BillDetailItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, BillDetailItem{
			SettlementBillDetail: row,
			RecordNo:             row.ReferenceID,
			OrderNo:              s.repo.LoadRechargeOrderNo(row.RechargeOrderID),
		})
	}
	return items
}

func (s *Service) loadRechargeOrderNo(orderID *uint64) string {
	return s.repo.LoadRechargeOrderNo(orderID)
}

func resolveSettlementPeriod(input CreateBillInput) (time.Time, time.Time, string, error) {
	trimmedCycle := strings.ToLower(strings.TrimSpace(input.Cycle))
	if trimmedCycle == "" {
		periodStart, err := time.Parse(time.RFC3339, strings.TrimSpace(input.PeriodStart))
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid periodStart: %w", err)
		}
		periodEnd, err := time.Parse(time.RFC3339, strings.TrimSpace(input.PeriodEnd))
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid periodEnd: %w", err)
		}
		periodStart = periodStart.UTC()
		periodEnd = periodEnd.UTC()
		if !periodEnd.After(periodStart) {
			return time.Time{}, time.Time{}, "", errors.New("periodEnd must be after periodStart")
		}
		return periodStart, periodEnd, deriveSettlementCycle(periodStart, periodEnd), nil
	}

	base, err := time.Parse(time.RFC3339, strings.TrimSpace(input.PeriodStart))
	if err != nil {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid periodStart: %w", err)
	}
	periodStart, periodEnd, err := normalizeSettlementCycleWindow(trimmedCycle, base.UTC())
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	if trimmedPeriodEnd := strings.TrimSpace(input.PeriodEnd); trimmedPeriodEnd != "" {
		explicitEnd, err := time.Parse(time.RFC3339, trimmedPeriodEnd)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid periodEnd: %w", err)
		}
		if !explicitEnd.UTC().Equal(periodEnd) {
			return time.Time{}, time.Time{}, "", fmt.Errorf("periodEnd does not match %s cycle derived end", trimmedCycle)
		}
	}
	return periodStart, periodEnd, trimmedCycle, nil
}

func deriveSettlementCycle(periodStart, periodEnd time.Time) string {
	if normalizedStart, normalizedEnd, err := normalizeSettlementCycleWindow("daily", periodStart); err == nil && normalizedStart.Equal(periodStart) && normalizedEnd.Equal(periodEnd) {
		return "daily"
	}
	if normalizedStart, normalizedEnd, err := normalizeSettlementCycleWindow("weekly", periodStart); err == nil && normalizedStart.Equal(periodStart) && normalizedEnd.Equal(periodEnd) {
		return "weekly"
	}
	if normalizedStart, normalizedEnd, err := normalizeSettlementCycleWindow("monthly", periodStart); err == nil && normalizedStart.Equal(periodStart) && normalizedEnd.Equal(periodEnd) {
		return "monthly"
	}
	return "custom"
}

func normalizeSettlementCycleWindow(cycle string, base time.Time) (time.Time, time.Time, error) {
	t := base.UTC()
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case "daily":
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 0, 1), nil
	case "weekly":
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		weekdayOffset := (int(start.Weekday()) + 6) % 7
		start = start.AddDate(0, 0, -weekdayOffset)
		return start, start.AddDate(0, 0, 7), nil
	case "monthly":
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported settlement cycle %q", cycle)
	}
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
