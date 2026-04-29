package identity

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type Account struct {
	UserID       uint64
	Username     string
	DisplayName  string
	PasswordHash string
	AgentID      *uint64
	Token        string
	Role         string
	Permissions  map[string]struct{}
	TenantScope  shared.Scope
}

type AuthScope struct {
	Level      shared.ScopeLevel
	TenantID   *uint64
	TenantName string
	TenantCode string
	BrandID    *uint64
	BrandName  string
	BrandCode  string
	AgentID    *uint64
}

type AuthUser struct {
	ID          uint64
	Username    string
	DisplayName string
	Roles       []string
	Permissions []string
	Scope       AuthScope
}

type AuthLoginResponse struct {
	Token       string
	Role        string
	Permissions []string
	User        AuthUser
}

type Role struct {
	model.AdminRole
	Permissions []string
}

type User struct {
	ID          uint64
	Username    string
	DisplayName string
	Status      model.AdminUserStatus
	TenantID    *uint64
	BrandID     *uint64
	AgentID     *uint64
	Roles       []string
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RoleInput struct {
	TenantID    *uint64
	BrandID     *uint64
	Code        string
	Name        string
	Permissions []string
}

type UserInput struct {
	Username    string
	Password    string
	DisplayName string
	Status      model.AdminUserStatus
	TenantID    *uint64
	BrandID     *uint64
	AgentID     *uint64
	Roles       []string
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, repo: newRepository(db)}
}

func (s *Service) Login(username, password string) (AuthLoginResponse, Account, bool) {
	account, ok := s.accountByCredentials(username, password)
	if !ok {
		return AuthLoginResponse{}, Account{}, false
	}
	return s.buildAuthLoginResponse(account), account, true
}

func (s *Service) AccountByToken(token string) (Account, bool) {
	return s.accountByToken(token)
}

func (s *Service) BuildAuthLoginResponse(account Account) AuthLoginResponse {
	return s.buildAuthLoginResponse(account)
}

func (s *Service) ListPermissions() ([]model.AdminPermission, error) {
	return s.repo.ListPermissions()
}

func (s *Service) ListRoles(scope shared.Scope) ([]Role, error) {
	roles, err := s.repo.ListRoles(scope)
	if err != nil {
		return nil, err
	}
	items := make([]Role, 0, len(roles))
	for _, role := range roles {
		permissions, err := s.repo.RolePermissionCodes(role.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, Role{AdminRole: role, Permissions: permissions})
	}
	return items, nil
}

func (s *Service) ListUsers(scope shared.Scope) ([]User, error) {
	users, err := s.repo.ListUsers(scope)
	if err != nil {
		return nil, err
	}
	items := make([]User, 0, len(users))
	for _, user := range users {
		roles, err := s.repo.UserRoleCodes(user.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, s.userView(user, roles))
	}
	return items, nil
}

func (s *Service) CreateRole(scope shared.Scope, input RoleInput) (Role, error) {
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return Role{}, errors.New("code and name are required")
	}
	tenantID, brandID, err := s.resolveRBACScope(scope, input.TenantID, input.BrandID, false)
	if err != nil {
		return Role{}, err
	}
	if err := s.ensureAdminRoleCodeAvailable(code, tenantID, brandID); err != nil {
		return Role{}, err
	}
	permissionCodes := normalizeCodes(input.Permissions)
	role := model.AdminRole{TenantID: tenantID, BrandID: brandID, Code: code, Name: name}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		if err := s.replaceRolePermissions(tx, role.ID, permissionCodes); err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRBACRoleCreated,
			AggregateType:  "admin_role",
			AggregateID:    strconv.FormatUint(role.ID, 10),
			TenantID:       role.TenantID,
			BrandID:        role.BrandID,
			OccurredAt:     role.CreatedAt,
			Producer:       "identity-service",
			IdempotencyKey: "rbac.role.created:" + strconv.FormatUint(role.ID, 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        Role{AdminRole: role, Permissions: permissionCodes},
		})
		return err
	}); err != nil {
		return Role{}, err
	}
	return Role{AdminRole: role, Permissions: permissionCodes}, nil
}

func (s *Service) UpdateRole(scope shared.Scope, id uint64, input RoleInput) (Role, Role, error) {
	var role model.AdminRole
	if err := s.db.First(&role, id).Error; err != nil {
		return Role{}, Role{}, err
	}
	if !shared.MatchesScopedRecord(scope, role.TenantID, role.BrandID) {
		return Role{}, Role{}, gorm.ErrRecordNotFound
	}
	beforePermissions, err := s.rolePermissionCodes(role.ID)
	if err != nil {
		return Role{}, Role{}, err
	}
	before := Role{AdminRole: role, Permissions: beforePermissions}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = role.Name
	}
	permissionCodes := normalizeCodes(input.Permissions)
	currentPermissions := beforePermissions
	if name == role.Name && slices.Equal(normalizeCodes(currentPermissions), permissionCodes) {
		return before, Role{AdminRole: role, Permissions: permissionCodes}, nil
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&role).Update("name", name).Error; err != nil {
			return err
		}
		role.Name = name
		if err := s.replaceRolePermissions(tx, role.ID, permissionCodes); err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRBACRoleUpdated,
			AggregateType:  "admin_role",
			AggregateID:    strconv.FormatUint(role.ID, 10),
			TenantID:       role.TenantID,
			BrandID:        role.BrandID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "identity-service",
			IdempotencyKey: "rbac.role.updated:" + strconv.FormatUint(role.ID, 10) + ":" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        eventbus.PayloadBeforeAfter(before, Role{AdminRole: role, Permissions: permissionCodes}),
		})
		return err
	}); err != nil {
		return Role{}, Role{}, err
	}
	return before, Role{AdminRole: role, Permissions: permissionCodes}, nil
}

func (s *Service) DeleteRole(scope shared.Scope, id uint64) (Role, error) {
	var role model.AdminRole
	if err := s.db.First(&role, id).Error; err != nil {
		return Role{}, err
	}
	if !shared.MatchesScopedRecord(scope, role.TenantID, role.BrandID) {
		return Role{}, gorm.ErrRecordNotFound
	}
	permissionCodes, err := s.rolePermissionCodes(role.ID)
	if err != nil {
		return Role{}, err
	}
	before := Role{AdminRole: role, Permissions: permissionCodes}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM admin_role_permission WHERE role_id = ?", role.ID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&role).Error; err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRBACRoleDeleted,
			AggregateType:  "admin_role",
			AggregateID:    strconv.FormatUint(role.ID, 10),
			TenantID:       role.TenantID,
			BrandID:        role.BrandID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "identity-service",
			IdempotencyKey: "rbac.role.deleted:" + strconv.FormatUint(role.ID, 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        before,
		})
		return err
	}); err != nil {
		return Role{}, err
	}
	return before, nil
}

func (s *Service) CreateUser(scope shared.Scope, input UserInput) (User, error) {
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		return User{}, errors.New("username and password are required")
	}
	roleCodes := normalizeCodes(input.Roles)
	if len(roleCodes) == 0 {
		return User{}, errors.New("at least one role is required")
	}
	status := input.Status
	if status == "" {
		status = model.AdminUserStatusActive
	}
	if status != model.AdminUserStatusActive && status != model.AdminUserStatusDisabled {
		return User{}, errors.New("status is invalid")
	}
	requireScoped := input.AgentID == nil && (len(scope.TenantIDs) > 0 || len(scope.BrandIDs) > 0)
	tenantID, brandID, err := s.resolveRBACScope(scope, input.TenantID, input.BrandID, requireScoped)
	if err != nil {
		return User{}, err
	}
	if input.AgentID != nil {
		agent, err := s.loadAgentForScope(*input.AgentID)
		if err != nil {
			return User{}, err
		}
		if !shared.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
			return User{}, gorm.ErrRecordNotFound
		}
		tenantID = agent.TenantID
		brandID = agent.BrandID
	}
	if err := s.ensureAdminUsernameAvailable(username, tenantID, brandID); err != nil {
		return User{}, err
	}
	user := model.AdminUser{Username: username, PasswordHash: password, DisplayName: strings.TrimSpace(input.DisplayName), Status: status, AgentID: input.AgentID}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := s.replaceUserRoles(tx, user.ID, roleCodes, tenantID, brandID); err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRBACUserCreated,
			AggregateType:  "admin_user",
			AggregateID:    strconv.FormatUint(user.ID, 10),
			TenantID:       tenantID,
			BrandID:        brandID,
			OccurredAt:     user.CreatedAt,
			Producer:       "identity-service",
			IdempotencyKey: "rbac.user.created:" + strconv.FormatUint(user.ID, 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        s.userView(user, roleCodes),
		})
		return err
	}); err != nil {
		return User{}, err
	}
	return s.userView(user, roleCodes), nil
}

func (s *Service) UpdateUser(scope shared.Scope, id uint64, input UserInput) (User, User, error) {
	var (
		user model.AdminUser
		err  error
	)
	if err := s.db.First(&user, id).Error; err != nil {
		return User{}, User{}, err
	}
	if !s.adminUserInScope(scope, user.ID) {
		return User{}, User{}, gorm.ErrRecordNotFound
	}
	beforeRoles, err := s.userRoleCodes(user.ID)
	if err != nil {
		return User{}, User{}, err
	}
	before := s.userView(user, beforeRoles)
	roleCodes := normalizeCodes(input.Roles)
	if len(roleCodes) == 0 {
		roleCodes = beforeRoles
	}
	status := input.Status
	if status == "" {
		status = user.Status
	}
	if status != model.AdminUserStatusActive && status != model.AdminUserStatusDisabled {
		return User{}, User{}, errors.New("status is invalid")
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = user.DisplayName
	}
	password := strings.TrimSpace(input.Password)
	requireScoped := input.AgentID == nil && (len(scope.TenantIDs) > 0 || len(scope.BrandIDs) > 0)
	tenantID, brandID, err := s.resolveRBACScope(scope, input.TenantID, input.BrandID, requireScoped)
	if err != nil {
		return User{}, User{}, err
	}
	if input.AgentID != nil {
		agent, err := s.loadAgentForScope(*input.AgentID)
		if err != nil {
			return User{}, User{}, err
		}
		if !shared.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
			return User{}, User{}, gorm.ErrRecordNotFound
		}
		tenantID = agent.TenantID
		brandID = agent.BrandID
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"display_name": displayName, "status": status, "agent_id": input.AgentID}
		if password != "" {
			updates["password_hash"] = password
		}
		if err := tx.Model(&model.AdminUser{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := s.replaceUserRoles(tx, user.ID, roleCodes, tenantID, brandID); err != nil {
			return err
		}
		afterView := s.userView(model.AdminUser{
			BaseModel:   user.BaseModel,
			Username:    user.Username,
			DisplayName: displayName,
			Status:      status,
			AgentID:     input.AgentID,
			LastLoginAt: user.LastLoginAt,
		}, roleCodes)
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRBACUserUpdated,
			AggregateType:  "admin_user",
			AggregateID:    strconv.FormatUint(user.ID, 10),
			TenantID:       tenantID,
			BrandID:        brandID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "identity-service",
			IdempotencyKey: "rbac.user.updated:" + strconv.FormatUint(user.ID, 10) + ":" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        eventbus.PayloadBeforeAfter(before, afterView),
		})
		return err
	}); err != nil {
		return User{}, User{}, err
	}
	if err := s.db.First(&user, id).Error; err != nil {
		return User{}, User{}, err
	}
	return before, s.userView(user, roleCodes), nil
}

func (s *Service) DeleteUser(scope shared.Scope, id uint64) (User, error) {
	var (
		user model.AdminUser
		err  error
	)
	if err := s.db.First(&user, id).Error; err != nil {
		return User{}, err
	}
	if !s.adminUserInScope(scope, user.ID) {
		return User{}, gorm.ErrRecordNotFound
	}
	roles, err := s.userRoleCodes(user.ID)
	if err != nil {
		return User{}, err
	}
	before := s.userView(user, roles)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM admin_user_role WHERE user_id = ?", user.ID).Error; err != nil {
			return err
		}
		archivedUsername := fmt.Sprintf("%s__deleted_%d_%d", user.Username, user.ID, time.Now().UTC().Unix())
		if err := tx.Model(&model.AdminUser{}).Where("id = ?", user.ID).Update("username", archivedUsername).Error; err != nil {
			return err
		}
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventRBACUserDeleted,
			AggregateType:  "admin_user",
			AggregateID:    strconv.FormatUint(user.ID, 10),
			TenantID:       before.TenantID,
			BrandID:        before.BrandID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "identity-service",
			IdempotencyKey: "rbac.user.deleted:" + strconv.FormatUint(user.ID, 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        before,
		})
		return err
	}); err != nil {
		return User{}, err
	}
	return before, nil
}

func (s *Service) accountByCredentials(username, password string) (Account, bool) {
	if s.db == nil {
		return Account{}, false
	}
	users, err := s.repo.FindUsersByUsernameActive(username)
	if err != nil || len(users) == 0 {
		return Account{}, false
	}
	for _, user := range users {
		if user.PasswordHash != strings.TrimSpace(password) {
			continue
		}
		account, ok := s.loadAuthAccount(user)
		if !ok {
			continue
		}
		now := time.Now().UTC()
		_ = s.repo.UpdateUserLastLogin(user.ID, now)
		return account, true
	}
	return Account{}, false
}

func (s *Service) accountByToken(token string) (Account, bool) {
	if s.db == nil {
		return Account{}, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Account{}, false
	}
	var (
		user model.AdminUser
		err  error
	)
	if strings.HasPrefix(token, "uid:") {
		id, err := strconv.ParseUint(strings.TrimPrefix(token, "uid:"), 10, 64)
		if err != nil || id == 0 {
			return Account{}, false
		}
		if user, err = s.repo.FindActiveUserByID(id); err != nil {
			return Account{}, false
		}
		return s.loadAuthAccount(user)
	}
	if user, err = s.repo.FindActiveUserByUsername(token); err != nil {
		return Account{}, false
	}
	return s.loadAuthAccount(user)
}

func (s *Service) loadAuthAccount(user model.AdminUser) (Account, bool) {
	rows, err := s.repo.LoadAuthRoleRows(user.ID)
	if err != nil {
		return Account{}, false
	}
	roleCode := ""
	permissions := map[string]struct{}{}
	for _, row := range rows {
		if roleCode == "" && row.RoleCode != "" {
			roleCode = row.RoleCode
		}
		if row.PermissionCode != "" {
			permissions[row.PermissionCode] = struct{}{}
		}
	}
	if roleCode == "" {
		return Account{}, false
	}
	return Account{
		UserID:       user.ID,
		Username:     user.Username,
		DisplayName:  shared.FirstNonEmpty(user.DisplayName, user.Username),
		PasswordHash: user.PasswordHash,
		AgentID:      user.AgentID,
		Token:        fmt.Sprintf("uid:%d", user.ID),
		Role:         roleCode,
		Permissions:  permissions,
		TenantScope:  s.deriveTenantScope(user),
	}, true
}

func (s *Service) deriveTenantScope(user model.AdminUser) shared.Scope {
	if s.db == nil {
		return shared.Scope{}
	}
	if user.AgentID != nil {
		agent, err := s.repo.LoadAgent(*user.AgentID)
		if err != nil {
			return shared.Scope{AgentID: user.AgentID}
		}
		scope := shared.Scope{AgentID: user.AgentID}
		if agent.TenantID != nil {
			scope.TenantIDs = []uint64{*agent.TenantID}
		}
		if agent.BrandID != nil {
			scope.BrandIDs = []uint64{*agent.BrandID}
		}
		return shared.NormalizeScope(scope)
	}
	rows, err := s.repo.LoadScopedRoleRows(user.ID)
	if err != nil {
		return shared.Scope{}
	}
	var scope shared.Scope
	hasExplicitScope := false
	for _, row := range rows {
		if row.BrandID != nil && *row.BrandID > 0 {
			hasExplicitScope = true
			scope.BrandIDs = append(scope.BrandIDs, *row.BrandID)
		} else if row.TenantID != nil && *row.TenantID > 0 {
			hasExplicitScope = true
			scope.TenantIDs = append(scope.TenantIDs, *row.TenantID)
		}
		if code := strings.TrimSpace(row.TenantCode); code != "" {
			scope.TenantCodes = append(scope.TenantCodes, code)
		}
	}
	if hasExplicitScope {
		return shared.NormalizeScope(scope)
	}
	return shared.Scope{}
}

func (s *Service) authScope(account Account) AuthScope {
	scope := shared.NormalizeScope(account.TenantScope)
	response := AuthScope{AgentID: account.AgentID, Level: shared.Level(scope)}
	if len(scope.TenantIDs) == 1 {
		tenantID := scope.TenantIDs[0]
		response.TenantID = &tenantID
		if tenant, err := s.repo.LoadTenantSummary(tenantID); err == nil {
			response.TenantCode = tenant.Code
			response.TenantName = shared.FirstNonEmpty(tenant.DisplayName, tenant.Name, tenant.Code)
		}
	}
	if response.TenantID == nil && len(scope.BrandIDs) == 1 {
		if brand, err := s.repo.LoadBrandSummary(scope.BrandIDs[0]); err == nil {
			tenantID := brand.TenantID
			response.TenantID = &tenantID
			if tenant, err := s.repo.LoadTenantSummary(tenantID); err == nil {
				response.TenantCode = tenant.Code
				response.TenantName = shared.FirstNonEmpty(tenant.DisplayName, tenant.Name, tenant.Code)
			}
		}
	}
	if len(scope.BrandIDs) == 1 && (len(scope.TenantIDs) == 0 || scope.AgentID != nil) {
		brandID := scope.BrandIDs[0]
		response.BrandID = &brandID
		if brand, err := s.repo.LoadBrandSummary(brandID); err == nil {
			response.BrandCode = brand.Code
			response.BrandName = shared.FirstNonEmpty(brand.DisplayName, brand.Name, brand.Code)
		}
	}
	return response
}

func (s *Service) buildAuthLoginResponse(account Account) AuthLoginResponse {
	permissions := permissionList(account.Permissions)
	return AuthLoginResponse{
		Token:       account.Token,
		Role:        account.Role,
		Permissions: permissions,
		User: AuthUser{
			ID:          account.UserID,
			Username:    account.Username,
			DisplayName: account.DisplayName,
			Roles:       []string{account.Role},
			Permissions: permissions,
			Scope:       s.authScope(account),
		},
	}
}

func (s *Service) rolePermissionCodes(roleID uint64) ([]string, error) {
	return s.repo.RolePermissionCodes(roleID)
}

func (s *Service) userRoleCodes(userID uint64) ([]string, error) {
	return s.repo.UserRoleCodes(userID)
}

func (s *Service) userView(user model.AdminUser, roles []string) User {
	tenantID, brandID := s.repo.PrimaryUserRoleScope(user.ID)
	return User{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Status: user.Status, TenantID: tenantID, BrandID: brandID, AgentID: user.AgentID, Roles: normalizeCodes(roles), LastLoginAt: user.LastLoginAt, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}

func (s *Service) resolveRBACScope(scope shared.Scope, tenantID, brandID *uint64, requireScoped bool) (*uint64, *uint64, error) {
	scope = shared.NormalizeScope(scope)
	if tenantID != nil && *tenantID == 0 {
		tenantID = nil
	}
	if brandID != nil && *brandID == 0 {
		brandID = nil
	}
	if tenantID == nil && brandID == nil && (len(scope.TenantIDs) > 0 || len(scope.BrandIDs) > 0) {
		if len(scope.TenantIDs) == 1 {
			tenantID = &scope.TenantIDs[0]
		}
		if len(scope.TenantIDs) == 0 && len(scope.BrandIDs) == 1 {
			brandID = &scope.BrandIDs[0]
		}
	}
	resolvedTenantID, resolvedBrandID, err := shared.ResolveTenantBrandScope(s.db, scope, tenantID, brandID, requireScoped)
	if err != nil {
		return nil, nil, err
	}
	if requireScoped && resolvedTenantID == nil {
		return nil, nil, errors.New("tenant scope is required")
	}
	return resolvedTenantID, resolvedBrandID, nil
}

func (s *Service) ensureAdminRoleCodeAvailable(code string, tenantID, brandID *uint64) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("role code is required")
	}
	count, err := s.repo.CountAdminRoleByCodeInScope(code, tenantID, brandID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("role code already exists in current scope")
	}
	return nil
}

func (s *Service) ensureAdminUsernameAvailable(username string, tenantID, brandID *uint64) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	count, err := s.repo.CountActiveAdminUsersByUsernameInScope(username, tenantID, brandID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("username already exists")
	}
	deletedUsers, err := s.repo.FindDeletedAdminUsersByUsernameInScope(username, tenantID, brandID)
	if err != nil {
		return err
	}
	for _, deletedUser := range deletedUsers {
		archivedUsername := fmt.Sprintf("%s__deleted_%d_%d", deletedUser.Username, deletedUser.ID, time.Now().UTC().Unix())
		if err := s.repo.UpdateArchivedUsername(deletedUser.ID, archivedUsername); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) replaceRolePermissions(tx *gorm.DB, roleID uint64, permissionCodes []string) error {
	return s.repo.ReplaceRolePermissions(tx, roleID, permissionCodes)
}

func (s *Service) replaceUserRoles(tx *gorm.DB, userID uint64, roleCodes []string, tenantID, brandID *uint64) error {
	return s.repo.ReplaceUserRoles(tx, userID, roleCodes, tenantID, brandID)
}

func (s *Service) adminUserInScope(scope shared.Scope, userID uint64) bool {
	return s.repo.AdminUserInScope(scope, userID)
}

func (s *Service) primaryUserRoleScope(userID uint64) (*uint64, *uint64) {
	return s.repo.PrimaryUserRoleScope(userID)
}

func (s *Service) loadAgentForScope(id uint64) (model.Agent, error) {
	return s.repo.LoadAgent(id)
}

func permissionList(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for permission := range values {
		items = append(items, permission)
	}
	slices.Sort(items)
	return items
}

func normalizeCodes(values []string) []string {
	set := make(map[string]struct{}, len(values))
	codes := make([]string, 0, len(values))
	for _, value := range values {
		code := strings.TrimSpace(value)
		if code == "" {
			continue
		}
		if _, exists := set[code]; exists {
			continue
		}
		set[code] = struct{}{}
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

func applyUserRoleExactScope(query *gorm.DB, tenantID, brandID *uint64) *gorm.DB {
	if tenantID == nil {
		query = query.Where("aur.tenant_id IS NULL")
	} else {
		query = query.Where("aur.tenant_id = ?", *tenantID)
	}
	if brandID == nil {
		query = query.Where("aur.brand_id IS NULL")
	} else {
		query = query.Where("aur.brand_id = ?", *brandID)
	}
	return query
}

func applyAdminRoleAssignableScope(query *gorm.DB, tenantID, brandID *uint64) *gorm.DB {
	if tenantID == nil {
		query = query.Where("tenant_id IS NULL")
	} else {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	if brandID == nil {
		query = query.Where("brand_id IS NULL")
	} else {
		query = query.Where("(brand_id = ? OR brand_id IS NULL)", *brandID)
	}
	return query
}
