package model

import (
	"time"

	"gorm.io/datatypes"
)

type Tenant struct {
	BaseModel
	Code           string         `gorm:"size:64;not null;uniqueIndex"`
	Name           string         `gorm:"size:128;not null"`
	DisplayName    string         `gorm:"size:128"`
	Status         TenantStatus   `gorm:"type:varchar(32);not null;default:'active';index"`
	DefaultBrandID *uint64        `gorm:"index"`
	Remark         string         `gorm:"size:255"`
	Meta           datatypes.JSON `gorm:"type:json"`
}

func (Tenant) TableName() string { return "tenant" }

type Brand struct {
	BaseModel
	TenantID    uint64         `gorm:"not null;index"`
	Code        string         `gorm:"size:64;not null;uniqueIndex:uk_brand_tenant_code"`
	Name        string         `gorm:"size:128;not null"`
	DisplayName string         `gorm:"size:128"`
	Status      BrandStatus    `gorm:"type:varchar(32);not null;default:'active';index"`
	Domain      string         `gorm:"size:255;index"`
	IsDefault   bool           `gorm:"not null;default:false;index"`
	Remark      string         `gorm:"size:255"`
	Meta        datatypes.JSON `gorm:"type:json"`
}

func (Brand) TableName() string { return "brand" }

type Agent struct {
	BaseModel
	TenantID              *uint64        `gorm:"index"`
	BrandID               *uint64        `gorm:"index"`
	AgentNo               string         `gorm:"size:64;not null;uniqueIndex"`
	Name                  string         `gorm:"size:128;not null"`
	DisplayName           string         `gorm:"size:128"`
	Phone                 string         `gorm:"size:32;index"`
	Email                 string         `gorm:"size:128;index"`
	Status                AgentStatus    `gorm:"type:varchar(32);not null;default:'pending';index"`
	Level                 uint32         `gorm:"not null;default:1"`
	ParentAgentID         *uint64        `gorm:"index"`
	InvitedByAgentID      *uint64        `gorm:"index"`
	PrimaryInviteCodeID   *uint64        `gorm:"index"`
	SettlementAccountNo   string         `gorm:"size:128;index"`
	SettlementAccountName string         `gorm:"size:128"`
	CountryCode           string         `gorm:"size:16"`
	Currency              string         `gorm:"size:16;not null;default:'CNY'"`
	Tags                  datatypes.JSON `gorm:"type:json"`
	Meta                  datatypes.JSON `gorm:"type:json"`
	ApprovedAt            *time.Time
	LastLoginAt           *time.Time
	Remark                string `gorm:"size:255"`
}

func (Agent) TableName() string { return "agent" }

type InviteCode struct {
	BaseModel
	TenantID     *uint64          `gorm:"index"`
	BrandID      *uint64          `gorm:"index"`
	AgentID      uint64           `gorm:"not null;index"`
	Code         string           `gorm:"size:64;not null;uniqueIndex"`
	Status       InviteCodeStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	IsPrimary    bool             `gorm:"not null;default:false;index"`
	MaxUseCount  uint32           `gorm:"not null;default:0"`
	UsedCount    uint32           `gorm:"not null;default:0"`
	ValidFrom    *time.Time       `gorm:"index"`
	ValidTo      *time.Time       `gorm:"index"`
	ExpiredAt    *time.Time       `gorm:"index"`
	LastUsedAt   *time.Time
	Channel      string         `gorm:"size:64;index"`
	ChannelScope datatypes.JSON `gorm:"type:json"`
	GameScope    datatypes.JSON `gorm:"type:json"`
	Remark       string         `gorm:"size:255"`
}

func (InviteCode) TableName() string { return "agent_invite_code" }

type Player struct {
	BaseModel
	TenantID         *uint64        `gorm:"index"`
	BrandID          *uint64        `gorm:"index"`
	PlayerNo         string         `gorm:"size:64;not null;uniqueIndex"`
	PlatformUserID   string         `gorm:"size:128;not null;uniqueIndex"`
	Nickname         string         `gorm:"size:128"`
	Phone            string         `gorm:"size:32;index"`
	RegisterGameID   *uint64        `gorm:"index"`
	RegisterIP       string         `gorm:"size:64"`
	RegisterDeviceID string         `gorm:"size:128"`
	CountryCode      string         `gorm:"size:16"`
	Currency         string         `gorm:"size:16;not null;default:'CNY'"`
	Status           string         `gorm:"size:32;not null;default:'active';index"`
	Meta             datatypes.JSON `gorm:"type:json"`
	RegisteredAt     time.Time      `gorm:"not null;index"`
}

func (Player) TableName() string { return "player" }

type Binding struct {
	BaseModel
	TenantID       *uint64       `gorm:"index"`
	BrandID        *uint64       `gorm:"index"`
	PlayerID       uint64        `gorm:"not null;uniqueIndex:uk_player_active_binding"`
	AgentID        uint64        `gorm:"not null;index"`
	InviteCodeID   *uint64       `gorm:"index"`
	Status         BindingStatus `gorm:"type:varchar(32);not null;default:'bound';index;uniqueIndex:uk_player_active_binding"`
	Source         BindingSource `gorm:"type:varchar(32);not null;default:'register'"`
	BoundAt        time.Time     `gorm:"not null;index"`
	EffectiveFrom  time.Time     `gorm:"not null"`
	EffectiveTo    *time.Time    `gorm:"index"`
	ApprovedBy     string        `gorm:"size:64"`
	ApprovalReason string        `gorm:"size:255"`
	Remark         string        `gorm:"size:255"`
}

func (Binding) TableName() string { return "user_agent_binding" }

type BindingHistory struct {
	BaseModel
	TenantID        *uint64        `gorm:"index"`
	BrandID         *uint64        `gorm:"index"`
	BindingID       uint64         `gorm:"not null;index"`
	PlayerID        uint64         `gorm:"not null;index"`
	FromAgentID     *uint64        `gorm:"index"`
	ToAgentID       uint64         `gorm:"not null;index"`
	InviteCodeID    *uint64        `gorm:"index"`
	Status          BindingStatus  `gorm:"type:varchar(32);not null;index"`
	Source          BindingSource  `gorm:"type:varchar(32);not null"`
	ChangedAt       time.Time      `gorm:"not null;index"`
	ChangedBy       string         `gorm:"size:64"`
	ChangeReason    string         `gorm:"size:255"`
	SnapshotPayload datatypes.JSON `gorm:"type:json"`
}

func (BindingHistory) TableName() string { return "user_agent_binding_history" }

type Relation struct {
	BaseModel
	TenantID          *uint64        `gorm:"index"`
	BrandID           *uint64        `gorm:"index"`
	AncestorAgentID   uint64         `gorm:"not null;uniqueIndex:uk_agent_relation_path"`
	DescendantAgentID uint64         `gorm:"not null;uniqueIndex:uk_agent_relation_path"`
	Depth             uint32         `gorm:"not null;uniqueIndex:uk_agent_relation_path"`
	DirectParentID    *uint64        `gorm:"index"`
	RelationType      RelationType   `gorm:"type:varchar(32);not null;default:'closure';index"`
	Status            RelationStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	EffectiveFrom     time.Time      `gorm:"not null;index"`
	EffectiveTo       *time.Time     `gorm:"index"`
}

func (Relation) TableName() string { return "agent_relation" }

type AgentRelationClosure struct {
	BaseModel
	TenantID          *uint64        `gorm:"index"`
	BrandID           *uint64        `gorm:"index"`
	AncestorAgentID   uint64         `gorm:"not null;uniqueIndex:uk_agent_relation_closure_path;index:idx_agent_relation_closure_ancestor_depth,priority:1"`
	DescendantAgentID uint64         `gorm:"not null;uniqueIndex:uk_agent_relation_closure_path;index:idx_agent_relation_closure_descendant_depth,priority:1"`
	Depth             uint32         `gorm:"not null;uniqueIndex:uk_agent_relation_closure_path;index:idx_agent_relation_closure_ancestor_depth,priority:2;index:idx_agent_relation_closure_descendant_depth,priority:2"`
	PathSnapshot      string         `gorm:"size:1024;not null;default:''"`
	ViaDirectParentID *uint64        `gorm:"index"`
	RelationType      RelationType   `gorm:"type:varchar(32);not null;default:'closure';index"`
	Status            RelationStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	EffectiveFrom     time.Time      `gorm:"not null;index"`
	EffectiveTo       *time.Time     `gorm:"index"`
}

func (AgentRelationClosure) TableName() string { return "agent_relation_closure" }

type AgentInviteApplication struct {
	BaseModel
	TenantID           *uint64                      `gorm:"index"`
	BrandID            *uint64                      `gorm:"index"`
	ApplicantAgentID   uint64                       `gorm:"not null;index"`
	InviterAgentID     uint64                       `gorm:"not null;index"`
	InviteCodeID       *uint64                      `gorm:"index"`
	Status             AgentInviteApplicationStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	ApplyRemark        string                       `gorm:"size:255"`
	AuditRemark        string                       `gorm:"size:255"`
	AuditBy            string                       `gorm:"size:64;index"`
	AuditedAt          *time.Time                   `gorm:"index"`
	ApprovedRelationID *uint64                      `gorm:"index"`
}

func (AgentInviteApplication) TableName() string { return "agent_invite_application" }
