package http

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

type ruleEvaluationConfig struct {
	RuleType       model.RuleType `json:"ruleType"`
	SettlementRate float64        `json:"settlementRate"`
	CommissionRate float64        `json:"commissionRate"`
	FixedAmount    float64        `json:"fixedAmount"`
	MinAgentLevel  uint32         `json:"minAgentLevel"`
	RechargeTypes  []string       `json:"rechargeTypes"`
	ActivityTags   []string       `json:"activityTags"`
	CapAmount      float64        `json:"capAmount"`
}

type ruleMatchContext struct {
	AgentLevel    uint32
	RechargeType  string
	ActivityTags  []string
	ChildRateHint float64
}

func normalizeRuleEvaluationConfig(rule model.CommissionRule) ruleEvaluationConfig {
	config := ruleEvaluationConfig{
		RuleType:       defaultRuleType(rule.RuleType),
		SettlementRate: defaultSettlementRate(rule.SettlementRate),
		CommissionRate: rule.CommissionRate,
		FixedAmount:    rule.FixedAmount,
	}
	if len(rule.ConfigPayload) == 0 {
		config.RechargeTypes = nil
		config.ActivityTags = nil
		return config
	}
	var raw ruleEvaluationConfig
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
		config.RechargeTypes = uniqueStringsForRuleTest(raw.RechargeTypes)
		config.ActivityTags = uniqueStringsForRuleTest(raw.ActivityTags)
		config.CapAmount = raw.CapAmount
	}
	return config
}

func matchesRuleEvaluationConfig(config ruleEvaluationConfig, ctx ruleMatchContext) bool {
	if config.MinAgentLevel > 0 && ctx.AgentLevel < config.MinAgentLevel {
		return false
	}
	if len(config.RechargeTypes) > 0 {
		rechargeType := strings.TrimSpace(ctx.RechargeType)
		if rechargeType == "" || !slices.Contains(uniqueStringsForRuleTest(config.RechargeTypes), rechargeType) {
			return false
		}
	}
	if len(config.ActivityTags) > 0 && !hasTagIntersectionForRuleTest(config.ActivityTags, ctx.ActivityTags) {
		return false
	}
	return true
}

func hasTagIntersectionForRuleTest(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, item := range uniqueStringsForRuleTest(left) {
		set[item] = struct{}{}
	}
	for _, item := range uniqueStringsForRuleTest(right) {
		if _, ok := set[item]; ok {
			return true
		}
	}
	return false
}

func uniqueStringsForRuleTest(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func FindApplicableRuleSnapshotForTest(tx *gorm.DB, agentID, gameID uint64, at time.Time) (*model.RuleSnapshot, error) {
	return findApplicableRuleSnapshotForTest(tx, agentID, gameID, at, ruleMatchContext{})
}

func findApplicableRuleSnapshotForTest(tx *gorm.DB, agentID, gameID uint64, at time.Time, ctx ruleMatchContext) (*model.RuleSnapshot, error) {
	var snapshots []model.RuleSnapshot
	if err := tx.Where("published_at <= ?", at).Find(&snapshots).Error; err != nil {
		return nil, err
	}

	var selected *model.RuleSnapshot
	for i := range snapshots {
		snapshot := snapshots[i]
		if snapshot.EffectiveFrom.After(at) {
			continue
		}
		if snapshot.EffectiveTo != nil && !snapshot.EffectiveTo.After(at) {
			continue
		}
		if !matchesRuleSnapshotForTest(snapshot, agentID, gameID) {
			continue
		}
		var rule model.CommissionRule
		if err := json.Unmarshal(snapshot.SnapshotPayload, &rule); err != nil {
			continue
		}
		if !matchesRuleEvaluationConfig(normalizeRuleEvaluationConfig(rule), ctx) {
			continue
		}
		if selected == nil || compareRuleSnapshotPriorityForTest(snapshot, *selected) > 0 {
			current := snapshot
			selected = &current
		}
	}
	return selected, nil
}

func compareRuleSnapshotPriorityForTest(left, right model.RuleSnapshot) int {
	leftScopeRank := ruleSnapshotScopeRankForTest(left.Scope)
	rightScopeRank := ruleSnapshotScopeRankForTest(right.Scope)
	if leftScopeRank != rightScopeRank {
		if leftScopeRank > rightScopeRank {
			return 1
		}
		return -1
	}

	leftPriority, rightPriority := extractRuleSnapshotPriorityForTest(left), extractRuleSnapshotPriorityForTest(right)
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

func extractRuleSnapshotPriorityForTest(snapshot model.RuleSnapshot) int32 {
	var rule model.CommissionRule
	if err := json.Unmarshal(snapshot.SnapshotPayload, &rule); err == nil {
		return rule.Priority
	}
	return 0
}

func ruleSnapshotScopeRankForTest(scope model.RuleScope) int {
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

func matchesRuleSnapshotForTest(snapshot model.RuleSnapshot, agentID, gameID uint64) bool {
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
