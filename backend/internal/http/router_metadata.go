package http

import (
	"slices"

	"game-admin/backend/internal/domain/model"
)

func defaultAgentStatus(status model.AgentStatus) model.AgentStatus {
	if status == "" {
		return model.AgentStatusPending
	}
	return status
}

func defaultTenantStatus(status model.TenantStatus) model.TenantStatus {
	if status == "" {
		return model.TenantStatusActive
	}
	return status
}

func defaultBrandStatus(status model.BrandStatus) model.BrandStatus {
	if status == "" {
		return model.BrandStatusActive
	}
	return status
}

func defaultActivityRewardRuleStatus(status model.ActivityRewardRuleStatus) model.ActivityRewardRuleStatus {
	if status == "" {
		return model.ActivityRewardRuleStatusDraft
	}
	return status
}

const (
	adminUsername                     = "admin"
	roleAdmin                         = "admin"
	roleOperator                      = "operator"
	roleFinance                       = "finance"
	permissionAgentRead               = "agent:read"
	permissionAgentWrite              = "agent:write"
	permissionTenantRead              = "tenant:read"
	permissionTenantWrite             = "tenant:write"
	permissionBrandRead               = "brand:read"
	permissionBrandWrite              = "brand:write"
	permissionInviteCodeManage        = "invite_code:manage"
	permissionPlayerRead              = "player:read"
	permissionPlayerWrite             = "player:write"
	permissionBindingManage           = "binding:manage"
	permissionGameRead                = "game:read"
	permissionGameWrite               = "game:write"
	permissionGamePublish             = "game:publish"
	permissionAgentGameAccessRead     = "agent_game_access:read"
	permissionAgentGameAccessWrite    = "agent_game_access:write"
	permissionRuleRead                = "rule:read"
	permissionRuleWrite               = "rule:write"
	permissionRulePublish             = "rule:publish"
	permissionActivityRewardRead      = "activity_reward:read"
	permissionActivityRewardWrite     = "activity_reward:write"
	permissionActivityRewardPublish   = "activity_reward:publish"
	permissionSettlementRead          = "settlement:read"
	permissionSettlementExecute       = "settlement:execute"
	permissionWithdrawalRead          = "withdrawal:read"
	permissionWithdrawalExecute       = "withdrawal:execute"
	permissionSettlementBillRead      = "settlement_bill:read"
	permissionSettlementBillConfirm   = "settlement_bill:confirm"
	permissionSettlementBillExport    = "settlement_bill:export"
	permissionRecalculationTaskRead   = "recalculation_task:read"
	permissionRecalculationTaskCreate = "recalculation_task:create"
	permissionAuditRead               = "audit:read"
	permissionRiskRead                = "risk:read"
	permissionRiskWrite               = "risk:write"
	permissionPlatformConfigRead      = "platform_config:read"
	permissionPlatformConfigWrite     = "platform_config:write"
	permissionReportRead              = "report:read"
	permissionRBACPermissionsView     = "rbac:permissions:view"
	permissionRBACUsersView           = "rbac:users:view"
	permissionRBACUsersWrite          = "rbac:users:write"
	permissionRBACRolesView           = "rbac:roles:view"
	permissionRBACRolesWrite          = "rbac:roles:write"
)

func permissionSet(values ...string) map[string]struct{} {
	permissions := make(map[string]struct{}, len(values))
	for _, value := range values {
		permissions[value] = struct{}{}
	}
	return permissions
}

func permissionList(values map[string]struct{}) []string {
	permissions := make([]string, 0, len(values))
	for value := range values {
		permissions = append(permissions, value)
	}
	slices.Sort(permissions)
	return permissions
}
