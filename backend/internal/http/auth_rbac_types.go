package http

import (
	"time"

	"game-admin/backend/internal/domain/model"
)

type authLoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authLoginIdentity struct {
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

type authUserScopeResponse struct {
	Level      ScopeLevel `json:"level"`
	TenantID   *uint64    `json:"tenantID,omitempty"`
	TenantName string     `json:"tenantName,omitempty"`
	TenantCode string     `json:"tenantCode,omitempty"`
	BrandID    *uint64    `json:"brandID,omitempty"`
	BrandName  string     `json:"brandName,omitempty"`
	BrandCode  string     `json:"brandCode,omitempty"`
	AgentID    *uint64    `json:"agentID,omitempty"`
}

type authUserResponse struct {
	ID          uint64                `json:"id"`
	Username    string                `json:"username"`
	DisplayName string                `json:"displayName"`
	Roles       []string              `json:"roles"`
	Permissions []string              `json:"permissions"`
	Scope       authUserScopeResponse `json:"scope"`
}

type authLoginResponse struct {
	Token       string            `json:"token"`
	Role        string            `json:"role"`
	Permissions []string          `json:"permissions"`
	Identity    authLoginIdentity `json:"identity"`
	User        authUserResponse  `json:"user"`
}

type authAccount struct {
	UserID       uint64
	Username     string
	DisplayName  string
	PasswordHash string
	AgentID      *uint64
	Token        string
	Role         string
	Permissions  map[string]struct{}
	TenantScope  TenantScope
}

type rbacRolePayload struct {
	TenantID    *uint64  `json:"tenantID"`
	BrandID     *uint64  `json:"brandID"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type rbacRoleResponse struct {
	model.AdminRole
	Permissions []string `json:"permissions"`
}

type rbacUserPayload struct {
	Username    string                `json:"username"`
	Password    string                `json:"password"`
	DisplayName string                `json:"displayName"`
	Status      model.AdminUserStatus `json:"status"`
	TenantID    *uint64               `json:"tenantID"`
	BrandID     *uint64               `json:"brandID"`
	AgentID     *uint64               `json:"agentID"`
	Roles       []string              `json:"roles"`
}

type rbacUserResponse struct {
	ID          uint64                `json:"id"`
	Username    string                `json:"username"`
	DisplayName string                `json:"displayName"`
	Status      model.AdminUserStatus `json:"status"`
	TenantID    *uint64               `json:"tenantID,omitempty"`
	BrandID     *uint64               `json:"brandID,omitempty"`
	AgentID     *uint64               `json:"agentID,omitempty"`
	Roles       []string              `json:"roles"`
	LastLoginAt *time.Time            `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
}

type agentGameAccessPayload struct {
	AgentID uint64             `json:"agentID"`
	GameID  uint64             `json:"gameID"`
	Status  model.AccessStatus `json:"status"`
	Remark  string             `json:"remark"`
}

type agentGameAccessListItem struct {
	model.AgentGameAccess
	AgentName string `json:"agentName"`
	GameCode  string `json:"gameCode"`
	GameName  string `json:"gameName"`
}
