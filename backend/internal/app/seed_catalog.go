package app

import "game-admin/backend/internal/domain/model"

type enumDictionarySeedItem struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	LabelEn     string `json:"labelEn,omitempty"`
	Description string `json:"description,omitempty"`
}

type enumDictionarySeedConfig struct {
	Strict bool                     `json:"strict"`
	Items  []enumDictionarySeedItem `json:"items"`
}

func systemPermissions() []model.AdminPermission {
	return []model.AdminPermission{
		{Code: "agent:read", Name: "Agent Read"},
		{Code: "agent:write", Name: "Agent Write"},
		{Code: "tenant:read", Name: "Tenant Read"},
		{Code: "tenant:write", Name: "Tenant Write"},
		{Code: "brand:read", Name: "Brand Read"},
		{Code: "brand:write", Name: "Brand Write"},
		{Code: "invite_code:manage", Name: "Invite Code Manage"},
		{Code: "player:read", Name: "Player Read"},
		{Code: "player:write", Name: "Player Write"},
		{Code: "binding:manage", Name: "Binding Manage"},
		{Code: "game:read", Name: "Game Read"},
		{Code: "game:write", Name: "Game Write"},
		{Code: "game:publish", Name: "Game Publish"},
		{Code: "agent_game_access:read", Name: "Agent Game Access Read"},
		{Code: "agent_game_access:write", Name: "Agent Game Access Write"},
		{Code: "rule:read", Name: "Rule Read"},
		{Code: "rule:write", Name: "Rule Write"},
		{Code: "rule:publish", Name: "Rule Publish"},
		{Code: "activity_reward:read", Name: "Activity Reward Read"},
		{Code: "activity_reward:write", Name: "Activity Reward Write"},
		{Code: "activity_reward:publish", Name: "Activity Reward Publish"},
		{Code: "settlement:read", Name: "Settlement Read"},
		{Code: "settlement:execute", Name: "Settlement Execute"},
		{Code: "withdrawal:read", Name: "Withdrawal Read"},
		{Code: "withdrawal:execute", Name: "Withdrawal Execute"},
		{Code: "settlement_bill:read", Name: "Settlement Bill Read"},
		{Code: "settlement_bill:confirm", Name: "Settlement Bill Confirm"},
		{Code: "settlement_bill:export", Name: "Settlement Bill Export"},
		{Code: "recalculation_task:read", Name: "Recalculation Task Read"},
		{Code: "recalculation_task:create", Name: "Recalculation Task Create"},
		{Code: "audit:read", Name: "Audit Read"},
		{Code: "risk:read", Name: "Risk Read"},
		{Code: "risk:write", Name: "Risk Write"},
		{Code: "platform_config:read", Name: "Platform Config Read"},
		{Code: "platform_config:write", Name: "Platform Config Write"},
		{Code: "report:read", Name: "Report Read"},
		{Code: "rbac:permissions:view", Name: "RBAC Permissions View"},
		{Code: "rbac:users:view", Name: "RBAC Users View"},
		{Code: "rbac:users:write", Name: "RBAC Users Write"},
		{Code: "rbac:roles:view", Name: "RBAC Roles View"},
		{Code: "rbac:roles:write", Name: "RBAC Roles Write"},
	}
}

func builtInRoles() []model.AdminRole {
	return []model.AdminRole{
		{Code: "agent", Name: "Agent Portal User"},
	}
}

func builtInRolePermissions() map[string][]string {
	return map[string][]string{
		"admin": {
			"agent:read", "agent:write", "tenant:read", "tenant:write", "brand:read", "brand:write",
			"invite_code:manage", "player:read", "player:write", "binding:manage",
			"game:read", "game:write", "game:publish", "agent_game_access:read", "agent_game_access:write",
			"rule:read", "rule:write", "rule:publish",
			"activity_reward:read", "activity_reward:write", "activity_reward:publish",
			"settlement:read", "settlement:execute", "withdrawal:read", "withdrawal:execute",
			"settlement_bill:read", "settlement_bill:confirm", "settlement_bill:export",
			"recalculation_task:read", "recalculation_task:create",
			"audit:read", "risk:read", "risk:write",
			"platform_config:read", "platform_config:write", "report:read",
			"rbac:permissions:view", "rbac:users:view", "rbac:users:write", "rbac:roles:view", "rbac:roles:write",
		},
		"agent": {
			"agent:read", "invite_code:manage", "player:read", "binding:manage", "settlement:read", "withdrawal:read",
		},
	}
}

func enumDictionarySeedConfigs() map[string]enumDictionarySeedConfig {
	return map[string]enumDictionarySeedConfig{
		"enum.dictionary.recharge_type": {
			Strict: true,
			Items: []enumDictionarySeedItem{
				{Value: "normal", Label: "常规充值", LabelEn: "Normal"},
				{Value: "first_deposit", Label: "首充", LabelEn: "First Deposit"},
				{Value: "vip", Label: "VIP充值", LabelEn: "VIP Recharge"},
			},
		},
		"enum.dictionary.activity_tag": {
			Strict: true,
			Items: []enumDictionarySeedItem{
				{Value: "campaign-a", Label: "活动A", LabelEn: "Campaign A"},
				{Value: "vip", Label: "VIP活动", LabelEn: "VIP Campaign"},
				{Value: "new-user", Label: "新用户活动", LabelEn: "New User Campaign"},
			},
		},
	}
}
