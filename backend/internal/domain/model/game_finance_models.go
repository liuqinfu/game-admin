package model

import (
	"time"

	"gorm.io/datatypes"
)

type Game struct {
	BaseModel
	TenantID    *uint64    `gorm:"index"`
	BrandID     *uint64    `gorm:"index"`
	GameCode    string     `gorm:"size:64;not null;uniqueIndex"`
	Name        string     `gorm:"size:128;not null"`
	Vendor      string     `gorm:"size:128;index"`
	Category    string     `gorm:"size:64;index"`
	Status      GameStatus `gorm:"type:varchar(32);not null;default:'draft';index"`
	IsAgentable bool       `gorm:"not null;default:true;index"`
	Sort        int32      `gorm:"not null;default:0"`
	Currency    string     `gorm:"size:16;not null;default:'CNY'"`
	LaunchAt    *time.Time
	OfflineAt   *time.Time
	ExtraConfig datatypes.JSON `gorm:"type:json"`
	Remark      string         `gorm:"size:255"`
}

func (Game) TableName() string { return "game" }

type GameIntegrationKey struct {
	BaseModel
	TenantID         *uint64                  `gorm:"index"`
	BrandID          *uint64                  `gorm:"index"`
	GameID           uint64                   `gorm:"not null;index"`
	Name             string                   `gorm:"size:128;not null"`
	AccessKey        string                   `gorm:"size:64;not null;uniqueIndex"`
	SecretCiphertext string                   `gorm:"size:255;not null"`
	Status           GameIntegrationKeyStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	Scopes           datatypes.JSON           `gorm:"type:json"`
	AllowedIPs       datatypes.JSON           `gorm:"type:json"`
	AllowedEndpoints datatypes.JSON           `gorm:"type:json"`
	ExpiresAt        *time.Time               `gorm:"index"`
	LastUsedAt       *time.Time               `gorm:"index"`
	CreatedBy        string                   `gorm:"size:64;index"`
	RotatedAt        *time.Time               `gorm:"index"`
	Remark           string                   `gorm:"size:255"`
}

func (GameIntegrationKey) TableName() string { return "game_integration_key" }

type AgentGameAccess struct {
	BaseModel
	TenantID       *uint64      `gorm:"index"`
	BrandID        *uint64      `gorm:"index"`
	AgentID        uint64       `gorm:"not null;uniqueIndex:uk_agent_game_access"`
	GameID         uint64       `gorm:"not null;uniqueIndex:uk_agent_game_access"`
	Status         AccessStatus `gorm:"type:varchar(32);not null;default:'enabled';index"`
	GrantedBy      string       `gorm:"size:64"`
	GrantedAt      time.Time    `gorm:"not null;index"`
	EffectiveFrom  time.Time    `gorm:"not null"`
	EffectiveTo    *time.Time   `gorm:"index"`
	SettlementMemo string       `gorm:"size:255"`
}

func (AgentGameAccess) TableName() string { return "agent_game_access" }

type CommissionRule struct {
	BaseModel
	TenantID           *uint64        `gorm:"index"`
	BrandID            *uint64        `gorm:"index"`
	RuleName           string         `gorm:"size:128;not null"`
	Scope              RuleScope      `gorm:"type:varchar(32);not null;index"`
	RuleType           RuleType       `gorm:"type:varchar(32);not null;default:'ratio'"`
	Status             RuleStatus     `gorm:"type:varchar(32);not null;default:'draft';index"`
	Priority           int32          `gorm:"not null;default:0;index"`
	Version            uint32         `gorm:"not null;default:1"`
	AgentID            *uint64        `gorm:"index"`
	GameID             *uint64        `gorm:"index"`
	MaxSettlementDepth uint32         `gorm:"not null;default:1"`
	SettlementRate     float64        `gorm:"type:decimal(10,4);not null;default:1"`
	CommissionRate     float64        `gorm:"type:decimal(10,4);not null;default:0"`
	FixedAmount        float64        `gorm:"type:decimal(18,2);not null;default:0"`
	Currency           string         `gorm:"size:16;not null;default:'CNY'"`
	EffectiveFrom      time.Time      `gorm:"not null;index"`
	EffectiveTo        *time.Time     `gorm:"index"`
	PublishedAt        *time.Time     `gorm:"index"`
	PublishedBy        string         `gorm:"size:64"`
	ConfigPayload      datatypes.JSON `gorm:"type:json"`
	UniqueKey          string         `gorm:"size:128;uniqueIndex"`
	Remark             string         `gorm:"size:255"`
}

func (CommissionRule) TableName() string { return "commission_rule" }

type RuleSnapshot struct {
	BaseModel
	TenantID        *uint64        `gorm:"index"`
	BrandID         *uint64        `gorm:"index"`
	RuleID          uint64         `gorm:"not null;index"`
	RuleVersion     uint32         `gorm:"not null"`
	RuleUniqueKey   string         `gorm:"size:128;not null;index"`
	Scope           RuleScope      `gorm:"type:varchar(32);not null;index"`
	AgentID         *uint64        `gorm:"index"`
	GameID          *uint64        `gorm:"index"`
	SnapshotHash    string         `gorm:"size:128;not null;uniqueIndex"`
	SnapshotPayload datatypes.JSON `gorm:"type:json;not null"`
	EffectiveFrom   time.Time      `gorm:"not null;index"`
	EffectiveTo     *time.Time     `gorm:"index"`
	PublishedAt     time.Time      `gorm:"not null;index"`
	PublishedBy     string         `gorm:"size:64"`
}

func (RuleSnapshot) TableName() string { return "rule_snapshot" }

type RechargeOrder struct {
	BaseModel
	TenantID           *uint64        `gorm:"index"`
	BrandID            *uint64        `gorm:"index"`
	OrderNo            string         `gorm:"size:64;not null;uniqueIndex"`
	ExternalOrderNo    string         `gorm:"size:128;index"`
	PlayerID           uint64         `gorm:"not null;index"`
	GameID             uint64         `gorm:"not null;index"`
	AgentID            *uint64        `gorm:"index"`
	BindingID          *uint64        `gorm:"index"`
	RuleSnapshotID     *uint64        `gorm:"index"`
	Amount             float64        `gorm:"type:decimal(18,2);not null"`
	PaidAmount         float64        `gorm:"type:decimal(18,2);not null;default:0"`
	PaymentChannelCost *float64       `gorm:"type:decimal(18,2)"`
	GrossProfitAmount  *float64       `gorm:"type:decimal(18,2)"`
	Currency           string         `gorm:"size:16;not null;default:'CNY'"`
	Status             OrderStatus    `gorm:"type:varchar(32);not null;default:'pending';index"`
	PaidAt             *time.Time     `gorm:"index"`
	CallbackAt         *time.Time     `gorm:"index"`
	Channel            string         `gorm:"size:64;index"`
	RechargeType       string         `gorm:"size:64;index"`
	ActivityTags       datatypes.JSON `gorm:"type:json"`
	CallbackPayload    datatypes.JSON `gorm:"type:json"`
	IdempotencyKey     string         `gorm:"size:128;uniqueIndex"`
	RiskFlag           string         `gorm:"size:64;index"`
	Remark             string         `gorm:"size:255"`
}

func (RechargeOrder) TableName() string { return "recharge_order" }

type RechargeCallbackLog struct {
	BaseModel
	RechargeOrderID uint64         `gorm:"not null;index"`
	CallbackStatus  CallbackStatus `gorm:"type:varchar(32);not null;default:'received';index"`
	RequestID       string         `gorm:"size:128;index"`
	CallbackSource  string         `gorm:"size:64;index"`
	Signature       string         `gorm:"size:255"`
	VerifiedAt      *time.Time     `gorm:"index"`
	RequestPayload  datatypes.JSON `gorm:"type:json;not null"`
	ResponsePayload datatypes.JSON `gorm:"type:json"`
	ErrorMessage    string         `gorm:"size:255"`
	RemoteIP        string         `gorm:"size:64"`
}

func (RechargeCallbackLog) TableName() string { return "recharge_callback_log" }

type CommissionRecord struct {
	BaseModel
	TenantID             *uint64          `gorm:"index"`
	BrandID              *uint64          `gorm:"index"`
	RecordNo             string           `gorm:"size:64;not null;uniqueIndex"`
	RechargeOrderID      uint64           `gorm:"not null;index"`
	PlayerID             uint64           `gorm:"not null;index"`
	AgentID              uint64           `gorm:"not null;index"`
	SourceAgentID        *uint64          `gorm:"index"`
	GameID               uint64           `gorm:"not null;index"`
	RuleID               *uint64          `gorm:"index"`
	RuleSnapshotID       *uint64          `gorm:"index"`
	SettlementDepth      uint32           `gorm:"not null;default:1"`
	CommissionBaseAmount float64          `gorm:"type:decimal(18,2);not null;default:0"`
	SettlementRate       float64          `gorm:"type:decimal(10,4);not null;default:1"`
	CommissionRate       float64          `gorm:"type:decimal(10,4);not null;default:0"`
	CommissionAmount     float64          `gorm:"type:decimal(18,2);not null;default:0"`
	Currency             string           `gorm:"size:16;not null;default:'CNY'"`
	Status               CommissionStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	EstimatedAt          time.Time        `gorm:"not null;index"`
	SettledAt            *time.Time       `gorm:"index"`
	ReversedFromID       *uint64          `gorm:"index"`
	RelationSnapshot     datatypes.JSON   `gorm:"type:json"`
	Meta                 datatypes.JSON   `gorm:"type:json"`
	Remark               string           `gorm:"size:255"`
}

func (CommissionRecord) TableName() string { return "commission_record" }

type AgentAccount struct {
	BaseModel
	TenantID           *uint64       `gorm:"index"`
	BrandID            *uint64       `gorm:"index"`
	AgentID            uint64        `gorm:"not null;uniqueIndex"`
	AccountNo          string        `gorm:"size:64;not null;uniqueIndex"`
	Status             AccountStatus `gorm:"type:varchar(32);not null;default:'active';index"`
	Currency           string        `gorm:"size:16;not null;default:'CNY'"`
	Balance            float64       `gorm:"type:decimal(18,2);not null;default:0"`
	AvailableBalance   float64       `gorm:"type:decimal(18,2);not null;default:0"`
	FrozenBalance      float64       `gorm:"type:decimal(18,2);not null;default:0"`
	WithdrawableAmount float64       `gorm:"type:decimal(18,2);not null;default:0"`
	TotalIncome        float64       `gorm:"type:decimal(18,2);not null;default:0"`
	TotalReversed      float64       `gorm:"type:decimal(18,2);not null;default:0"`
	LastSettledAt      *time.Time    `gorm:"index"`
	Version            uint64        `gorm:"not null;default:1"`
}

func (AgentAccount) TableName() string { return "agent_account" }

type AgentAccountLedger struct {
	BaseModel
	TenantID       *uint64         `gorm:"index"`
	BrandID        *uint64         `gorm:"index"`
	AccountID      uint64          `gorm:"not null;index"`
	AgentID        uint64          `gorm:"not null;index"`
	ReferenceType  string          `gorm:"size:64;not null;index"`
	ReferenceID    string          `gorm:"size:64;not null;index"`
	LedgerType     LedgerType      `gorm:"type:varchar(32);not null;index"`
	Direction      LedgerDirection `gorm:"type:varchar(16);not null"`
	Amount         float64         `gorm:"type:decimal(18,2);not null"`
	BalanceBefore  float64         `gorm:"type:decimal(18,2);not null;default:0"`
	BalanceAfter   float64         `gorm:"type:decimal(18,2);not null;default:0"`
	FrozenBefore   float64         `gorm:"type:decimal(18,2);not null;default:0"`
	FrozenAfter    float64         `gorm:"type:decimal(18,2);not null;default:0"`
	Currency       string          `gorm:"size:16;not null;default:'CNY'"`
	OccurredAt     time.Time       `gorm:"not null;index"`
	IdempotencyKey string          `gorm:"size:128;uniqueIndex"`
	Remark         string          `gorm:"size:255"`
}

func (AgentAccountLedger) TableName() string { return "agent_account_ledger" }

type WithdrawalRequest struct {
	BaseModel
	TenantID             *uint64          `gorm:"index"`
	BrandID              *uint64          `gorm:"index"`
	RequestNo            string           `gorm:"size:64;not null;uniqueIndex"`
	TenantCode           string           `gorm:"size:64;not null;default:'default';index"`
	AgentID              uint64           `gorm:"not null;index"`
	AccountID            uint64           `gorm:"not null;index"`
	Currency             string           `gorm:"size:16;not null;default:'CNY'"`
	Amount               float64          `gorm:"type:decimal(18,2);not null"`
	FeeAmount            float64          `gorm:"type:decimal(18,2);not null;default:0"`
	TaxAmount            float64          `gorm:"type:decimal(18,2);not null;default:0"`
	PayableAmount        float64          `gorm:"type:decimal(18,2);not null;default:0"`
	Status               WithdrawalStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	Channel              string           `gorm:"size:64;index"`
	BankAccountName      string           `gorm:"size:128"`
	BankAccountNo        string           `gorm:"size:128"`
	BankName             string           `gorm:"size:128"`
	BankAccountSnapshot  datatypes.JSON   `gorm:"type:json"`
	PayoutReference      string           `gorm:"size:128;index"`
	PayoutReceiptPayload datatypes.JSON   `gorm:"type:json"`
	SubmittedBy          string           `gorm:"size:64;index"`
	ReviewedBy           string           `gorm:"size:64;index"`
	ReviewedAt           *time.Time       `gorm:"index"`
	PaidAt               *time.Time       `gorm:"index"`
	ApprovedLedgerID     *uint64          `gorm:"index"`
	CompletedLedgerID    *uint64          `gorm:"index"`
	FailureLedgerID      *uint64          `gorm:"index"`
	IdempotencyKey       string           `gorm:"size:128;uniqueIndex"`
	Remark               string           `gorm:"size:255"`
}

func (WithdrawalRequest) TableName() string { return "withdrawal_request" }
