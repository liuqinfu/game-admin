package rule

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type ListFilter struct {
	Status  string
	Scope   string
	GameID  string
	AgentID string
}

type OpenAPIListFilter struct {
	Status  string
	Scope   string
	AgentID string
}

type CreateInput struct {
	TenantID           *uint64
	BrandID            *uint64
	RuleName           string
	Scope              model.RuleScope
	RuleType           model.RuleType
	Status             model.RuleStatus
	Priority           int32
	Version            uint32
	AgentID            *uint64
	GameID             *uint64
	MaxSettlementDepth uint32
	SettlementRate     float64
	CommissionRate     float64
	FixedAmount        float64
	MinAgentLevel      uint32
	RechargeTypes      []string
	ActivityTags       []string
	CapAmount          float64
	Currency           string
	EffectiveFrom      string
	Remark             string
}

type PublishInput struct{ PublishedBy string }

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) List(scope shared.Scope, filter ListFilter) ([]model.CommissionRule, error) {
	return s.repo.ListRules(scope, filter)
}

func (s *Service) ListForGame(game model.Game, filter OpenAPIListFilter) ([]model.CommissionRule, error) {
	return shared.ResolveCachedRuleList(game, filter, func() ([]model.CommissionRule, error) {
		return s.repo.ListRulesForGame(game, filter)
	})
}

func (s *Service) Create(scope shared.Scope, input CreateInput) (model.CommissionRule, error) {
	tenantID, brandID, err := shared.ResolveTenantBrandScope(s.db, scope, input.TenantID, input.BrandID, false)
	if err != nil {
		return model.CommissionRule{}, err
	}
	if err := s.validateDictionaryValues(input.RechargeTypes, input.ActivityTags); err != nil {
		return model.CommissionRule{}, err
	}
	effectiveFrom, err := time.Parse(time.RFC3339, input.EffectiveFrom)
	if err != nil {
		return model.CommissionRule{}, err
	}
	ruleScope := defaultRuleScope(input.Scope)
	if err := validateRuleScope(ruleScope, input.AgentID, input.GameID); err != nil {
		return model.CommissionRule{}, err
	}
	if err := s.ensureRuleReferencesInScope(scope, tenantID, brandID, input.AgentID, input.GameID); err != nil {
		return model.CommissionRule{}, err
	}
	configPayload, _ := json.Marshal(map[string]any{
		"ruleType":       input.RuleType,
		"settlementRate": input.SettlementRate,
		"commissionRate": input.CommissionRate,
		"fixedAmount":    input.FixedAmount,
		"minAgentLevel":  input.MinAgentLevel,
		"rechargeTypes":  input.RechargeTypes,
		"activityTags":   input.ActivityTags,
		"capAmount":      input.CapAmount,
	})
	rule := model.CommissionRule{
		TenantID:           tenantID,
		BrandID:            brandID,
		RuleName:           input.RuleName,
		Scope:              ruleScope,
		RuleType:           defaultRuleType(input.RuleType),
		Status:             defaultRuleStatus(input.Status),
		Priority:           input.Priority,
		Version:            defaultRuleVersion(input.Version),
		AgentID:            input.AgentID,
		GameID:             input.GameID,
		MaxSettlementDepth: shared.DefaultRuleDepth(input.MaxSettlementDepth),
		SettlementRate:     defaultSettlementRate(input.SettlementRate),
		CommissionRate:     input.CommissionRate,
		FixedAmount:        input.FixedAmount,
		Currency:           shared.DefaultCurrency(input.Currency),
		EffectiveFrom:      effectiveFrom,
		ConfigPayload:      configPayload,
		UniqueKey:          buildScopedRuleUniqueKey(tenantID, brandID, ruleScope, input.AgentID, input.GameID, input.RuleName),
		Remark:             input.Remark,
	}
	created, err := rule, s.repo.CreateRule(&rule)
	if err == nil {
		shared.BumpRuleOpenAPICache()
		shared.BumpEnumDictionaryCache()
	}
	return created, err
}

func (s *Service) Publish(scope shared.Scope, id uint64, publishedBy string) (model.CommissionRule, model.CommissionRule, error) {
	rule, err := s.repo.FindRule(id)
	if err != nil {
		return model.CommissionRule{}, model.CommissionRule{}, err
	}
	if !shared.MatchesScopedRecord(scope, rule.TenantID, rule.BrandID) {
		return model.CommissionRule{}, model.CommissionRule{}, gorm.ErrRecordNotFound
	}
	if rule.Status != model.RuleStatusDraft {
		return model.CommissionRule{}, model.CommissionRule{}, errors.New("only draft rules can be published")
	}
	now := time.Now().UTC()
	before := rule
	rule.Status = model.RuleStatusPublished
	rule.PublishedAt = &now
	rule.PublishedBy = strings.TrimSpace(publishedBy)
	err = s.repo.WithTx(func(tx *gorm.DB) error {
		if err := s.repo.DisablePublishedRulesTx(tx, rule.UniqueKey, rule.ID, rule.TenantID, rule.BrandID); err != nil {
			return err
		}
		if err := s.repo.SaveRuleTx(tx, &rule); err != nil {
			return err
		}
		snapshotPayload, err := json.Marshal(rule)
		if err != nil {
			return err
		}
		snapshot := model.RuleSnapshot{
			TenantID:        rule.TenantID,
			BrandID:         rule.BrandID,
			RuleID:          rule.ID,
			RuleVersion:     rule.Version,
			RuleUniqueKey:   rule.UniqueKey,
			Scope:           rule.Scope,
			AgentID:         rule.AgentID,
			GameID:          rule.GameID,
			SnapshotHash:    strconv.FormatInt(now.UnixNano(), 10) + "-" + rule.UniqueKey,
			SnapshotPayload: snapshotPayload,
			EffectiveFrom:   rule.EffectiveFrom,
			EffectiveTo:     rule.EffectiveTo,
			PublishedAt:     now,
			PublishedBy:     rule.PublishedBy,
		}
		return s.repo.CreateRuleSnapshotTx(tx, &snapshot)
	})
	if err == nil {
		shared.BumpRuleOpenAPICache()
	}
	return before, rule, err
}

func (s *Service) ensureRuleReferencesInScope(scope shared.Scope, tenantID, brandID *uint64, agentID, gameID *uint64) error {
	if agentID != nil {
		agent, err := s.repo.FindAgent(*agentID)
		if err != nil {
			return err
		}
		if !shared.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
			return gorm.ErrRecordNotFound
		}
		if err := shared.EnsureSameScope("agent", tenantID, brandID, agent.TenantID, agent.BrandID); err != nil {
			return err
		}
	}
	if gameID != nil {
		game, err := s.repo.FindGame(*gameID)
		if err != nil {
			return err
		}
		if !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
			return gorm.ErrRecordNotFound
		}
		if err := shared.EnsureSameScope("game", tenantID, brandID, game.TenantID, game.BrandID); err != nil {
			return err
		}
	}
	return nil
}

func defaultRuleStatus(status model.RuleStatus) model.RuleStatus {
	if status == "" {
		return model.RuleStatusDraft
	}
	return status
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
func defaultRuleScope(scope model.RuleScope) model.RuleScope {
	if scope == "" {
		return model.RuleScopePlatform
	}
	return scope
}
func defaultRuleVersion(version uint32) uint32 {
	if version == 0 {
		return 1
	}
	return version
}

func (s *Service) validateDictionaryValues(rechargeTypes, activityTags []string) error {
	rechargeDictionary, err := s.repo.ResolveEnumDictionaryCached(shared.EnumDictionaryRechargeTypeCode)
	if err != nil {
		return err
	}
	if err := shared.ValidateEnumDictionaryValues(rechargeDictionary, "rechargeTypes", rechargeTypes); err != nil {
		return err
	}
	activityDictionary, err := s.repo.ResolveEnumDictionaryCached(shared.EnumDictionaryActivityTagCode)
	if err != nil {
		return err
	}
	return shared.ValidateEnumDictionaryValues(activityDictionary, "activityTags", activityTags)
}

func uniqueStrings(values []string) []string {
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
func defaultSettlementRate(rate float64) float64 {
	if rate <= 0 {
		return 1
	}
	return rate
}
func validateRuleScope(scope model.RuleScope, agentID, gameID *uint64) error {
	switch scope {
	case model.RuleScopePlatform:
		if agentID != nil || gameID != nil {
			return errors.New("platform scope cannot bind agentID or gameID")
		}
	case model.RuleScopeGame:
		if gameID == nil {
			return errors.New("game scope requires gameID")
		}
		if agentID != nil {
			return errors.New("game scope cannot bind agentID")
		}
	case model.RuleScopeAgent:
		if agentID == nil {
			return errors.New("agent scope requires agentID")
		}
		if gameID != nil {
			return errors.New("agent scope cannot bind gameID")
		}
	case model.RuleScopeAgentGame:
		if agentID == nil || gameID == nil {
			return errors.New("agent_game scope requires both agentID and gameID")
		}
	default:
		return errors.New("unsupported scope")
	}
	return nil
}
func buildScopedRuleUniqueKey(tenantID, brandID *uint64, scope model.RuleScope, agentID, gameID *uint64, ruleName string) string {
	parts := make([]string, 0, 6)
	if tenantID != nil {
		parts = append(parts, "tenant:"+strconv.FormatUint(*tenantID, 10))
	} else {
		parts = append(parts, "tenant:platform")
	}
	if brandID != nil {
		parts = append(parts, "brand:"+strconv.FormatUint(*brandID, 10))
	} else {
		parts = append(parts, "brand:all")
	}
	parts = append(parts, string(scope))
	if agentID != nil {
		parts = append(parts, "agent:"+strconv.FormatUint(*agentID, 10))
	}
	if gameID != nil {
		parts = append(parts, "game:"+strconv.FormatUint(*gameID, 10))
	}
	if name := strings.TrimSpace(ruleName); name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, "|")
}
