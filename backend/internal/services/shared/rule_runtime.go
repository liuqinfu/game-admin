package shared

import (
	"encoding/json"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type RuleEvaluationConfig struct {
	RuleType       model.RuleType `json:"ruleType"`
	SettlementRate float64        `json:"settlementRate"`
	CommissionRate float64        `json:"commissionRate"`
	FixedAmount    float64        `json:"fixedAmount"`
	MinAgentLevel  uint32         `json:"minAgentLevel"`
	RechargeTypes  []string       `json:"rechargeTypes"`
	ActivityTags   []string       `json:"activityTags"`
	CapAmount      float64        `json:"capAmount"`
}

type RuleMatchContext struct {
	AgentLevel   uint32
	RechargeType string
	ActivityTags []string
}

func NormalizeRuleEvaluationConfig(rule model.CommissionRule) RuleEvaluationConfig {
	config := RuleEvaluationConfig{
		RuleType:       defaultRuleType(rule.RuleType),
		SettlementRate: defaultSettlementRate(rule.SettlementRate),
		CommissionRate: rule.CommissionRate,
		FixedAmount:    rule.FixedAmount,
	}
	if len(rule.ConfigPayload) == 0 {
		return config
	}
	var raw RuleEvaluationConfig
	if err := json.Unmarshal(rule.ConfigPayload, &raw); err == nil {
		if raw.RuleType != "" {
			config.RuleType = defaultRuleType(raw.RuleType)
		}
		if raw.CommissionRate != 0 {
			config.CommissionRate = raw.CommissionRate
		}
		if raw.SettlementRate != 0 {
			config.SettlementRate = defaultSettlementRate(raw.SettlementRate)
		}
		if raw.FixedAmount != 0 {
			config.FixedAmount = raw.FixedAmount
		}
		config.MinAgentLevel = raw.MinAgentLevel
		config.RechargeTypes = UniqueStrings(raw.RechargeTypes)
		config.ActivityTags = UniqueStrings(raw.ActivityTags)
		config.CapAmount = raw.CapAmount
	}
	return config
}

func ParseActivityTagsJSON(value datatypes.JSON) []string {
	if len(value) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(value, &tags); err != nil {
		return nil
	}
	return UniqueStrings(tags)
}

func ActivityTagsJSON(tags []string) datatypes.JSON {
	normalized := UniqueStrings(tags)
	if len(normalized) == 0 {
		return nil
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}
	return datatypes.JSON(payload)
}

func MatchesRuleEvaluationConfig(config RuleEvaluationConfig, ctx RuleMatchContext) bool {
	if config.MinAgentLevel > 0 && ctx.AgentLevel < config.MinAgentLevel {
		return false
	}
	if len(config.RechargeTypes) > 0 {
		rechargeType := strings.TrimSpace(ctx.RechargeType)
		if rechargeType == "" {
			return false
		}
		allowed := UniqueStrings(config.RechargeTypes)
		matched := false
		for _, item := range allowed {
			if item == rechargeType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(config.ActivityTags) > 0 && !hasTagIntersection(config.ActivityTags, ctx.ActivityTags) {
		return false
	}
	return true
}

func SettlementBaseAmount(config RuleEvaluationConfig, paidAmount float64) float64 {
	return Round2(paidAmount * defaultSettlementRate(config.SettlementRate))
}

func ComputeCommissionAmount(config RuleEvaluationConfig, paidAmount, childRate float64) float64 {
	baseAmount := SettlementBaseAmount(config, paidAmount)
	switch defaultRuleType(config.RuleType) {
	case model.RuleTypeFixedShare:
		return config.FixedAmount
	case model.RuleTypeDifferential:
		return Round2(maxFloat(config.CommissionRate-childRate, 0) * baseAmount)
	case model.RuleTypePoint:
		return config.FixedAmount
	case model.RuleTypeCapped:
		base := config.FixedAmount
		if config.CommissionRate > 0 {
			base = baseAmount * config.CommissionRate
		}
		if config.CapAmount > 0 && base > config.CapAmount {
			base = config.CapAmount
		}
		return base
	default:
		return baseAmount * config.CommissionRate
	}
}

func FindApplicableRuleSnapshotForContext(tx *gorm.DB, agentID, gameID uint64, at time.Time, ctx RuleMatchContext) (*model.RuleSnapshot, error) {
	var snapshots []model.RuleSnapshot
	if err := tx.Where("published_at <= ?", at).Find(&snapshots).Error; err != nil {
		return nil, err
	}
	var selected *model.RuleSnapshot
	for i := range snapshots {
		snapshot := snapshots[i]
		if snapshot.EffectiveFrom.After(at) || (snapshot.EffectiveTo != nil && !snapshot.EffectiveTo.After(at)) || !matchesRuleSnapshot(snapshot, agentID, gameID) {
			continue
		}
		var rule model.CommissionRule
		if err := json.Unmarshal(snapshot.SnapshotPayload, &rule); err != nil {
			continue
		}
		if !MatchesRuleEvaluationConfig(NormalizeRuleEvaluationConfig(rule), ctx) {
			continue
		}
		if selected == nil || compareRuleSnapshotPriority(snapshot, *selected) > 0 {
			current := snapshot
			selected = &current
		}
	}
	return selected, nil
}

func defaultRuleType(ruleType model.RuleType) model.RuleType {
	switch ruleType {
	case "", model.RuleTypeRatio:
		return model.RuleTypeRatio
	case model.RuleTypeFixedShare:
		return model.RuleTypeFixedShare
	case model.RuleTypeDifferential:
		return model.RuleTypeDifferential
	case model.RuleTypePoint:
		return model.RuleTypePoint
	case model.RuleTypeCapped:
		return model.RuleTypeCapped
	default:
		return model.RuleTypeRatio
	}
}

func defaultSettlementRate(rate float64) float64 {
	if rate <= 0 {
		return 1
	}
	return rate
}

func hasTagIntersection(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, item := range UniqueStrings(left) {
		set[item] = struct{}{}
	}
	for _, item := range UniqueStrings(right) {
		if _, ok := set[item]; ok {
			return true
		}
	}
	return false
}

func compareRuleSnapshotPriority(left, right model.RuleSnapshot) int {
	leftScopeRank := ruleSnapshotScopeRank(left.Scope)
	rightScopeRank := ruleSnapshotScopeRank(right.Scope)
	if leftScopeRank != rightScopeRank {
		if leftScopeRank > rightScopeRank {
			return 1
		}
		return -1
	}
	leftPriority, rightPriority := extractRuleSnapshotPriority(left), extractRuleSnapshotPriority(right)
	if leftPriority != rightPriority {
		if leftPriority > rightPriority {
			return 1
		}
		return -1
	}
	if !left.EffectiveFrom.Equal(right.EffectiveFrom) {
		if left.EffectiveFrom.After(right.EffectiveFrom) {
			return 1
		}
		return -1
	}
	if left.RuleVersion != right.RuleVersion {
		if left.RuleVersion > right.RuleVersion {
			return 1
		}
		return -1
	}
	if !left.PublishedAt.Equal(right.PublishedAt) {
		if left.PublishedAt.After(right.PublishedAt) {
			return 1
		}
		return -1
	}
	if left.ID > right.ID {
		return 1
	}
	if left.ID < right.ID {
		return -1
	}
	return 0
}

func extractRuleSnapshotPriority(snapshot model.RuleSnapshot) int32 {
	var rule model.CommissionRule
	if err := json.Unmarshal(snapshot.SnapshotPayload, &rule); err == nil {
		return rule.Priority
	}
	return 0
}

func ruleSnapshotScopeRank(scope model.RuleScope) int {
	switch scope {
	case model.RuleScopeAgentGame:
		return 4
	case model.RuleScopeAgent:
		return 3
	case model.RuleScopeGame:
		return 2
	case model.RuleScopePlatform:
		return 1
	default:
		return 0
	}
}

func matchesRuleSnapshot(snapshot model.RuleSnapshot, agentID, gameID uint64) bool {
	switch snapshot.Scope {
	case model.RuleScopeAgentGame:
		return snapshot.AgentID != nil && snapshot.GameID != nil && *snapshot.AgentID == agentID && *snapshot.GameID == gameID
	case model.RuleScopeAgent:
		return snapshot.AgentID != nil && *snapshot.AgentID == agentID
	case model.RuleScopeGame:
		return snapshot.GameID != nil && *snapshot.GameID == gameID
	case model.RuleScopePlatform:
		return true
	default:
		return false
	}
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
