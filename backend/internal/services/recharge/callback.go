package recharge

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CallbackInput struct {
	OrderNo            string
	ExternalOrderNo    string
	PlayerID           uint64
	GameID             uint64
	Amount             float64
	PaidAmount         float64
	PaymentChannelCost *float64
	GrossProfitAmount  *float64
	Currency           string
	Status             string
	PaidAt             string
	CallbackAt         string
	Channel            string
	RechargeType       string
	RequestID          string
	CallbackSource     string
	Signature          string
	IdempotencyKey     string
	ActivityTags       []string
	Remark             string
}

type CallbackResult struct {
	Order       model.RechargeOrder
	Commissions int
	Ledgers     int
	Duplicate   bool
	Frozen      bool
}

type ruleEvaluationConfig = sharedsvc.RuleEvaluationConfig
type ruleMatchContext = sharedsvc.RuleMatchContext

type enumDictionaryItem = sharedsvc.EnumDictionaryItem
type enumDictionaryResponse = sharedsvc.EnumDictionaryResponse

const (
	enumDictionaryRechargeTypeCode = sharedsvc.EnumDictionaryRechargeTypeCode
	enumDictionaryActivityTagCode  = sharedsvc.EnumDictionaryActivityTagCode
)

func (s *Service) ProcessCallback(input CallbackInput, remoteIP string) (*CallbackResult, error) {
	if strings.TrimSpace(input.OrderNo) == "" {
		return nil, errors.New("orderNo is required")
	}
	if input.PlayerID == 0 {
		return nil, errors.New("playerID is required")
	}
	if input.GameID == 0 {
		return nil, errors.New("gameID is required")
	}
	if input.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}
	status := model.OrderStatus(strings.TrimSpace(input.Status))
	if status == "" {
		status = model.OrderStatusPaid
	}
	if status != model.OrderStatusPaid && status != model.OrderStatusRefunded {
		return nil, errors.New("only paid or refunded recharge callbacks are supported")
	}
	if err := s.validateRechargeCallbackDictionaryValues(input); err != nil {
		return nil, err
	}
	paidAt, err := sharedsvc.ParseOptionalTime(input.PaidAt)
	if err != nil {
		return nil, errors.New("invalid paidAt")
	}
	callbackAt, err := sharedsvc.ParseOptionalTime(input.CallbackAt)
	if err != nil {
		return nil, errors.New("invalid callbackAt")
	}
	if paidAt == nil {
		now := time.Now().UTC()
		paidAt = &now
	}
	if callbackAt == nil {
		now := time.Now().UTC()
		callbackAt = &now
	}
	requestPayload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	result := &CallbackResult{}
	err = s.repo.WithTx(func(tx *gorm.DB) error {
		player, err := s.repo.FindPlayerTx(tx, input.PlayerID)
		if err != nil {
			return err
		}
		game, err := s.repo.FindGameTx(tx, input.GameID)
		if err != nil {
			return err
		}
		if input.IdempotencyKey != "" {
			existing, err := s.repo.FindRechargeOrderByIdempotencyKeyTx(tx, input.IdempotencyKey)
			if err == nil {
				result.Order = existing
				result.Duplicate = true
				return nil
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		existing, err := s.repo.FindRechargeOrderByOrderNoTx(tx, input.OrderNo)
		if err == nil {
			if status == model.OrderStatusRefunded {
				callbackLog := model.RechargeCallbackLog{
					RechargeOrderID: existing.ID,
					CallbackStatus:  model.CallbackStatusVerified,
					RequestID:       input.RequestID,
					CallbackSource:  input.CallbackSource,
					Signature:       input.Signature,
					VerifiedAt:      callbackAt,
					RequestPayload:  requestPayload,
					RemoteIP:        remoteIP,
				}
				if err := s.repo.CreateRechargeCallbackLogTx(tx, &callbackLog); err != nil {
					return err
				}
				commissions, ledgers, refundedOrder, duplicateRefund, err := s.reverseRechargeOrder(tx, existing, *callbackAt)
				if err != nil {
					return err
				}
				if _, err := eventbus.Publish(tx, eventbus.PublishInput{
					EventType:      eventbus.EventRechargeOrderRefunded,
					AggregateType:  "recharge_order",
					AggregateID:    strconv.FormatUint(refundedOrder.ID, 10),
					TenantID:       refundedOrder.TenantID,
					BrandID:        refundedOrder.BrandID,
					OccurredAt:     *callbackAt,
					Producer:       "recharge-service",
					IdempotencyKey: "recharge.refunded:" + refundedOrder.OrderNo,
					Consumers:      eventbus.ConsumersDataPlatformSync(),
					Payload:        rechargeOrderPayload(refundedOrder, duplicateRefund, commissions, ledgers),
				}); err != nil {
					return err
				}
				result.Order = refundedOrder
				result.Commissions = commissions
				result.Ledgers = ledgers
				result.Duplicate = duplicateRefund
				return nil
			}
			result.Order = existing
			result.Duplicate = true
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if status == model.OrderStatusRefunded {
			return errors.New("recharge order not found for refund callback")
		}
		if err := sharedsvc.EnsureSameScope("game", player.TenantID, player.BrandID, game.TenantID, game.BrandID); err != nil {
			return err
		}
		binding, ancestors, err := s.resolveRechargeSettlementContext(tx, input.PlayerID)
		if err != nil {
			return err
		}
		if binding != nil {
			if err := sharedsvc.EnsureSameScope("binding", player.TenantID, player.BrandID, binding.TenantID, binding.BrandID); err != nil {
				return err
			}
			if err := s.validateAgentGameSettlementAccess(tx, *binding, game, *paidAt); err != nil {
				return err
			}
		}
		order := model.RechargeOrder{
			TenantID:        player.TenantID,
			BrandID:         player.BrandID,
			OrderNo:         input.OrderNo,
			ExternalOrderNo: input.ExternalOrderNo,
			PlayerID:        input.PlayerID,
			GameID:          input.GameID,
			Amount:          input.Amount,
			PaidAmount:      input.Amount,
			Currency:        sharedsvc.DefaultCurrency(sharedsvc.FirstNonEmpty(input.Currency, player.Currency, game.Currency)),
			Status:          model.OrderStatusPaid,
			PaidAt:          paidAt,
			CallbackAt:      callbackAt,
			Channel:         strings.TrimSpace(input.Channel),
			RechargeType:    strings.TrimSpace(input.RechargeType),
			ActivityTags:    sharedsvc.ActivityTagsJSON(input.ActivityTags),
			CallbackPayload: requestPayload,
			IdempotencyKey:  strings.TrimSpace(input.IdempotencyKey),
			Remark:          strings.TrimSpace(input.Remark),
		}
		if input.PaidAmount > 0 {
			order.PaidAmount = sharedsvc.Round2(input.PaidAmount)
		}
		order.PaymentChannelCost = sharedsvc.NormalizeOptionalAmount(input.PaymentChannelCost)
		order.GrossProfitAmount = sharedsvc.NormalizeOptionalAmount(input.GrossProfitAmount)
		if binding != nil {
			order.AgentID = &binding.AgentID
			order.BindingID = &binding.ID
			riskFreeze, err := s.repo.FindLatestFreezeLedgerByAgentTx(tx, binding.AgentID)
			if err == nil && riskFreeze.ReferenceType == "risk_case" {
				order.RiskFlag = riskFreeze.ReferenceID
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := s.repo.CreateRechargeOrderTx(tx, &order); err != nil {
			return err
		}
		callbackLog := model.RechargeCallbackLog{
			RechargeOrderID: order.ID,
			CallbackStatus:  model.CallbackStatusVerified,
			RequestID:       input.RequestID,
			CallbackSource:  input.CallbackSource,
			Signature:       input.Signature,
			VerifiedAt:      callbackAt,
			RequestPayload:  requestPayload,
			RemoteIP:        remoteIP,
		}
		if err := s.repo.CreateRechargeCallbackLogTx(tx, &callbackLog); err != nil {
			return err
		}
		if binding != nil && strings.TrimSpace(order.RiskFlag) == "" {
			commissions, ledgers, err := s.settleRechargeOrder(tx, order, *binding, ancestors, *paidAt)
			if err != nil {
				return err
			}
			result.Commissions = commissions
			result.Ledgers = ledgers
		}
		if _, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRechargeOrderPaid,
			AggregateType:  "recharge_order",
			AggregateID:    strconv.FormatUint(order.ID, 10),
			TenantID:       order.TenantID,
			BrandID:        order.BrandID,
			OccurredAt:     *callbackAt,
			Producer:       "recharge-service",
			IdempotencyKey: "recharge.paid:" + order.OrderNo,
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        rechargeOrderPayload(order, false, result.Commissions, result.Ledgers),
		}); err != nil {
			return err
		}
		result.Frozen = strings.TrimSpace(order.RiskFlag) != ""
		result.Order = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) resolveRechargeSettlementContext(tx *gorm.DB, playerID uint64) (*model.Binding, []model.Relation, error) {
	var binding model.Binding
	binding, err := s.repo.FindBoundBindingByPlayerTx(tx, playerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	ancestors, err := s.repo.ListActiveRelationsByDescendantTx(tx, binding.AgentID)
	if err != nil {
		return nil, nil, err
	}
	if len(ancestors) == 0 {
		closures, err := s.repo.ListActiveRelationClosuresByDescendantTx(tx, binding.AgentID)
		if err != nil {
			return nil, nil, err
		}
		ancestors = make([]model.Relation, 0, len(closures))
		for _, closure := range closures {
			ancestors = append(ancestors, model.Relation{
				TenantID:          closure.TenantID,
				BrandID:           closure.BrandID,
				AncestorAgentID:   closure.AncestorAgentID,
				DescendantAgentID: closure.DescendantAgentID,
				Depth:             closure.Depth,
				DirectParentID:    closure.ViaDirectParentID,
				RelationType:      closure.RelationType,
				Status:            closure.Status,
				EffectiveFrom:     closure.EffectiveFrom,
				EffectiveTo:       closure.EffectiveTo,
			})
		}
	}
	return &binding, ancestors, nil
}

func (s *Service) settleRechargeOrder(tx *gorm.DB, order model.RechargeOrder, binding model.Binding, ancestors []model.Relation, occurredAt time.Time) (int, int, error) {
	if len(ancestors) == 0 {
		ancestors = []model.Relation{{AncestorAgentID: binding.AgentID, DescendantAgentID: binding.AgentID, Depth: 1, Status: model.RelationStatusActive}}
	}
	boundAgent, err := s.loadAgent(tx, binding.AgentID)
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
		agent, err := s.loadAgent(tx, relation.AncestorAgentID)
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
		if err := s.repo.CreateCommissionRecordTx(tx, &record); err != nil {
			return commissions, ledgers, err
		}
		account, err := sharedsvc.EnsureAgentAccount(tx, relation.AncestorAgentID, order.Currency)
		if err != nil {
			return commissions, ledgers, err
		}
		before := account.Balance
		after := sharedsvc.Round2(before + amount)
		withdrawable := sharedsvc.Round2(account.WithdrawableAmount + amount)
		if err := s.repo.UpdateAgentAccountSettlementTx(tx, account.ID, after, withdrawable, occurredAt, &amount, nil); err != nil {
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
		if err := s.repo.CreateAgentAccountLedgerTx(tx, &ledger); err != nil {
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

func (s *Service) reverseRechargeOrder(tx *gorm.DB, order model.RechargeOrder, occurredAt time.Time) (int, int, model.RechargeOrder, bool, error) {
	if order.Status == model.OrderStatusRefunded {
		return 0, 0, order, true, nil
	}
	reversalCount, err := s.repo.CountRechargeReversalRecordsTx(tx, order.ID)
	if err != nil {
		return 0, 0, order, false, err
	}
	if reversalCount > 0 {
		if err := s.repo.UpdateRechargeOrderRefundedTx(tx, order.ID, occurredAt); err != nil {
			return 0, 0, order, false, err
		}
		order.Status = model.OrderStatusRefunded
		order.CallbackAt = &occurredAt
		return 0, 0, order, true, nil
	}
	records, err := s.repo.ListRechargeCommissionRecordsTx(tx, order.ID)
	if err != nil {
		return 0, 0, order, false, err
	}
	commissions := 0
	ledgers := 0
	for idx, record := range records {
		if record.CommissionAmount <= 0 || record.Status == model.CommissionStatusReversed {
			continue
		}
		reversal := model.CommissionRecord{
			TenantID:             record.TenantID,
			BrandID:              record.BrandID,
			RecordNo:             sharedsvc.BuildReferenceNo("COM-R", order.OrderNo, idx+1),
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
			Currency:             record.Currency,
			Status:               model.CommissionStatusReversed,
			EstimatedAt:          occurredAt,
			SettledAt:            &occurredAt,
			ReversedFromID:       &record.ID,
			Remark:               "recharge refund reversal",
		}
		if len(record.Meta) > 0 {
			reversal.Meta = append([]byte(nil), record.Meta...)
		}
		if err := s.repo.CreateCommissionRecordTx(tx, &reversal); err != nil {
			if sharedsvc.IsUniqueConstraintError(err) {
				return commissions, ledgers, order, true, nil
			}
			return commissions, ledgers, order, false, err
		}
		if err := s.repo.UpdateCommissionRecordReversedTx(tx, record.ID, occurredAt); err != nil {
			return commissions, ledgers, order, false, err
		}
		account, err := sharedsvc.EnsureAgentAccount(tx, record.AgentID, record.Currency)
		if err != nil {
			return commissions, ledgers, order, false, err
		}
		before := account.Balance
		after := sharedsvc.Round2(before - record.CommissionAmount)
		withdrawable := sharedsvc.Round2(account.WithdrawableAmount - record.CommissionAmount)
		if withdrawable < 0 {
			withdrawable = 0
		}
		if after < 0 {
			after = 0
		}
		if err := s.repo.UpdateAgentAccountSettlementTx(tx, account.ID, after, withdrawable, occurredAt, nil, &record.CommissionAmount); err != nil {
			return commissions, ledgers, order, false, err
		}
		ledger := model.AgentAccountLedger{
			TenantID:       record.TenantID,
			BrandID:        record.BrandID,
			AccountID:      account.ID,
			AgentID:        record.AgentID,
			ReferenceType:  "commission_record",
			ReferenceID:    strconv.FormatUint(reversal.ID, 10),
			LedgerType:     model.LedgerTypeReverse,
			Direction:      model.LedgerDirectionDebit,
			Amount:         record.CommissionAmount,
			BalanceBefore:  before,
			BalanceAfter:   after,
			FrozenBefore:   account.FrozenBalance,
			FrozenAfter:    account.FrozenBalance,
			Currency:       record.Currency,
			OccurredAt:     occurredAt,
			IdempotencyKey: sharedsvc.BuildReferenceNo("LEDGER-R", order.OrderNo, idx+1),
			Remark:         "recharge refund reversal",
		}
		if err := s.repo.CreateAgentAccountLedgerTx(tx, &ledger); err != nil {
			if sharedsvc.IsUniqueConstraintError(err) {
				return commissions, ledgers, order, true, nil
			}
			return commissions, ledgers, order, false, err
		}
		commissions++
		ledgers++
	}
	if err := s.repo.UpdateRechargeOrderRefundedTx(tx, order.ID, occurredAt); err != nil {
		return commissions, ledgers, order, false, err
	}
	order.Status = model.OrderStatusRefunded
	order.CallbackAt = &occurredAt
	return commissions, ledgers, order, false, nil
}

func (s *Service) validateAgentGameSettlementAccess(tx *gorm.DB, binding model.Binding, game model.Game, at time.Time) error {
	if !game.IsAgentable {
		return errors.New("game is not agent-accessible")
	}
	access, err := s.repo.FindAgentGameAccessTx(tx, binding.AgentID, game.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("agent does not have access to this game")
		}
		return err
	}
	if access.Status != model.AccessStatusEnabled || access.EffectiveFrom.After(at) || (access.EffectiveTo != nil && !access.EffectiveTo.After(at)) {
		return errors.New("agent does not have access to this game")
	}
	return nil
}

func (s *Service) findApplicableRuleSnapshotForContext(tx *gorm.DB, agentID, gameID uint64, at time.Time, ctx ruleMatchContext) (*model.RuleSnapshot, error) {
	return sharedsvc.FindApplicableRuleSnapshotForContext(tx, agentID, gameID, at, ctx)
}

func (s *Service) validateRechargeCallbackDictionaryValues(input CallbackInput) error {
	rechargeDictionary, err := s.resolveEnumDictionary(enumDictionaryRechargeTypeCode)
	if err != nil {
		return err
	}
	if err := sharedsvc.ValidateEnumDictionaryValue(rechargeDictionary, "rechargeType", input.RechargeType); err != nil {
		return err
	}
	activityDictionary, err := s.resolveEnumDictionary(sharedsvc.EnumDictionaryActivityTagCode)
	if err != nil {
		return err
	}
	return sharedsvc.ValidateEnumDictionaryValues(activityDictionary, "activityTags", input.ActivityTags)
}

func (s *Service) resolveEnumDictionary(code string) (enumDictionaryResponse, error) {
	dictionary, err := sharedsvc.ResolveEnumDictionary(s.db, code)
	if err != nil {
		return enumDictionaryResponse{}, err
	}
	return dictionary, nil
}

func (s *Service) loadAgent(tx *gorm.DB, agentID uint64) (model.Agent, error) {
	return s.repo.FindAgentTx(tx, agentID)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
