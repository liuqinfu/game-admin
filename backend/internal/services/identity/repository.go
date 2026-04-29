package identity

import (
	"errors"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type roleRow struct {
	RoleCode       string
	PermissionCode string
}

type scopedRoleRow struct {
	TenantID   *uint64
	TenantCode string
	BrandID    *uint64
}

type Repository interface {
	ListPermissions() ([]model.AdminPermission, error)
	ListRoles(scope shared.Scope) ([]model.AdminRole, error)
	ListUsers(scope shared.Scope) ([]model.AdminUser, error)
	FindRole(id uint64) (model.AdminRole, error)
	FindUser(id uint64) (model.AdminUser, error)
	FindUsersByUsernameActive(username string) ([]model.AdminUser, error)
	FindActiveUserByID(id uint64) (model.AdminUser, error)
	FindActiveUserByUsername(username string) (model.AdminUser, error)
	LoadAuthRoleRows(userID uint64) ([]roleRow, error)
	LoadScopedRoleRows(userID uint64) ([]scopedRoleRow, error)
	RolePermissionCodes(roleID uint64) ([]string, error)
	UserRoleCodes(userID uint64) ([]string, error)
	PrimaryUserRoleScope(userID uint64) (*uint64, *uint64)
	CountAdminRoleByCodeInScope(code string, tenantID, brandID *uint64) (int64, error)
	CountActiveAdminUsersByUsernameInScope(username string, tenantID, brandID *uint64) (int64, error)
	FindDeletedAdminUsersByUsernameInScope(username string, tenantID, brandID *uint64) ([]model.AdminUser, error)
	UpdateUserLastLogin(userID uint64, at time.Time) error
	UpdateArchivedUsername(userID uint64, username string) error
	LoadAgent(id uint64) (model.Agent, error)
	LoadTenantSummary(id uint64) (model.Tenant, error)
	LoadBrandSummary(id uint64) (model.Brand, error)
	AdminUserInScope(scope shared.Scope, userID uint64) bool
	WithTx(func(*gorm.DB) error) error
	ReplaceRolePermissions(tx *gorm.DB, roleID uint64, permissionCodes []string) error
	ReplaceUserRoles(tx *gorm.DB, userID uint64, roleCodes []string, tenantID, brandID *uint64) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListPermissions() ([]model.AdminPermission, error) {
	var items []model.AdminPermission
	return items, r.db.Order("code asc").Find(&items).Error
}

func (r *gormRepository) ListRoles(scope shared.Scope) ([]model.AdminRole, error) {
	var roles []model.AdminRole
	query := applyAdminRoleScope(r.db.Order("code asc"), scope)
	return roles, query.Find(&roles).Error
}

func (r *gormRepository) ListUsers(scope shared.Scope) ([]model.AdminUser, error) {
	var users []model.AdminUser
	query := applyAdminUserScope(r.db.Model(&model.AdminUser{}).Order("created_at desc, id desc"), scope)
	return users, query.Find(&users).Error
}

func (r *gormRepository) FindRole(id uint64) (model.AdminRole, error) {
	var role model.AdminRole
	return role, r.db.First(&role, id).Error
}

func (r *gormRepository) FindUser(id uint64) (model.AdminUser, error) {
	var user model.AdminUser
	return user, r.db.First(&user, id).Error
}

func (r *gormRepository) FindUsersByUsernameActive(username string) ([]model.AdminUser, error) {
	var users []model.AdminUser
	err := r.db.Where("username = ? AND status = ?", strings.TrimSpace(username), model.AdminUserStatusActive).Order("id asc").Find(&users).Error
	return users, err
}

func (r *gormRepository) FindActiveUserByID(id uint64) (model.AdminUser, error) {
	var user model.AdminUser
	return user, r.db.Where("id = ? AND status = ?", id, model.AdminUserStatusActive).First(&user).Error
}

func (r *gormRepository) FindActiveUserByUsername(username string) (model.AdminUser, error) {
	var user model.AdminUser
	return user, r.db.Where("username = ? AND status = ?", strings.TrimSpace(username), model.AdminUserStatusActive).First(&user).Error
}

func (r *gormRepository) LoadAuthRoleRows(userID uint64) ([]roleRow, error) {
	var rows []roleRow
	err := r.db.Table("admin_user_role aur").
		Select("ar.code AS role_code, ap.code AS permission_code").
		Joins("JOIN admin_role ar ON ar.id = aur.role_id").
		Joins("LEFT JOIN admin_role_permission arp ON arp.role_id = ar.id").
		Joins("LEFT JOIN admin_permission ap ON ap.id = arp.permission_id").
		Where("aur.user_id = ?", userID).
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) LoadScopedRoleRows(userID uint64) ([]scopedRoleRow, error) {
	var rows []scopedRoleRow
	err := r.db.Table("admin_user_role aur").
		Select("aur.tenant_id, COALESCE(t.code, '') AS tenant_code, aur.brand_id").
		Joins("LEFT JOIN tenant t ON t.id = aur.tenant_id").
		Where("aur.user_id = ?", userID).
		Order("aur.id asc").
		Scan(&rows).Error
	return rows, err
}

func (r *gormRepository) RolePermissionCodes(roleID uint64) ([]string, error) {
	var codes []string
	err := r.db.Table("admin_role_permission arp").
		Distinct("ap.code").
		Joins("JOIN admin_permission ap ON ap.id = arp.permission_id").
		Where("arp.role_id = ?", roleID).
		Order("ap.code asc").
		Scan(&codes).Error
	return codes, err
}

func (r *gormRepository) UserRoleCodes(userID uint64) ([]string, error) {
	var codes []string
	err := r.db.Table("admin_user_role aur").
		Distinct("ar.code").
		Joins("JOIN admin_role ar ON ar.id = aur.role_id").
		Where("aur.user_id = ?", userID).
		Order("ar.code asc").
		Scan(&codes).Error
	return codes, err
}

func (r *gormRepository) PrimaryUserRoleScope(userID uint64) (*uint64, *uint64) {
	var row model.AdminUserRole
	if err := r.db.Where("user_id = ?", userID).Order("id asc").First(&row).Error; err != nil {
		return nil, nil
	}
	return row.TenantID, row.BrandID
}

func (r *gormRepository) CountAdminRoleByCodeInScope(code string, tenantID, brandID *uint64) (int64, error) {
	var count int64
	query := applyUserRoleLikeExactScope(r.db.Model(&model.AdminRole{}).Where("code = ?", code), "tenant_id", "brand_id", tenantID, brandID)
	return count, query.Count(&count).Error
}

func (r *gormRepository) CountActiveAdminUsersByUsernameInScope(username string, tenantID, brandID *uint64) (int64, error) {
	var count int64
	query := r.db.Table("admin_user AS au").Joins("JOIN admin_user_role aur ON aur.user_id = au.id").Where("au.username = ? AND au.deleted_at IS NULL", username)
	query = applyUserRoleExactScope(query, tenantID, brandID)
	return count, query.Count(&count).Error
}

func (r *gormRepository) FindDeletedAdminUsersByUsernameInScope(username string, tenantID, brandID *uint64) ([]model.AdminUser, error) {
	var deletedUsers []model.AdminUser
	query := r.db.Unscoped().Table("admin_user AS au").Select("au.*").Joins("JOIN admin_user_role aur ON aur.user_id = au.id").Where("au.username = ? AND au.deleted_at IS NOT NULL", username)
	query = applyUserRoleExactScope(query, tenantID, brandID)
	return deletedUsers, query.Find(&deletedUsers).Error
}

func (r *gormRepository) UpdateUserLastLogin(userID uint64, at time.Time) error {
	return r.db.Model(&model.AdminUser{}).Where("id = ?", userID).Update("last_login_at", at).Error
}

func (r *gormRepository) UpdateArchivedUsername(userID uint64, username string) error {
	return r.db.Unscoped().Model(&model.AdminUser{}).Where("id = ?", userID).Update("username", username).Error
}

func (r *gormRepository) LoadAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) LoadTenantSummary(id uint64) (model.Tenant, error) {
	var tenant model.Tenant
	return tenant, r.db.Select("code", "name", "display_name").First(&tenant, id).Error
}

func (r *gormRepository) LoadBrandSummary(id uint64) (model.Brand, error) {
	var brand model.Brand
	return brand, r.db.Select("tenant_id", "code", "name", "display_name").First(&brand, id).Error
}

func (r *gormRepository) AdminUserInScope(scope shared.Scope, userID uint64) bool {
	scope = shared.NormalizeScope(scope)
	if len(scope.TenantIDs) == 0 && len(scope.BrandIDs) == 0 {
		return true
	}
	var count int64
	query := r.db.Model(&model.AdminUserRole{}).Where("user_id = ?", userID)
	switch {
	case len(scope.TenantIDs) > 0 && len(scope.BrandIDs) > 0:
		query = query.Where("(tenant_id IN ? OR brand_id IN ?)", scope.TenantIDs, scope.BrandIDs)
	case len(scope.TenantIDs) > 0:
		query = query.Where("tenant_id IN ?", scope.TenantIDs)
	case len(scope.BrandIDs) > 0:
		query = query.Where("brand_id IN ?", scope.BrandIDs)
	}
	return query.Count(&count).Error == nil && count > 0
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}

func (r *gormRepository) ReplaceRolePermissions(tx *gorm.DB, roleID uint64, permissionCodes []string) error {
	if err := tx.Exec("DELETE FROM admin_role_permission WHERE role_id = ?", roleID).Error; err != nil {
		return err
	}
	if len(permissionCodes) == 0 {
		return nil
	}
	var permissions []model.AdminPermission
	if err := tx.Where("code IN ?", permissionCodes).Find(&permissions).Error; err != nil {
		return err
	}
	permissionByCode := make(map[string]model.AdminPermission, len(permissions))
	for _, permission := range permissions {
		permissionByCode[permission.Code] = permission
	}
	for _, code := range permissionCodes {
		permission, ok := permissionByCode[code]
		if !ok {
			return errors.New("permission not found: " + code)
		}
		if err := tx.Create(&model.AdminRolePermission{RoleID: roleID, PermissionID: permission.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *gormRepository) ReplaceUserRoles(tx *gorm.DB, userID uint64, roleCodes []string, tenantID, brandID *uint64) error {
	if err := tx.Exec("DELETE FROM admin_user_role WHERE user_id = ?", userID).Error; err != nil {
		return err
	}
	roleCodes = normalizeCodes(roleCodes)
	if len(roleCodes) == 0 {
		return nil
	}
	var roles []model.AdminRole
	query := applyAdminRoleAssignableScope(tx.Where("code IN ?", roleCodes), tenantID, brandID)
	if err := query.Find(&roles).Error; err != nil {
		return err
	}
	roleByCode := make(map[string]model.AdminRole, len(roles))
	for _, role := range roles {
		roleByCode[role.Code] = role
	}
	for _, code := range roleCodes {
		role, ok := roleByCode[code]
		if !ok {
			return errors.New("role not found: " + code)
		}
		if err := tx.Create(&model.AdminUserRole{UserID: userID, RoleID: role.ID, TenantID: tenantID, BrandID: brandID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyAdminRoleScope(query *gorm.DB, scope shared.Scope) *gorm.DB {
	scope = shared.NormalizeScope(scope)
	if len(scope.TenantIDs) == 0 && len(scope.BrandIDs) == 0 {
		return query
	}
	switch {
	case len(scope.TenantIDs) > 0 && len(scope.BrandIDs) > 0:
		query = query.Where("(tenant_id IN ? OR brand_id IN ?)", scope.TenantIDs, scope.BrandIDs)
	case len(scope.TenantIDs) > 0:
		query = query.Where("tenant_id IN ?", scope.TenantIDs)
	case len(scope.BrandIDs) > 0:
		query = query.Where("brand_id IN ?", scope.BrandIDs)
	}
	return query
}

func applyAdminUserScope(query *gorm.DB, scope shared.Scope) *gorm.DB {
	scope = shared.NormalizeScope(scope)
	if len(scope.TenantIDs) == 0 && len(scope.BrandIDs) == 0 {
		return query
	}
	switch {
	case len(scope.TenantIDs) > 0 && len(scope.BrandIDs) > 0:
		return query.Where("EXISTS (SELECT 1 FROM admin_user_role aur WHERE aur.user_id = admin_user.id AND (aur.tenant_id IN ? OR aur.brand_id IN ?))", scope.TenantIDs, scope.BrandIDs)
	case len(scope.TenantIDs) > 0:
		return query.Where("EXISTS (SELECT 1 FROM admin_user_role aur WHERE aur.user_id = admin_user.id AND aur.tenant_id IN ?)", scope.TenantIDs)
	default:
		return query.Where("EXISTS (SELECT 1 FROM admin_user_role aur WHERE aur.user_id = admin_user.id AND aur.brand_id IN ?)", scope.BrandIDs)
	}
}

func applyUserRoleLikeExactScope(query *gorm.DB, tenantColumn, brandColumn string, tenantID, brandID *uint64) *gorm.DB {
	if tenantID == nil {
		query = query.Where(tenantColumn + " IS NULL")
	} else {
		query = query.Where(tenantColumn+" = ?", *tenantID)
	}
	if brandID == nil {
		query = query.Where(brandColumn + " IS NULL")
	} else {
		query = query.Where(brandColumn+" = ?", *brandID)
	}
	return query
}
