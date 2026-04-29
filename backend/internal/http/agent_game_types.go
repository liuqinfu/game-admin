package http

import (
	"time"

	"game-admin/backend/internal/domain/model"
)

type inviteCodePayload struct {
	AgentID      uint64                 `json:"agentID"`
	Code         string                 `json:"code"`
	Status       model.InviteCodeStatus `json:"status"`
	Channel      string                 `json:"channel"`
	Remark       string                 `json:"remark"`
	MaxUseCount  uint32                 `json:"maxUseCount"`
	IsPrimary    bool                   `json:"isPrimary"`
	ValidFrom    string                 `json:"validFrom"`
	ValidTo      string                 `json:"validTo"`
	ChannelScope []string               `json:"channelScope"`
	GameScope    []uint64               `json:"gameScope"`
}

type registerWithInvitePayload struct {
	PlayerNo       string `json:"playerNo"`
	PlatformUserID string `json:"platformUserID"`
	Nickname       string `json:"nickname"`
	Phone          string `json:"phone"`
	CountryCode    string `json:"countryCode"`
	Currency       string `json:"currency"`
	InviteCode     string `json:"inviteCode"`
	Remark         string `json:"remark"`
}

type playerPayload struct {
	TenantID       *uint64 `json:"tenantID"`
	BrandID        *uint64 `json:"brandID"`
	PlayerNo       string  `json:"playerNo"`
	PlatformUserID string  `json:"platformUserID"`
	Nickname       string  `json:"nickname"`
	Phone          string  `json:"phone"`
	CountryCode    string  `json:"countryCode"`
	Currency       string  `json:"currency"`
}

type bindingPayload struct {
	PlayerID uint64 `json:"playerID"`
	Code     string `json:"inviteCode"`
	Remark   string `json:"remark"`
}

type agentInviteApplicationPayload struct {
	ApplicantAgentID uint64 `json:"applicantAgentID"`
	InviteCode       string `json:"inviteCode"`
	ApplyRemark      string `json:"applyRemark"`
}

type agentInviteApplicationAuditPayload struct {
	Status      model.AgentInviteApplicationStatus `json:"status"`
	AuditRemark string                             `json:"auditRemark"`
}

type gamePayload struct {
	TenantID    *uint64          `json:"tenantID"`
	BrandID     *uint64          `json:"brandID"`
	GameCode    string           `json:"gameCode"`
	Name        string           `json:"name"`
	Vendor      string           `json:"vendor"`
	Category    string           `json:"category"`
	Status      model.GameStatus `json:"status"`
	IsAgentable *bool            `json:"isAgentable"`
	Sort        int32            `json:"sort"`
	Remark      string           `json:"remark"`
}

type gameCreateResponse struct {
	Game                  model.Game              `json:"game"`
	IntegrationCredential *gameCredentialResponse `json:"integrationCredential,omitempty"`
}

type gameCredentialResponse struct {
	ID         uint64                         `json:"id"`
	GameID     uint64                         `json:"gameID"`
	Name       string                         `json:"name"`
	AccessKey  string                         `json:"accessKey"`
	SecretKey  string                         `json:"secretKey,omitempty"`
	Status     model.GameIntegrationKeyStatus `json:"status"`
	Scopes     []string                       `json:"scopes"`
	CreatedAt  time.Time                      `json:"createdAt"`
	LastUsedAt *time.Time                     `json:"lastUsedAt,omitempty"`
	RotatedAt  *time.Time                     `json:"rotatedAt,omitempty"`
	ExpiresAt  *time.Time                     `json:"expiresAt,omitempty"`
	Remark     string                         `json:"remark,omitempty"`
}

type gameOpenAPIContext struct {
	Credential model.GameIntegrationKey
	Game       model.Game
	Scopes     map[string]struct{}
}

type tenantPayload struct {
	Code        string             `json:"code"`
	Name        string             `json:"name"`
	DisplayName string             `json:"displayName"`
	Status      model.TenantStatus `json:"status"`
	Remark      string             `json:"remark"`
}

type tenantStatusPayload struct {
	Status model.TenantStatus `json:"status"`
}

type brandPayload struct {
	TenantID    uint64            `json:"tenantID"`
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Status      model.BrandStatus `json:"status"`
	Domain      string            `json:"domain"`
	IsDefault   bool              `json:"isDefault"`
	Remark      string            `json:"remark"`
}

type brandStatusPayload struct {
	Status model.BrandStatus `json:"status"`
}
