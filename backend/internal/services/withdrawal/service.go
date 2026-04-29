package withdrawal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	accountcontract "game-admin/backend/internal/contract/account"
	riskcontract "game-admin/backend/internal/contract/risk"
	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	db            *gorm.DB
	repo          Repository
	accountClient accountcontract.Client
	riskClient    riskcontract.Client
}

type ListFilter struct {
	TenantCode string
	Status     string
	AgentID    string
	RequestNo  string
}

type ListItem struct {
	model.WithdrawalRequest
	AgentName string
}

type CreateInput struct {
	TenantCode      string
	TenantID        *uint64
	BrandID         *uint64
	AgentID         uint64
	Amount          float64
	Currency        string
	Channel         string
	BankAccountName string
	BankAccountNo   string
	BankName        string
	IdempotencyKey  string
	Remark          string
}

type ReviewInput struct {
	Action string
	Remark string
}

type PayoutInput struct {
	Action         string
	Remark         string
	Reference      string
	ReceiptPayload map[string]any
}

func NewService(db *gorm.DB) *Service {
	return NewServiceWithClients(
		db,
		accountcontract.NewLocalDBClient(db),
		riskcontract.NewLocalDBClient(db),
	)
}

func NewReadService(db *gorm.DB) *Service {
	return &Service{db: db, repo: newRepository(db)}
}

func NewServiceWithAccountClient(db *gorm.DB, accountClient accountcontract.Client) *Service {
	return NewServiceWithClients(db, accountClient, riskcontract.NewLocalDBClient(db))
}

func NewServiceWithClients(db *gorm.DB, accountClient accountcontract.Client, riskClient riskcontract.Client) *Service {
	return &Service{db: db, repo: newRepository(db), accountClient: accountClient, riskClient: riskClient}
}

func (s *Service) List(scope sharedsvc.Scope, filter ListFilter) ([]ListItem, error) {
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, errors.New("invalid agentID")
		}
	}
	rows, err := s.repo.ListWithdrawals(scope, filter)
	if err != nil {
		return nil, err
	}
	items := make([]ListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ListItem{WithdrawalRequest: row, AgentName: s.loadAgentName(row.AgentID)})
	}
	return items, nil
}

func (s *Service) ListRiskSignals(scope sharedsvc.Scope, limit int) ([]RiskSignal, error) {
	return s.repo.ListRiskSignals(scope, limit)
}

func (s *Service) FindLatestRiskWithdrawal(query LatestRiskWithdrawalQuery) (*model.WithdrawalRequest, error) {
	return s.repo.FindLatestRiskWithdrawal(query)
}

func (s *Service) Create(ctx context.Context, scope sharedsvc.Scope, input CreateInput, submittedBy string) (model.WithdrawalRequest, error) {
	agent, err := s.ensureAgentInCurrentScope(scope, input.AgentID)
	if err != nil {
		return model.WithdrawalRequest{}, err
	}
	if input.TenantID == nil {
		input.TenantID = agent.TenantID
	}
	if input.BrandID == nil {
		input.BrandID = agent.BrandID
	}
	if input.TenantID != nil && agent.TenantID != nil && *input.TenantID != *agent.TenantID {
		return model.WithdrawalRequest{}, gorm.ErrRecordNotFound
	}
	if input.BrandID != nil && agent.BrandID != nil && *input.BrandID != *agent.BrandID {
		return model.WithdrawalRequest{}, gorm.ErrRecordNotFound
	}
	if err := s.ensureScopeReferences(scope, input.TenantID, input.BrandID); err != nil {
		return model.WithdrawalRequest{}, err
	}
	if len(scope.TenantCodes) > 0 {
		tenantCode := strings.TrimSpace(input.TenantCode)
		if tenantCode != "" && !slices.Contains(scope.TenantCodes, tenantCode) {
			return model.WithdrawalRequest{}, gorm.ErrRecordNotFound
		}
	}
	return s.create(ctx, scope, input, submittedBy)
}

func (s *Service) Review(ctx context.Context, scope sharedsvc.Scope, id uint64, input ReviewInput, reviewer string) (model.WithdrawalRequest, model.WithdrawalRequest, error) {
	withdrawal, err := s.loadScopedWithdrawal(scope, id)
	if err != nil {
		return model.WithdrawalRequest{}, model.WithdrawalRequest{}, err
	}
	return s.review(ctx, scope, withdrawal.ID, input, reviewer)
}

func (s *Service) Payout(ctx context.Context, scope sharedsvc.Scope, id uint64, input PayoutInput, operator string) (model.WithdrawalRequest, model.WithdrawalRequest, error) {
	withdrawal, err := s.loadScopedWithdrawal(scope, id)
	if err != nil {
		return model.WithdrawalRequest{}, model.WithdrawalRequest{}, err
	}
	return s.payout(ctx, scope, withdrawal.ID, input, operator)
}

func (s *Service) loadScopedWithdrawal(scope sharedsvc.Scope, id uint64) (model.WithdrawalRequest, error) {
	withdrawal, err := s.repo.FindWithdrawal(id)
	if err != nil {
		return model.WithdrawalRequest{}, err
	}
	if !sharedsvc.MatchesScopedRecord(scope, withdrawal.TenantID, withdrawal.BrandID) {
		return model.WithdrawalRequest{}, gorm.ErrRecordNotFound
	}
	if len(scope.TenantCodes) > 0 {
		tenantCode := strings.TrimSpace(withdrawal.TenantCode)
		if tenantCode == "" || !slices.Contains(scope.TenantCodes, tenantCode) {
			return model.WithdrawalRequest{}, gorm.ErrRecordNotFound
		}
	}
	return withdrawal, nil
}

func (s *Service) create(ctx context.Context, scope sharedsvc.Scope, input CreateInput, submittedBy string) (model.WithdrawalRequest, error) {
	if input.AgentID == 0 {
		return model.WithdrawalRequest{}, errors.New("agentID is required")
	}
	if input.Amount <= 0 {
		return model.WithdrawalRequest{}, errors.New("amount must be greater than 0")
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return model.WithdrawalRequest{}, errors.New("idempotencyKey is required")
	}
	if err := s.ensureAgentExists(input.AgentID); err != nil {
		return model.WithdrawalRequest{}, err
	}
	var request model.WithdrawalRequest
	ledger, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
		AgentID:        input.AgentID,
		ReferenceType:  "withdrawal_request",
		ReferenceID:    strings.TrimSpace(input.IdempotencyKey),
		LedgerType:     model.LedgerTypeFreeze,
		Amount:         sharedsvc.Round2(input.Amount),
		Currency:       sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), "CNY"),
		IdempotencyKey: "withdrawal-freeze:" + strings.TrimSpace(input.IdempotencyKey),
		Remark:         strings.TrimSpace(input.Remark),
	})
	if err != nil {
		return model.WithdrawalRequest{}, err
	}
	compensations := []func() error{
		func() error {
			_, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
				AgentID:        input.AgentID,
				ReferenceType:  "withdrawal_request_compensation",
				ReferenceID:    strings.TrimSpace(input.IdempotencyKey),
				LedgerType:     model.LedgerTypeUnfreeze,
				Amount:         sharedsvc.Round2(input.Amount),
				Currency:       sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), "CNY"),
				IdempotencyKey: "withdrawal-compensate-create:" + strings.TrimSpace(input.IdempotencyKey),
				Remark:         "compensate withdrawal create",
			})
			return err
		},
	}
	err = s.repo.WithTx(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		bankSnapshot, err := json.Marshal(map[string]any{
			"bankAccountName": strings.TrimSpace(input.BankAccountName),
			"bankAccountNo":   strings.TrimSpace(input.BankAccountNo),
			"bankName":        strings.TrimSpace(input.BankName),
			"channel":         strings.TrimSpace(input.Channel),
		})
		if err != nil {
			return err
		}
		request = model.WithdrawalRequest{
			RequestNo:           fmt.Sprintf("WD-%d-%d", input.AgentID, now.UnixNano()),
			TenantID:            input.TenantID,
			BrandID:             input.BrandID,
			TenantCode:          sharedsvc.FirstNonEmpty(strings.TrimSpace(input.TenantCode), "default"),
			AgentID:             input.AgentID,
			AccountID:           ledger.AccountID,
			Currency:            sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), ledger.Currency, "CNY"),
			Amount:              sharedsvc.Round2(input.Amount),
			FeeAmount:           0,
			TaxAmount:           0,
			PayableAmount:       sharedsvc.Round2(input.Amount),
			Status:              model.WithdrawalStatusPending,
			Channel:             strings.TrimSpace(input.Channel),
			BankAccountName:     strings.TrimSpace(input.BankAccountName),
			BankAccountNo:       strings.TrimSpace(input.BankAccountNo),
			BankName:            strings.TrimSpace(input.BankName),
			BankAccountSnapshot: datatypes.JSON(bankSnapshot),
			SubmittedBy:         sharedsvc.FirstNonEmpty(strings.TrimSpace(submittedBy), "system"),
			ApprovedLedgerID:    &ledger.ID,
			IdempotencyKey:      strings.TrimSpace(input.IdempotencyKey),
			Remark:              strings.TrimSpace(input.Remark),
		}
		if err := s.repo.CreateWithdrawalTx(tx, &request); err != nil {
			return err
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventWithdrawalCreated,
			AggregateType:  "withdrawal_request",
			AggregateID:    strconv.FormatUint(request.ID, 10),
			TenantID:       request.TenantID,
			BrandID:        request.BrandID,
			OccurredAt:     now,
			Producer:       "withdrawal-service",
			IdempotencyKey: "withdrawal.created:" + request.RequestNo,
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        withdrawalCreatedPayload(request),
		})
		return err
	})
	if err != nil {
		_ = runCompensations(compensations)
	}
	return request, err
}

func (s *Service) review(ctx context.Context, scope sharedsvc.Scope, id uint64, input ReviewInput, reviewer string) (model.WithdrawalRequest, model.WithdrawalRequest, error) {
	var before model.WithdrawalRequest
	if err := s.db.First(&before, id).Error; err != nil {
		return model.WithdrawalRequest{}, model.WithdrawalRequest{}, err
	}
	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "approve" && action != "reject" {
		return before, model.WithdrawalRequest{}, errors.New("action must be approve or reject")
	}
	if before.Status != model.WithdrawalStatusPending {
		if action == "approve" && before.Status == model.WithdrawalStatusApproved {
			return before, before, nil
		}
		if action == "reject" && before.Status == model.WithdrawalStatusRejected {
			return before, before, nil
		}
		return before, model.WithdrawalRequest{}, errors.New("withdrawal request is not pending")
	}
	var after model.WithdrawalRequest
	var compensations []func() error
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		current := before
		now := time.Now().UTC()
		current.ReviewedBy = sharedsvc.FirstNonEmpty(strings.TrimSpace(reviewer), "system")
		current.ReviewedAt = &now
		current.Remark = strings.TrimSpace(input.Remark)
		if action == "approve" {
			current.Status = model.WithdrawalStatusApproved
		} else {
			ledger, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
				AgentID:        current.AgentID,
				ReferenceType:  "withdrawal_request",
				ReferenceID:    current.RequestNo,
				LedgerType:     model.LedgerTypeUnfreeze,
				Amount:         current.Amount,
				Currency:       current.Currency,
				IdempotencyKey: "withdrawal-reject:" + current.IdempotencyKey,
				Remark:         strings.TrimSpace(input.Remark),
			})
			if err != nil {
				return err
			}
			compensations = append(compensations, func() error {
				_, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
					AgentID:        current.AgentID,
					ReferenceType:  "withdrawal_request_compensation",
					ReferenceID:    current.RequestNo,
					LedgerType:     model.LedgerTypeFreeze,
					Amount:         current.Amount,
					Currency:       current.Currency,
					IdempotencyKey: "withdrawal-compensate-reject:" + current.IdempotencyKey,
					Remark:         "compensate withdrawal reject",
				})
				return err
			})
			current.Status = model.WithdrawalStatusRejected
			current.FailureLedgerID = &ledger.ID
		}
		if err := s.repo.SaveWithdrawalTx(tx, &current); err != nil {
			return err
		}
		eventType := eventbus.EventWithdrawalRejected
		if current.Status == model.WithdrawalStatusApproved {
			eventType = eventbus.EventWithdrawalApproved
		}
		if _, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventType,
			AggregateType:  "withdrawal_request",
			AggregateID:    strconv.FormatUint(current.ID, 10),
			TenantID:       current.TenantID,
			BrandID:        current.BrandID,
			OccurredAt:     now,
			Producer:       "withdrawal-service",
			IdempotencyKey: "withdrawal.review:" + current.RequestNo + ":" + string(current.Status),
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        withdrawalReviewedPayload(current),
		}); err != nil {
			return err
		}
		after = current
		return nil
	})
	if err != nil {
		_ = runCompensations(compensations)
	}
	return before, after, err
}

func (s *Service) payout(ctx context.Context, scope sharedsvc.Scope, id uint64, input PayoutInput, operator string) (model.WithdrawalRequest, model.WithdrawalRequest, error) {
	var before model.WithdrawalRequest
	if err := s.db.First(&before, id).Error; err != nil {
		return model.WithdrawalRequest{}, model.WithdrawalRequest{}, err
	}
	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "success" && action != "fail" && action != "return" {
		return before, model.WithdrawalRequest{}, errors.New("action must be success, fail or return")
	}
	if before.Status == model.WithdrawalStatusPaid && action == "success" {
		return before, before, nil
	}
	if before.Status == model.WithdrawalStatusFailed && action == "fail" {
		return before, before, nil
	}
	if before.Status == model.WithdrawalStatusReturned && action == "return" {
		return before, before, nil
	}
	if before.Status != model.WithdrawalStatusApproved {
		return before, model.WithdrawalRequest{}, errors.New("withdrawal request is not approved")
	}
	var after model.WithdrawalRequest
	var compensations []func() error
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		current := before
		now := time.Now().UTC()
		current.ReviewedBy = sharedsvc.FirstNonEmpty(strings.TrimSpace(operator), current.ReviewedBy, "system")
		current.Remark = strings.TrimSpace(input.Remark)
		current.PayoutReference = strings.TrimSpace(input.Reference)
		if len(input.ReceiptPayload) > 0 {
			receipt, err := json.Marshal(input.ReceiptPayload)
			if err != nil {
				return err
			}
			current.PayoutReceiptPayload = datatypes.JSON(receipt)
		}
		current.Status = model.WithdrawalStatusPaying
		switch action {
		case "success":
			if current.ApprovedLedgerID != nil {
				if _, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
					AgentID:        current.AgentID,
					ReferenceType:  "withdrawal_request",
					ReferenceID:    current.RequestNo,
					LedgerType:     model.LedgerTypeUnfreeze,
					Amount:         current.PayableAmount,
					Currency:       current.Currency,
					IdempotencyKey: "withdrawal-complete-unfreeze:" + current.IdempotencyKey,
					Remark:         strings.TrimSpace(input.Remark),
				}); err != nil {
					return err
				}
				compensations = append(compensations, func() error {
					_, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
						AgentID:        current.AgentID,
						ReferenceType:  "withdrawal_request_compensation",
						ReferenceID:    current.RequestNo,
						LedgerType:     model.LedgerTypeFreeze,
						Amount:         current.PayableAmount,
						Currency:       current.Currency,
						IdempotencyKey: "withdrawal-compensate-complete-unfreeze:" + current.IdempotencyKey,
						Remark:         "compensate withdrawal complete unfreeze",
					})
					return err
				})
			}
			settlementLedger, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
				AgentID:        current.AgentID,
				ReferenceType:  "withdrawal_request",
				ReferenceID:    current.RequestNo,
				LedgerType:     model.LedgerTypeReverse,
				Amount:         current.PayableAmount,
				Currency:       current.Currency,
				IdempotencyKey: "withdrawal-complete:" + current.IdempotencyKey,
				Remark:         strings.TrimSpace(input.Remark),
			})
			if err != nil {
				return err
			}
			compensations = append(compensations, func() error {
				_, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
					AgentID:        current.AgentID,
					ReferenceType:  "withdrawal_request_compensation",
					ReferenceID:    current.RequestNo,
					LedgerType:     model.LedgerTypeIncome,
					Amount:         current.PayableAmount,
					Currency:       current.Currency,
					IdempotencyKey: "withdrawal-compensate-complete-reverse:" + current.IdempotencyKey,
					Remark:         "compensate withdrawal complete reverse",
				})
				return err
			})
			releasedCaseNos, err := s.riskClient.ReleasePendingCases(ctx, scope, current.AgentID, strings.TrimSpace(input.Remark), sharedsvc.FirstNonEmpty(strings.TrimSpace(operator), "finance"))
			if err != nil {
				return err
			}
			if len(releasedCaseNos) > 0 {
				compensations = append(compensations, func() error {
					_, err := s.riskClient.RestorePendingCases(ctx, scope, releasedCaseNos, "compensate withdrawal payout", sharedsvc.FirstNonEmpty(strings.TrimSpace(operator), "finance"))
					return err
				})
			}
			if err != nil {
				return err
			}
			current.Status = model.WithdrawalStatusPaid
			current.PaidAt = &now
			current.CompletedLedgerID = &settlementLedger.ID
		case "fail", "return":
			ledger, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
				AgentID:        current.AgentID,
				ReferenceType:  "withdrawal_request",
				ReferenceID:    current.RequestNo,
				LedgerType:     model.LedgerTypeUnfreeze,
				Amount:         current.Amount,
				Currency:       current.Currency,
				IdempotencyKey: "withdrawal-" + action + ":" + current.IdempotencyKey,
				Remark:         strings.TrimSpace(input.Remark),
			})
			if err != nil {
				return err
			}
			compensations = append(compensations, func() error {
				_, err := s.accountClient.CreateLedger(ctx, scope, accountcontract.LedgerCreateInput{
					AgentID:        current.AgentID,
					ReferenceType:  "withdrawal_request_compensation",
					ReferenceID:    current.RequestNo,
					LedgerType:     model.LedgerTypeFreeze,
					Amount:         current.Amount,
					Currency:       current.Currency,
					IdempotencyKey: "withdrawal-compensate-" + action + ":" + current.IdempotencyKey,
					Remark:         "compensate withdrawal " + action,
				})
				return err
			})
			if action == "fail" {
				current.Status = model.WithdrawalStatusFailed
			} else {
				current.Status = model.WithdrawalStatusReturned
			}
			current.FailureLedgerID = &ledger.ID
		}
		if err := s.repo.SaveWithdrawalTx(tx, &current); err != nil {
			return err
		}
		eventType := eventbus.EventWithdrawalPaid
		switch current.Status {
		case model.WithdrawalStatusFailed:
			eventType = eventbus.EventWithdrawalFailed
		case model.WithdrawalStatusReturned:
			eventType = eventbus.EventWithdrawalReturned
		}
		if _, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventType,
			AggregateType:  "withdrawal_request",
			AggregateID:    strconv.FormatUint(current.ID, 10),
			TenantID:       current.TenantID,
			BrandID:        current.BrandID,
			OccurredAt:     now,
			Producer:       "withdrawal-service",
			IdempotencyKey: "withdrawal.payout:" + current.RequestNo + ":" + string(current.Status),
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        withdrawalPayoutPayload(current),
		}); err != nil {
			return err
		}
		after = current
		return nil
	})
	if err != nil {
		_ = runCompensations(compensations)
	}
	return before, after, err
}

func runCompensations(actions []func() error) error {
	for index := len(actions) - 1; index >= 0; index-- {
		if actions[index] == nil {
			continue
		}
		if err := actions[index](); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureAgentExists(agentID uint64) error {
	return s.repo.EnsureAgentExists(agentID)
}

func (s *Service) ensureAgentInCurrentScope(scope sharedsvc.Scope, agentID uint64) (model.Agent, error) {
	agent, err := s.repo.FindAgent(agentID)
	if err != nil {
		return model.Agent{}, err
	}
	if !sharedsvc.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
		return model.Agent{}, gorm.ErrRecordNotFound
	}
	if scope.AgentID != nil {
		ids, err := s.repo.CurrentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return model.Agent{}, err
		}
		if !slices.Contains(ids, agent.ID) {
			return model.Agent{}, gorm.ErrRecordNotFound
		}
	}
	return agent, nil
}

func (s *Service) currentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return s.repo.CurrentAgentScopeIDs(agentID)
}

func (s *Service) ensureScopeReferences(scope sharedsvc.Scope, tenantID *uint64, brandID *uint64) error {
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

func (s *Service) loadAgentName(agentID uint64) string {
	return s.repo.LoadAgentName(agentID)
}
