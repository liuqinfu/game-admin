package http

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
)

func defaultInviteStatus(status model.InviteCodeStatus) model.InviteCodeStatus {
	if status == "" {
		return model.InviteCodeStatusActive
	}
	return status
}

func defaultGameStatus(status model.GameStatus) model.GameStatus {
	if status == "" {
		return model.GameStatusDraft
	}
	return status
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

func defaultLevel(level uint32) uint32 {
	if level == 0 {
		return 1
	}
	return level
}

func defaultRuleVersion(version uint32) uint32 {
	if version == 0 {
		return 1
	}
	return version
}

func defaultSettlementRate(rate float64) float64 {
	if rate <= 0 {
		return 1
	}
	return rate
}

func validateGameStatusTransition(current, next model.GameStatus) error {
	if next == "" {
		return errors.New("status is required")
	}
	if current == model.GameStatusOffline && next == model.GameStatusDraft {
		return errors.New("offline games cannot be moved back to draft")
	}
	if current == model.GameStatusArchived && next != model.GameStatusArchived {
		return errors.New("archived games cannot change status")
	}
	return nil
}

func applyGameStatus(game *model.Game, status model.GameStatus, now time.Time) {
	game.Status = status
	switch status {
	case model.GameStatusOnline:
		game.LaunchAt = &now
		game.OfflineAt = nil
	case model.GameStatusOffline:
		game.OfflineAt = &now
	case model.GameStatusDraft:
		game.LaunchAt = nil
		game.OfflineAt = nil
	}
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

func buildRuleUniqueKey(scope model.RuleScope, agentID, gameID *uint64, ruleName string) string {
	return buildScopedRuleUniqueKey(nil, nil, scope, agentID, gameID, ruleName)
}

var (
	errAgentInviteRestricted      = errors.New("frozen or inactive agent cannot issue invite bindings")
	errInviteCodeUnavailable      = errors.New("invite code is invalid or unavailable")
	errDuplicateBinding           = errors.New("player is already bound to an agent")
	errInviteCodePrimaryConflict  = errors.New("agent already has a primary invite code")
	errAgentInviteLoopDetected    = errors.New("agent relation loop detected")
	errInviteApplicationProcessed = errors.New("agent invite application already processed")
)
