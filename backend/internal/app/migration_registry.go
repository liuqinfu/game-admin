package app

import (
	"reflect"
	"slices"
	"strings"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

type migrationStep struct {
	name    string
	modules []string
	run     func(*gorm.DB) error
}

var sharedMigrationModels = []any{
	&SystemInfo{},
	&model.DomainEvent{},
	&model.DomainEventDelivery{},
	&model.OperationAuditLog{},
}

var moduleMigrationModels = map[string][]any{
	"auth": {
		&model.AdminUser{},
	},
	"rbac": {
		&model.AdminRole{},
		&model.AdminPermission{},
		&model.AdminUserRole{},
		&model.AdminRolePermission{},
	},
	"tenant": {
		&model.Tenant{},
		&model.Brand{},
		&model.PlatformConfig{},
	},
	"agent": {
		&model.Agent{},
		&model.InviteCode{},
		&model.Player{},
		&model.Binding{},
		&model.BindingHistory{},
		&model.AgentInviteApplication{},
	},
	"relation": {
		&model.Agent{},
		&model.Relation{},
		&model.AgentRelationClosure{},
	},
	"game": {
		&model.Game{},
		&model.GameIntegrationKey{},
		&model.AgentGameAccess{},
	},
	"rule": {
		&model.PlatformConfig{},
		&model.Agent{},
		&model.Game{},
		&model.CommissionRule{},
		&model.RuleSnapshot{},
	},
	"activity": {
		&model.ActivityRewardRule{},
		&model.ActivityRewardRecord{},
	},
	"recharge": {
		&model.Player{},
		&model.Binding{},
		&model.Relation{},
		&model.AgentRelationClosure{},
		&model.Game{},
		&model.AgentGameAccess{},
		&model.RechargeOrder{},
		&model.RechargeCallbackLog{},
		&model.CommissionRecord{},
		&model.AgentAccount{},
		&model.AgentAccountLedger{},
	},
	"openapi": {
		&model.PlatformConfig{},
		&model.Game{},
		&model.GameIntegrationKey{},
		&model.AgentGameAccess{},
		&model.CommissionRule{},
		&model.RuleSnapshot{},
		&model.Player{},
		&model.InviteCode{},
		&model.Binding{},
		&model.RechargeOrder{},
		&model.RechargeCallbackLog{},
	},
	"settlement": {
		&model.Agent{},
		&model.CommissionRecord{},
		&model.SettlementBill{},
		&model.SettlementBillDetail{},
		&model.RecalculationTask{},
	},
	"account": {
		&model.RechargeOrder{},
		&model.CommissionRecord{},
		&model.AgentAccount{},
		&model.AgentAccountLedger{},
	},
	"withdrawal": {
		&model.Agent{},
		&model.AgentAccount{},
		&model.AgentAccountLedger{},
		&model.WithdrawalRequest{},
		&model.RiskCase{},
	},
	"risk": {
		&model.Agent{},
		&model.AgentAccountLedger{},
		&model.WithdrawalRequest{},
		&model.RiskCase{},
	},
	"report": {
		&model.Agent{},
		&model.AgentAccount{},
		&model.AgentAccountLedger{},
		&model.WithdrawalRequest{},
		&model.SettlementBill{},
		&model.RechargeOrder{},
		&model.RiskCase{},
	},
	"audit": {},
}

func autoMigrateModels(modules []string) []any {
	items := append([]any{}, sharedMigrationModels...)
	if len(normalizeMigrationModules(modules)) == 0 {
		return append(items, model.Phase1Models...)
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		seen[modelTypeKey(item)] = struct{}{}
	}
	for _, moduleName := range normalizeMigrationModules(modules) {
		for _, item := range moduleMigrationModels[moduleName] {
			key := modelTypeKey(item)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, item)
		}
	}
	return items
}

func migrationSteps(modules []string) []migrationStep {
	all := []migrationStep{
		{name: "drop legacy admin username index", modules: []string{"auth"}, run: dropLegacyAdminUsernameUniqueIndex},
		{name: "drop legacy admin role code index", modules: []string{"rbac"}, run: dropLegacyAdminRoleCodeUniqueIndex},
		{name: "migrate scoped commission rule keys", modules: []string{"rule"}, run: migrateScopedCommissionRuleKeys},
		{name: "seed permissions", modules: []string{"rbac"}, run: seedSystemPermissions},
		{name: "seed built-in roles", modules: []string{"rbac"}, run: seedBuiltInRoles},
		{name: "seed built-in role permissions", modules: []string{"rbac"}, run: seedBuiltInRolePermissions},
		{name: "seed enum dictionaries", modules: []string{"tenant", "rule", "openapi"}, run: seedEnumDictionaries},
	}
	if len(normalizeMigrationModules(modules)) == 0 {
		return all
	}
	items := make([]migrationStep, 0, len(all))
	for _, step := range all {
		if shouldRunMigrationStep(step, modules) {
			items = append(items, step)
		}
	}
	return items
}

func shouldRunMigrationStep(step migrationStep, modules []string) bool {
	normalized := normalizeMigrationModules(modules)
	if len(normalized) == 0 || len(step.modules) == 0 {
		return true
	}
	for _, moduleName := range step.modules {
		if slices.Contains(normalized, moduleName) {
			return true
		}
	}
	return false
}

func normalizeMigrationModules(modules []string) []string {
	if len(modules) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	items := make([]string, 0, len(modules))
	for _, moduleName := range modules {
		moduleName = strings.TrimSpace(moduleName)
		if moduleName == "" {
			continue
		}
		if _, ok := moduleMigrationModels[moduleName]; !ok {
			continue
		}
		if _, ok := seen[moduleName]; ok {
			continue
		}
		seen[moduleName] = struct{}{}
		items = append(items, moduleName)
	}
	return items
}

func modelTypeKey(item any) string {
	return reflect.TypeOf(item).String()
}
