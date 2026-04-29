package http

import (
	"strings"
	"time"

	"game-admin/backend/internal/cache"
	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
	gametrics "game-admin/backend/internal/metrics"
	"gorm.io/gorm"
)

type HealthResponse struct {
	Status       string             `json:"status"`
	Service      string             `json:"service"`
	Version      string             `json:"version"`
	Timestamp    time.Time          `json:"timestamp"`
	RequestID    string             `json:"requestID,omitempty"`
	TraceID      string             `json:"traceID,omitempty"`
	Dependencies []HealthDependency `json:"dependencies,omitempty"`
}

type HealthDependency struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Optional  bool   `json:"optional,omitempty"`
	CheckedAt string `json:"checkedAt,omitempty"`
}

type HealthProvider interface {
	Live(time.Time, string, string) HealthResponse
	Ready(time.Time, string, string) HealthResponse
}

type RouterDependencies struct {
	Health  HealthProvider
	Metrics *gametrics.Runtime
	DB      *gorm.DB
	Redis   *cache.Runtime
	Config  config.Config
}

type RouterConfig struct {
	ServiceName    string
	EnabledModules []string
}

const (
	RouterModuleAll          = "all"
	RouterModuleAuth         = "auth"
	RouterModuleRBAC         = "rbac"
	RouterModuleTenant       = "tenant"
	RouterModuleAgent        = "agent"
	RouterModuleRelation     = "relation"
	RouterModuleGame         = "game"
	RouterModuleRule         = "rule"
	RouterModuleActivity     = "activity"
	RouterModuleRecharge     = "recharge"
	RouterModuleSettlement   = "settlement"
	RouterModuleAccount      = "account"
	RouterModuleWithdrawal   = "withdrawal"
	RouterModuleRisk         = "risk"
	RouterModuleReport       = "report"
	RouterModuleAudit        = "audit"
	RouterModuleOpenAPI      = "openapi"
	RouterModuleNotification = "notification"
	RouterModuleDataSync     = "data-sync"
)

type TenantScope struct {
	TenantIDs   []uint64
	TenantCodes []string
	BrandIDs    []uint64
	AgentID     *uint64
}

type ScopeLevel string

const (
	ScopeLevelPlatform ScopeLevel = "platform"
	ScopeLevelTenant   ScopeLevel = "tenant"
	ScopeLevelBrand    ScopeLevel = "brand"
	ScopeLevelAgent    ScopeLevel = "agent"
)

type listResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type agentPayload struct {
	TenantID              *uint64           `json:"tenantID"`
	BrandID               *uint64           `json:"brandID"`
	AgentNo               string            `json:"agentNo"`
	Name                  string            `json:"name"`
	DisplayName           string            `json:"displayName"`
	Phone                 string            `json:"phone"`
	Email                 string            `json:"email"`
	Status                model.AgentStatus `json:"status"`
	Level                 uint32            `json:"level"`
	SettlementAccountNo   string            `json:"settlementAccountNo"`
	SettlementAccountName string            `json:"settlementAccountName"`
	Remark                string            `json:"remark"`
}

func (c RouterConfig) moduleEnabled(module string) bool {
	if len(c.EnabledModules) == 0 {
		return true
	}

	for _, item := range c.EnabledModules {
		item = strings.TrimSpace(strings.ToLower(item))
		if item == "" {
			continue
		}
		if item == RouterModuleAll || item == strings.ToLower(module) {
			return true
		}
	}
	return false
}
