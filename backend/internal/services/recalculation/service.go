package recalculation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	settlementcontract "game-admin/backend/internal/contract/settlement"
	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	db               *gorm.DB
	repo             Repository
	settlementClient settlementcontract.Client
}

type ListItem struct {
	model.RecalculationTask
	AgentName string
	BillNo    string
}

type CreateInput struct {
	TaskType         model.RecalculationTaskType
	AgentID          *uint64
	SettlementBillID *uint64
	PeriodStart      string
	PeriodEnd        string
	Operator         string
	Reason           string
	Scope            string
	Remark           string
}

func NewService(db *gorm.DB) *Service {
	return NewServiceWithSettlementClient(db, settlementcontract.NewLocalDBClient(db))
}

func NewServiceWithSettlementClient(db *gorm.DB, settlementClient settlementcontract.Client) *Service {
	return &Service{
		db:               db,
		repo:             newRepository(db),
		settlementClient: settlementClient,
	}
}

func (s *Service) List(scope sharedsvc.Scope, statusValue string) ([]ListItem, error) {
	tasks, err := s.repo.ListTasks(scope, strings.TrimSpace(statusValue))
	if err != nil {
		return nil, err
	}
	items := make([]ListItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, s.BuildListItem(task))
	}
	return items, nil
}

func (s *Service) BuildListItem(task model.RecalculationTask) ListItem {
	item := ListItem{RecalculationTask: task}
	if task.AgentID != nil {
		item.AgentName = s.loadAgentName(*task.AgentID)
	}
	if task.SettlementBillID != nil {
		item.BillNo = s.loadSettlementBillNo(*task.SettlementBillID)
	}
	return item
}

func (s *Service) Create(ctx context.Context, scope sharedsvc.Scope, payload CreateInput, requestedBy string) (model.RecalculationTask, error) {
	if payload.AgentID != nil {
		if _, err := s.ensureAgentInCurrentScope(scope, *payload.AgentID); err != nil {
			return model.RecalculationTask{}, err
		}
	}
	if payload.SettlementBillID != nil {
		bill, err := s.repo.FindSettlementBillInScope(scope, *payload.SettlementBillID)
		if err != nil {
			return model.RecalculationTask{}, err
		}
		if _, err := s.ensureAgentInCurrentScope(scope, bill.AgentID); err != nil {
			return model.RecalculationTask{}, err
		}
	}
	return s.create(ctx, scope, payload, requestedBy)
}

func (s *Service) create(ctx context.Context, scope sharedsvc.Scope, payload CreateInput, requestedBy string) (model.RecalculationTask, error) {
	taskType := payload.TaskType
	if taskType == "" {
		taskType = model.RecalculationTaskTypeSettlementBill
	}
	periodStart, err := sharedsvc.ParseOptionalTime(payload.PeriodStart)
	if err != nil {
		return model.RecalculationTask{}, fmt.Errorf("invalid periodStart: %w", err)
	}
	periodEnd, err := sharedsvc.ParseOptionalTime(payload.PeriodEnd)
	if err != nil {
		return model.RecalculationTask{}, fmt.Errorf("invalid periodEnd: %w", err)
	}
	if periodStart != nil && periodEnd != nil && !periodEnd.After(*periodStart) {
		return model.RecalculationTask{}, errors.New("periodEnd must be after periodStart")
	}
	if payload.AgentID != nil {
		if err := s.ensureAgentExists(*payload.AgentID); err != nil {
			return model.RecalculationTask{}, err
		}
	}
	var taskTenantID *uint64
	var taskBrandID *uint64
	if payload.SettlementBillID != nil {
		bill, err := s.repo.FindSettlementBill(*payload.SettlementBillID)
		if err != nil {
			return model.RecalculationTask{}, err
		}
		taskTenantID = bill.TenantID
		taskBrandID = bill.BrandID
		if payload.AgentID == nil {
			agentID := bill.AgentID
			payload.AgentID = &agentID
		} else if *payload.AgentID != bill.AgentID {
			return model.RecalculationTask{}, errors.New("agentID does not match settlement bill")
		}
		if periodStart == nil {
			value := bill.PeriodStart.UTC()
			periodStart = &value
		}
		if periodEnd == nil {
			value := bill.PeriodEnd.UTC()
			periodEnd = &value
		}
	}
	if taskType != model.RecalculationTaskTypeSettlementBill && taskType != model.RecalculationTaskTypeCommission {
		return model.RecalculationTask{}, fmt.Errorf("taskType %s is not supported", taskType)
	}
	if payload.AgentID == nil {
		return model.RecalculationTask{}, errors.New("agentID is required")
	}
	if taskTenantID == nil && taskBrandID == nil {
		agent, err := s.loadAgent(*payload.AgentID)
		if err != nil {
			return model.RecalculationTask{}, err
		}
		taskTenantID = agent.TenantID
		taskBrandID = agent.BrandID
	}
	if periodStart == nil || periodEnd == nil {
		return model.RecalculationTask{}, errors.New("periodStart and periodEnd are required")
	}
	if !periodEnd.After(*periodStart) {
		return model.RecalculationTask{}, errors.New("periodEnd must be after periodStart")
	}
	now := time.Now().UTC()
	remarkParts := make([]string, 0, 4)
	if operator := strings.TrimSpace(payload.Operator); operator != "" {
		remarkParts = append(remarkParts, "operator="+operator)
	}
	if reason := strings.TrimSpace(payload.Reason); reason != "" {
		remarkParts = append(remarkParts, "reason="+reason)
	}
	if scopeValue := strings.TrimSpace(payload.Scope); scopeValue != "" {
		remarkParts = append(remarkParts, "scope="+scopeValue)
	}
	if remark := strings.TrimSpace(payload.Remark); remark != "" {
		remarkParts = append(remarkParts, remark)
	}
	task := model.RecalculationTask{
		TenantID:         taskTenantID,
		BrandID:          taskBrandID,
		TaskNo:           fmt.Sprintf("RT-%d", now.UnixNano()),
		TaskType:         taskType,
		Status:           model.RecalculationTaskStatusProcessing,
		AgentID:          payload.AgentID,
		SettlementBillID: payload.SettlementBillID,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		RequestedBy:      sharedsvc.FirstNonEmpty(strings.TrimSpace(payload.Operator), strings.TrimSpace(requestedBy), "system"),
		StartedAt:        &now,
		Remark:           strings.Join(remarkParts, "; "),
	}
	if err := s.repo.CreateTask(&task); err != nil {
		return model.RecalculationTask{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventRecalculationTaskCreated,
		AggregateType:  "recalculation_task",
		AggregateID:    strconv.FormatUint(task.ID, 10),
		TenantID:       task.TenantID,
		BrandID:        task.BrandID,
		OccurredAt:     task.CreatedAt,
		Producer:       "recalculation-service",
		IdempotencyKey: "recalculation.task.created:" + task.TaskNo,
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        task,
	}); err != nil {
		return model.RecalculationTask{}, err
	}
	runSummary, runErr := s.run(ctx, scope, task, payload, *periodStart, *periodEnd)
	completedAt := time.Now().UTC()
	updates := map[string]any{"completed_at": completedAt, "updated_at": completedAt}
	if runErr != nil {
		updates["status"] = model.RecalculationTaskStatusFailed
		updates["error_message"] = runErr.Error()
		updates["result_summary"] = sharedsvc.MustJSONBytes(map[string]any{"result": "failed", "taskType": taskType})
		if err := s.repo.UpdateTaskResult(task.ID, updates); err != nil {
			return model.RecalculationTask{}, err
		}
		task.Status = model.RecalculationTaskStatusFailed
		task.ErrorMessage = runErr.Error()
		task.CompletedAt = &completedAt
		if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
			EventType:      eventbus.EventRecalculationTaskFailed,
			AggregateType:  "recalculation_task",
			AggregateID:    strconv.FormatUint(task.ID, 10),
			TenantID:       task.TenantID,
			BrandID:        task.BrandID,
			OccurredAt:     completedAt,
			Producer:       "recalculation-service",
			IdempotencyKey: "recalculation.task.failed:" + task.TaskNo,
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        recalculationTaskPayload(task, map[string]any{"result": "failed", "taskType": taskType}),
		}); err != nil {
			return model.RecalculationTask{}, err
		}
		return model.RecalculationTask{}, runErr
	}
	updates["status"] = model.RecalculationTaskStatusCompleted
	updates["error_message"] = ""
	updates["result_summary"] = sharedsvc.MustJSONBytes(runSummary)
	if err := s.repo.UpdateTaskResult(task.ID, updates); err != nil {
		return model.RecalculationTask{}, err
	}
	task, err = s.repo.ReloadTask(task.ID)
	if err != nil {
		return model.RecalculationTask{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventRecalculationTaskCompleted,
		AggregateType:  "recalculation_task",
		AggregateID:    strconv.FormatUint(task.ID, 10),
		TenantID:       task.TenantID,
		BrandID:        task.BrandID,
		OccurredAt:     completedAt,
		Producer:       "recalculation-service",
		IdempotencyKey: "recalculation.task.completed:" + task.TaskNo,
		Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
		Payload:        recalculationTaskPayload(task, runSummary),
	}); err != nil {
		return model.RecalculationTask{}, err
	}
	return task, nil
}

func (s *Service) run(ctx context.Context, scope sharedsvc.Scope, task model.RecalculationTask, payload CreateInput, periodStart, periodEnd time.Time) (map[string]any, error) {
	switch task.TaskType {
	case model.RecalculationTaskTypeSettlementBill:
		runPayload := settlementcontract.CreateBillInput{
			AgentID:     *payload.AgentID,
			PeriodStart: periodStart.UTC().Format(time.RFC3339),
			PeriodEnd:   periodEnd.UTC().Format(time.RFC3339),
			Currency:    "CNY",
			Adjustment:  0,
			Remark:      strings.TrimSpace(payload.Remark),
		}
		if payload.SettlementBillID != nil {
			var bill model.SettlementBill
			if err := s.db.Select("currency").First(&bill, *payload.SettlementBillID).Error; err != nil {
				return nil, err
			}
			runPayload.Currency = sharedsvc.FirstNonEmpty(strings.TrimSpace(bill.Currency), runPayload.Currency)
		}
		generatedBillID, detailCount, commissionAmount, err := s.settlementClient.GenerateBillWithSourceTask(ctx, scope, runPayload, &task.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"result":                    "completed",
			"taskType":                  task.TaskType,
			"generatedSettlementBillID": generatedBillID,
			"detailCount":               detailCount,
			"commissionAmount":          commissionAmount,
		}, nil
	case model.RecalculationTaskTypeCommission:
		recalculated, createdRecords, impactedAccounts, err := s.recalculateCommissionRecords(*payload.AgentID, periodStart, periodEnd, strings.TrimSpace(payload.Remark), &task.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"result":               "completed",
			"taskType":             task.TaskType,
			"recalculatedCount":    recalculated,
			"createdRecordCount":   createdRecords,
			"impactedAccountCount": impactedAccounts,
			"periodStart":          periodStart.UTC().Format(time.RFC3339),
			"periodEnd":            periodEnd.UTC().Format(time.RFC3339),
		}, nil
	default:
		return nil, fmt.Errorf("taskType %s is not supported", task.TaskType)
	}
}

func (s *Service) recalculateCommissionRecords(agentID uint64, periodStart, periodEnd time.Time, remark string, taskID *uint64) (int, int, int, error) {
	recalculated := 0
	createdRecords := 0
	impactedAccounts := map[uint64]struct{}{}
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		orders, err := s.repo.ListRechargeOrdersForRecalculationTx(tx, agentID, periodStart, periodEnd)
		if err != nil {
			return err
		}
		for _, order := range orders {
			if order.PaidAt == nil || order.BindingID == nil || *order.BindingID == 0 {
				continue
			}
			binding, err := s.repo.FindBindingTx(tx, *order.BindingID)
			if err != nil {
				return err
			}
			ancestors, err := s.repo.ListActiveRelationsByDescendantTx(tx, binding.AgentID)
			if err != nil {
				return err
			}
			existing, err := s.repo.ListExistingCommissionRecordsTx(tx, order.ID)
			if err != nil {
				return err
			}
			for _, record := range existing {
				if record.Status == model.CommissionStatusReversed || record.AgentID == 0 {
					continue
				}
				account, err := sharedsvc.EnsureAgentAccount(tx, record.AgentID, record.Currency)
				if err != nil {
					return err
				}
				before := account.Balance
				after := sharedsvc.Round2(before - record.CommissionAmount)
				withdrawable := sharedsvc.Round2(account.WithdrawableAmount - record.CommissionAmount)
				if err := s.repo.UpdateAgentAccountForReversalTx(tx, account.ID, after, withdrawable); err != nil {
					return err
				}
				ledgerRemark := strings.TrimSpace(sharedsvc.FirstNonEmpty(remark, "commission recalculation reversal"))
				ledger := model.AgentAccountLedger{
					TenantID:       record.TenantID,
					BrandID:        record.BrandID,
					AccountID:      account.ID,
					AgentID:        record.AgentID,
					ReferenceType:  "recalculation_task",
					ReferenceID:    strconv.FormatUint(*taskID, 10),
					LedgerType:     model.LedgerTypeReverse,
					Direction:      model.LedgerDirectionDebit,
					Amount:         record.CommissionAmount,
					BalanceBefore:  before,
					BalanceAfter:   after,
					FrozenBefore:   account.FrozenBalance,
					FrozenAfter:    account.FrozenBalance,
					Currency:       sharedsvc.DefaultCurrency(record.Currency),
					OccurredAt:     time.Now().UTC(),
					IdempotencyKey: sharedsvc.BuildReferenceNo("RC-REV", strconv.FormatUint(record.ID, 10), 0),
					Remark:         ledgerRemark,
				}
				if err := s.repo.CreateLedgerTx(tx, &ledger); err != nil {
					return err
				}
				now := time.Now().UTC()
				reversalMeta := sharedsvc.MustJSONBytes(map[string]any{
					"source":              "commission_recalculation",
					"recalculationTaskID": taskID,
					"reversedRecordID":    record.ID,
				})
				reversal := model.CommissionRecord{
					TenantID:             record.TenantID,
					BrandID:              record.BrandID,
					RecordNo:             sharedsvc.BuildReferenceNo("COM-RC", strconv.FormatUint(order.ID, 10), recalculated+1),
					RechargeOrderID:      record.RechargeOrderID,
					PlayerID:             record.PlayerID,
					AgentID:              record.AgentID,
					GameID:               record.GameID,
					RuleID:               record.RuleID,
					RuleSnapshotID:       record.RuleSnapshotID,
					SettlementDepth:      record.SettlementDepth,
					CommissionBaseAmount: record.CommissionBaseAmount,
					CommissionRate:       record.CommissionRate,
					CommissionAmount:     -record.CommissionAmount,
					Currency:             sharedsvc.DefaultCurrency(record.Currency),
					Status:               model.CommissionStatusReversed,
					EstimatedAt:          now,
					SettledAt:            &now,
					ReversedFromID:       &record.ID,
					Meta:                 reversalMeta,
					Remark:               ledgerRemark,
				}
				if err := s.repo.CreateCommissionRecordTx(tx, &reversal); err != nil {
					return err
				}
				if err := s.repo.UpdateCommissionRecordAfterRecalcTx(tx, record.ID, &now, strings.TrimSpace(sharedsvc.FirstNonEmpty(record.Remark+"; recalculated", "recalculated"))); err != nil {
					return err
				}
				account.Balance = after
				account.WithdrawableAmount = withdrawable
				impactedAccounts[record.AgentID] = struct{}{}
				recalculated++
			}
			commissions, _, err := s.settleRechargeOrder(tx, order, binding, ancestors, *order.PaidAt)
			if err != nil {
				return err
			}
			createdRecords += commissions
			for _, relation := range ancestors {
				impactedAccounts[relation.AncestorAgentID] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, 0, err
	}
	return recalculated, createdRecords, len(impactedAccounts), nil
}

func (s *Service) settleRechargeOrder(tx *gorm.DB, order model.RechargeOrder, binding model.Binding, ancestors []model.Relation, occurredAt time.Time) (int, int, error) {
	if len(ancestors) == 0 {
		ancestors = []model.Relation{{AncestorAgentID: binding.AgentID, DescendantAgentID: binding.AgentID, Depth: 1, Status: model.RelationStatusActive}}
	}
	boundAgent, err := s.loadAgentTx(tx, binding.AgentID)
	if err != nil {
		return 0, 0, err
	}
	baseSnapshot, err := s.findApplicableRuleSnapshotForContext(tx, binding.AgentID, order.GameID, occurredAt, ruleMatchContext{
		AgentLevel:   boundAgent.Level,
		RechargeType: order.RechargeType,
		ActivityTags: sharedsvc.ParseActivityTagsJSON(order.ActivityTags),
	})
	if err != nil {
		return 0, 0, err
	}
	commissions := 0
	ledgers := 0
	childRate := 0.0
	for _, relation := range ancestors {
		if relation.AncestorAgentID == 0 {
			continue
		}
		agent, err := s.loadAgentTx(tx, relation.AncestorAgentID)
		if err != nil {
			return commissions, ledgers, err
		}
		snapshot, err := s.findApplicableRuleSnapshotForContext(tx, relation.AncestorAgentID, order.GameID, occurredAt, ruleMatchContext{
			AgentLevel:   agent.Level,
			RechargeType: order.RechargeType,
			ActivityTags: sharedsvc.ParseActivityTagsJSON(order.ActivityTags),
		})
		if err != nil {
			return commissions, ledgers, err
		}
		if snapshot == nil {
			snapshot = baseSnapshot
		}
		if snapshot == nil {
			continue
		}
		var rule model.CommissionRule
		if err := json.Unmarshal(snapshot.SnapshotPayload, &rule); err != nil {
			return commissions, ledgers, err
		}
		if limit := int(sharedsvc.DefaultRuleDepth(rule.MaxSettlementDepth)); commissions >= limit {
			break
		}
		config := sharedsvc.NormalizeRuleEvaluationConfig(rule)
		paidAmount := order.PaidAmount
		if paidAmount <= 0 {
			paidAmount = order.Amount
		}
		baseAmount := sharedsvc.SettlementBaseAmount(config, paidAmount)
		amount := sharedsvc.Round2(sharedsvc.ComputeCommissionAmount(config, paidAmount, childRate))
		if amount <= 0 {
			childRate = config.CommissionRate
			continue
		}
		relationPayload, _ := json.Marshal(relation)
		record := model.CommissionRecord{
			TenantID:             order.TenantID,
			BrandID:              order.BrandID,
			RecordNo:             sharedsvc.BuildReferenceNo("COM", order.OrderNo, commissions+1),
			RechargeOrderID:      order.ID,
			PlayerID:             order.PlayerID,
			AgentID:              relation.AncestorAgentID,
			SourceAgentID:        &binding.AgentID,
			GameID:               order.GameID,
			RuleID:               sharedsvc.ScopeUint64Ptr(rule.ID),
			RuleSnapshotID:       sharedsvc.ScopeUint64Ptr(snapshot.ID),
			SettlementDepth:      uint32(commissions + 1),
			CommissionBaseAmount: baseAmount,
			SettlementRate:       config.SettlementRate,
			CommissionRate:       config.CommissionRate,
			CommissionAmount:     amount,
			Currency:             order.Currency,
			Status:               model.CommissionStatusSettled,
			EstimatedAt:          occurredAt,
			SettledAt:            &occurredAt,
			RelationSnapshot:     datatypes.JSON(relationPayload),
			Remark:               "recharge settlement",
		}
		if err := tx.Create(&record).Error; err != nil {
			return commissions, ledgers, err
		}
		account, err := sharedsvc.EnsureAgentAccount(tx, relation.AncestorAgentID, order.Currency)
		if err != nil {
			return commissions, ledgers, err
		}
		before := account.Balance
		after := sharedsvc.Round2(before + amount)
		withdrawable := sharedsvc.Round2(account.WithdrawableAmount + amount)
		if err := tx.Model(&model.AgentAccount{}).Where("id = ?", account.ID).Updates(map[string]any{
			"balance":             after,
			"available_balance":   after,
			"withdrawable_amount": withdrawable,
			"total_income":        gorm.Expr("total_income + ?", amount),
			"last_settled_at":     occurredAt,
			"version":             gorm.Expr("version + 1"),
		}).Error; err != nil {
			return commissions, ledgers, err
		}
		ledger := model.AgentAccountLedger{
			TenantID:       order.TenantID,
			BrandID:        order.BrandID,
			AccountID:      account.ID,
			AgentID:        relation.AncestorAgentID,
			ReferenceType:  "commission_record",
			ReferenceID:    strconv.FormatUint(record.ID, 10),
			LedgerType:     model.LedgerTypeIncome,
			Direction:      model.LedgerDirectionCredit,
			Amount:         amount,
			BalanceBefore:  before,
			BalanceAfter:   after,
			FrozenBefore:   account.FrozenBalance,
			FrozenAfter:    account.FrozenBalance,
			Currency:       order.Currency,
			OccurredAt:     occurredAt,
			IdempotencyKey: sharedsvc.BuildReferenceNo("LEDGER", order.OrderNo, commissions+1),
			Remark:         "recharge settlement income",
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return commissions, ledgers, err
		}
		if commissions == 0 {
			order.RuleSnapshotID = sharedsvc.ScopeUint64Ptr(snapshot.ID)
		}
		account.Balance = after
		account.WithdrawableAmount = withdrawable
		childRate = config.CommissionRate
		commissions++
		ledgers++
	}
	return commissions, ledgers, nil
}

type ruleEvaluationConfig = sharedsvc.RuleEvaluationConfig
type ruleMatchContext = sharedsvc.RuleMatchContext

func (s *Service) findApplicableRuleSnapshotForContext(tx *gorm.DB, agentID, gameID uint64, at time.Time, ctx ruleMatchContext) (*model.RuleSnapshot, error) {
	return sharedsvc.FindApplicableRuleSnapshotForContext(tx, agentID, gameID, at, ctx)
}

func (s *Service) loadAgent(agentID uint64) (model.Agent, error) {
	return s.repo.FindAgent(agentID)
}

func (s *Service) loadAgentTx(tx *gorm.DB, agentID uint64) (model.Agent, error) {
	return s.repo.FindAgentTx(tx, agentID)
}

func (s *Service) ensureAgentInCurrentScope(scope sharedsvc.Scope, agentID uint64) (model.Agent, error) {
	agent, err := s.loadAgent(agentID)
	if err != nil {
		return model.Agent{}, err
	}
	if !sharedsvc.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
		return model.Agent{}, gorm.ErrRecordNotFound
	}
	if scope.AgentID == nil {
		return agent, nil
	}
	ids, err := s.currentAgentScopeIDs(*scope.AgentID)
	if err != nil {
		return model.Agent{}, err
	}
	if !slices.Contains(ids, agent.ID) {
		return model.Agent{}, gorm.ErrRecordNotFound
	}
	return agent, nil
}

func (s *Service) currentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return s.repo.CurrentAgentScopeIDs(agentID)
}

func (s *Service) ensureAgentExists(agentID uint64) error {
	count, err := s.repo.CountAgent(agentID)
	if err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) loadAgentName(agentID uint64) string {
	return s.repo.LoadAgentName(agentID)
}

func (s *Service) loadSettlementBillNo(billID uint64) string {
	return s.repo.LoadSettlementBillNo(billID)
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
