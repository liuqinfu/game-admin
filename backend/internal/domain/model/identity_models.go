package model

import "time"

type AdminUser struct {
	BaseModel
	Username     string          `gorm:"size:64;not null;index"`
	PasswordHash string          `gorm:"size:255;not null"`
	DisplayName  string          `gorm:"size:128"`
	Status       AdminUserStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	AgentID      *uint64         `gorm:"index"`
	LastLoginAt  *time.Time      `gorm:"index"`
}

func (AdminUser) TableName() string { return "admin_user" }

type AdminRole struct {
	BaseModel
	TenantID *uint64 `gorm:"index;uniqueIndex:uk_admin_role_scope_code"`
	BrandID  *uint64 `gorm:"index;uniqueIndex:uk_admin_role_scope_code"`
	Code     string  `gorm:"size:64;not null;uniqueIndex:uk_admin_role_scope_code"`
	Name     string  `gorm:"size:128;not null"`
}

func (AdminRole) TableName() string { return "admin_role" }

type AdminPermission struct {
	BaseModel
	Code string `gorm:"size:128;not null;uniqueIndex"`
	Name string `gorm:"size:128;not null"`
}

func (AdminPermission) TableName() string { return "admin_permission" }

type AdminUserRole struct {
	BaseModel
	UserID   uint64  `gorm:"not null;uniqueIndex:uk_admin_user_role"`
	RoleID   uint64  `gorm:"not null;uniqueIndex:uk_admin_user_role"`
	TenantID *uint64 `gorm:"index"`
	BrandID  *uint64 `gorm:"index"`
}

func (AdminUserRole) TableName() string { return "admin_user_role" }

type AdminRolePermission struct {
	BaseModel
	RoleID       uint64 `gorm:"not null;uniqueIndex:uk_admin_role_permission"`
	PermissionID uint64 `gorm:"not null;uniqueIndex:uk_admin_role_permission"`
}

func (AdminRolePermission) TableName() string { return "admin_role_permission" }
