package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"game-admin/backend/internal/domain/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return AutoMigrateModules(db, nil)
}

func AutoMigrateModules(db *gorm.DB, modules []string) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if err := db.AutoMigrate(autoMigrateModels(modules)...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	for _, step := range migrationSteps(modules) {
		if err := step.run(db); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return nil
}

func dropLegacyAdminUsernameUniqueIndex(db *gorm.DB) error {
	for _, name := range []string{"idx_admin_user_username", "username"} {
		if db.Migrator().HasIndex(&model.AdminUser{}, name) {
			if err := db.Migrator().DropIndex(&model.AdminUser{}, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func dropLegacyAdminRoleCodeUniqueIndex(db *gorm.DB) error {
	for _, name := range []string{"idx_admin_role_code", "code"} {
		if db.Migrator().HasIndex(&model.AdminRole{}, name) {
			if err := db.Migrator().DropIndex(&model.AdminRole{}, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateScopedCommissionRuleKeys(db *gorm.DB) error {
	var rules []model.CommissionRule
	if err := db.Find(&rules).Error; err != nil {
		return err
	}
	for _, rule := range rules {
		parts := make([]string, 0, 6)
		if rule.TenantID != nil {
			parts = append(parts, fmt.Sprintf("tenant:%d", *rule.TenantID))
		} else {
			parts = append(parts, "tenant:platform")
		}
		if rule.BrandID != nil {
			parts = append(parts, fmt.Sprintf("brand:%d", *rule.BrandID))
		} else {
			parts = append(parts, "brand:all")
		}
		parts = append(parts, string(rule.Scope))
		if rule.AgentID != nil {
			parts = append(parts, fmt.Sprintf("agent:%d", *rule.AgentID))
		}
		if rule.GameID != nil {
			parts = append(parts, fmt.Sprintf("game:%d", *rule.GameID))
		}
		if rule.RuleName != "" {
			parts = append(parts, rule.RuleName)
		}
		uniqueKey := strings.Join(parts, "|")
		if uniqueKey == rule.UniqueKey {
			continue
		}
		if err := db.Model(&model.CommissionRule{}).Where("id = ?", rule.ID).Update("unique_key", uniqueKey).Error; err != nil {
			return err
		}
		if err := db.Model(&model.RuleSnapshot{}).Where("rule_id = ?", rule.ID).Update("rule_unique_key", uniqueKey).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedSystemPermissions(db *gorm.DB) error {
	for _, permission := range systemPermissions() {
		if err := db.Where("code = ?", permission.Code).FirstOrCreate(&permission).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedBuiltInRoles(db *gorm.DB) error {
	for _, role := range builtInRoles() {
		if err := db.Where("code = ?", role.Code).FirstOrCreate(&role).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedBuiltInRolePermissions(db *gorm.DB) error {
	for roleCode, permissionCodes := range builtInRolePermissions() {
		var role model.AdminRole
		if err := db.Where("code = ?", roleCode).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		for _, permissionCode := range permissionCodes {
			var permission model.AdminPermission
			if err := db.Where("code = ?", permissionCode).First(&permission).Error; err != nil {
				return err
			}
			rolePermission := model.AdminRolePermission{RoleID: role.ID, PermissionID: permission.ID}
			if err := db.Where("role_id = ? AND permission_id = ?", role.ID, permission.ID).FirstOrCreate(&rolePermission).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func seedEnumDictionaries(db *gorm.DB) error {
	for key, value := range enumDictionarySeedConfigs() {
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		record := model.PlatformConfig{
			Key:         key,
			Value:       datatypes.JSON(payload),
			Description: "Built-in enum dictionary",
			UpdatedBy:   "system",
		}
		if err := db.Where("tenant_id IS NULL AND brand_id IS NULL AND key = ?", key).Attrs(record).FirstOrCreate(&record).Error; err != nil {
			return err
		}
	}
	return nil
}
