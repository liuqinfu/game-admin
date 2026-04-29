package model

import (
	"time"

	"gorm.io/datatypes"
)

type ActivityRewardRuleStatus string

const (
	ActivityRewardRuleStatusDraft    ActivityRewardRuleStatus = "draft"
	ActivityRewardRuleStatusActive   ActivityRewardRuleStatus = "active"
	ActivityRewardRuleStatusDisabled ActivityRewardRuleStatus = "disabled"
)

type ActivityRewardRecordStatus string

const (
	ActivityRewardRecordStatusPending  ActivityRewardRecordStatus = "pending"
	ActivityRewardRecordStatusGranted  ActivityRewardRecordStatus = "granted"
	ActivityRewardRecordStatusFailed   ActivityRewardRecordStatus = "failed"
	ActivityRewardRecordStatusReversed ActivityRewardRecordStatus = "reversed"
)

type ActivityRewardRule struct {
	BaseModel
	TenantID      *uint64                  `gorm:"index"`
	BrandID       *uint64                  `gorm:"index"`
	Name          string                   `gorm:"size:128;not null;index"`
	ActivityType  string                   `gorm:"size:64;not null;index"`
	RewardType    string                   `gorm:"size:64;not null;index"`
	Status        ActivityRewardRuleStatus `gorm:"type:varchar(32);not null;default:'draft';index"`
	RewardValue   float64                  `gorm:"type:decimal(18,2);not null;default:0"`
	Currency      string                   `gorm:"size:16;not null;default:'CNY'"`
	TriggerValue  float64                  `gorm:"type:decimal(18,2);not null;default:0"`
	DailyLimit    uint64                   `gorm:"not null;default:0"`
	TotalLimit    uint64                   `gorm:"not null;default:0"`
	StartAt       *time.Time               `gorm:"index"`
	EndAt         *time.Time               `gorm:"index"`
	ConfigPayload datatypes.JSON           `gorm:"type:json"`
	Remark        string                   `gorm:"size:255"`
}

func (ActivityRewardRule) TableName() string { return "activity_reward_rule" }

type ActivityRewardRecord struct {
	BaseModel
	RecordNo        string                     `gorm:"size:64;not null;uniqueIndex"`
	RuleID          uint64                     `gorm:"not null;index"`
	TenantID        *uint64                    `gorm:"index"`
	BrandID         *uint64                    `gorm:"index"`
	AgentID         *uint64                    `gorm:"index"`
	PlayerID        *uint64                    `gorm:"index"`
	ActivityType    string                     `gorm:"size:64;not null;index"`
	RewardType      string                     `gorm:"size:64;not null;index"`
	Status          ActivityRewardRecordStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	RewardValue     float64                    `gorm:"type:decimal(18,2);not null;default:0"`
	Currency        string                     `gorm:"size:16;not null;default:'CNY'"`
	ReferenceType   string                     `gorm:"size:64;index"`
	ReferenceID     string                     `gorm:"size:64;index"`
	RuleName        string                     `gorm:"size:128;index"`
	SnapshotPayload datatypes.JSON             `gorm:"type:json"`
	GrantedAt       *time.Time                 `gorm:"index"`
	Remark          string                     `gorm:"size:255"`
}

func (ActivityRewardRecord) TableName() string { return "activity_reward_record" }

type SettlementBill struct {
	BaseModel
	TenantID         *uint64              `gorm:"index"`
	BrandID          *uint64              `gorm:"index"`
	BillNo           string               `gorm:"size:64;not null;uniqueIndex"`
	AgentID          uint64               `gorm:"not null;index"`
	PeriodStart      time.Time            `gorm:"not null;index"`
	PeriodEnd        time.Time            `gorm:"not null;index"`
	Currency         string               `gorm:"size:16;not null;default:'CNY'"`
	Status           SettlementBillStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	CommissionAmount float64              `gorm:"type:decimal(18,2);not null;default:0"`
	AdjustmentAmount float64              `gorm:"type:decimal(18,2);not null;default:0"`
	PayableAmount    float64              `gorm:"type:decimal(18,2);not null;default:0"`
	GeneratedAt      *time.Time           `gorm:"index"`
	ConfirmedAt      *time.Time           `gorm:"index"`
	ConfirmedBy      string               `gorm:"size:64"`
	SourceTaskID     *uint64              `gorm:"index"`
	SummaryPayload   datatypes.JSON       `gorm:"type:json"`
	Remark           string               `gorm:"size:255"`
}

func (SettlementBill) TableName() string { return "settlement_bill" }

type SettlementBillDetail struct {
	BaseModel
	TenantID           *uint64        `gorm:"index"`
	BrandID            *uint64        `gorm:"index"`
	SettlementBillID   uint64         `gorm:"not null;index"`
	AgentID            uint64         `gorm:"not null;index"`
	CommissionRecordID *uint64        `gorm:"index"`
	RechargeOrderID    *uint64        `gorm:"index"`
	ReferenceType      string         `gorm:"size:64;not null;index"`
	ReferenceID        string         `gorm:"size:64;not null;index"`
	CommissionAmount   float64        `gorm:"type:decimal(18,2);not null;default:0"`
	AdjustmentAmount   float64        `gorm:"type:decimal(18,2);not null;default:0"`
	Amount             float64        `gorm:"type:decimal(18,2);not null;default:0"`
	Currency           string         `gorm:"size:16;not null;default:'CNY'"`
	OccurredAt         time.Time      `gorm:"not null;index"`
	SnapshotPayload    datatypes.JSON `gorm:"type:json"`
	Remark             string         `gorm:"size:255"`
}

func (SettlementBillDetail) TableName() string { return "settlement_bill_detail" }

type RecalculationTask struct {
	BaseModel
	TenantID         *uint64                 `gorm:"index"`
	BrandID          *uint64                 `gorm:"index"`
	TaskNo           string                  `gorm:"size:64;not null;uniqueIndex"`
	TaskType         RecalculationTaskType   `gorm:"type:varchar(32);not null;default:'settlement_bill';index"`
	Status           RecalculationTaskStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	AgentID          *uint64                 `gorm:"index"`
	SettlementBillID *uint64                 `gorm:"index"`
	PeriodStart      *time.Time              `gorm:"index"`
	PeriodEnd        *time.Time              `gorm:"index"`
	RequestedBy      string                  `gorm:"size:64;index"`
	StartedAt        *time.Time              `gorm:"index"`
	CompletedAt      *time.Time              `gorm:"index"`
	ResultSummary    datatypes.JSON          `gorm:"type:json"`
	ErrorMessage     string                  `gorm:"size:255"`
	Remark           string                  `gorm:"size:255"`
}

func (RecalculationTask) TableName() string { return "recalculation_task" }

type OperationAuditLog struct {
	BaseModel
	OperatorID    string         `gorm:"size:64;not null;index"`
	OperatorName  string         `gorm:"size:128"`
	OperatorRole  string         `gorm:"size:64;index"`
	Module        AuditModule    `gorm:"type:varchar(32);not null;index"`
	Action        string         `gorm:"size:64;not null;index"`
	TargetType    string         `gorm:"size:64;not null;index"`
	TargetID      string         `gorm:"size:64;not null;index"`
	RequestID     string         `gorm:"size:128;index"`
	TraceID       string         `gorm:"size:128;index"`
	Result        AuditResult    `gorm:"type:varchar(32);not null;default:'success';index"`
	IP            string         `gorm:"size:64"`
	UserAgent     string         `gorm:"size:255"`
	BeforePayload datatypes.JSON `gorm:"type:json"`
	AfterPayload  datatypes.JSON `gorm:"type:json"`
	DiffPayload   datatypes.JSON `gorm:"type:json"`
	ErrorMessage  string         `gorm:"size:255"`
	OccurredAt    time.Time      `gorm:"not null;index"`
}

func (OperationAuditLog) TableName() string { return "operation_audit_log" }

type RiskCaseStatus string

const (
	RiskCaseStatusPending   RiskCaseStatus = "pending"
	RiskCaseStatusReleased  RiskCaseStatus = "released"
	RiskCaseStatusConfirmed RiskCaseStatus = "confirmed"
)

type RiskCase struct {
	BaseModel
	TenantID        *uint64        `gorm:"index"`
	BrandID         *uint64        `gorm:"index"`
	CaseNo          string         `gorm:"size:64;not null;uniqueIndex"`
	AgentID         uint64         `gorm:"not null;index"`
	Reason          string         `gorm:"size:255"`
	Status          RiskCaseStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	FreezeRequested bool           `gorm:"not null;default:false"`
	Amount          float64        `gorm:"type:decimal(18,2);not null;default:0"`
	Currency        string         `gorm:"size:16;not null;default:'CNY'"`
	Remark          string         `gorm:"size:255"`
	ReviewedAt      *time.Time     `gorm:"index"`
	ReviewedBy      string         `gorm:"size:64;index"`
	ReviewRemark    string         `gorm:"size:255"`
}

func (RiskCase) TableName() string { return "risk_case" }

type PlatformConfig struct {
	BaseModel
	TenantID    *uint64        `gorm:"index"`
	BrandID     *uint64        `gorm:"index"`
	Key         string         `gorm:"size:128;not null;index:uk_platform_config_scope_key,unique"`
	Value       datatypes.JSON `gorm:"type:json;not null"`
	Description string         `gorm:"size:255"`
	UpdatedBy   string         `gorm:"size:64;index"`
}

func (PlatformConfig) TableName() string { return "platform_config" }

type DomainEvent struct {
	BaseModel
	Topic          string         `gorm:"size:128;not null;index"`
	EventType      string         `gorm:"size:128;not null;index"`
	AggregateType  string         `gorm:"size:64;not null;index"`
	AggregateID    string         `gorm:"size:128;not null;index"`
	TenantID       *uint64        `gorm:"index"`
	BrandID        *uint64        `gorm:"index"`
	OccurredAt     time.Time      `gorm:"not null;index"`
	Producer       string         `gorm:"size:64;index"`
	IdempotencyKey string         `gorm:"size:160;index"`
	Payload        datatypes.JSON `gorm:"type:json;not null"`
	Meta           datatypes.JSON `gorm:"type:json"`
}

func (DomainEvent) TableName() string { return "domain_event" }

type DomainEventDelivery struct {
	BaseModel
	EventID             uint64                    `gorm:"not null;uniqueIndex:uk_domain_event_delivery_consumer;index"`
	Consumer            string                    `gorm:"size:64;not null;uniqueIndex:uk_domain_event_delivery_consumer;index"`
	Status              DomainEventDeliveryStatus `gorm:"type:varchar(32);not null;default:'pending';index"`
	Attempts            uint32                    `gorm:"not null;default:0"`
	MaxAttempts         uint32                    `gorm:"not null;default:20"`
	AvailableAt         time.Time                 `gorm:"not null;index"`
	LockedAt            *time.Time                `gorm:"index"`
	LockedBy            string                    `gorm:"size:128"`
	LastError           string                    `gorm:"size:255"`
	ResultPayload       datatypes.JSON            `gorm:"type:json"`
	DeliveredAt         *time.Time                `gorm:"index"`
	LastAttemptedAt     *time.Time                `gorm:"index"`
	LastPublishedAt     *time.Time                `gorm:"index"`
	DeadLetteredAt      *time.Time                `gorm:"index"`
	DeadLetterReason    string                    `gorm:"size:255"`
	DeadLetterPayload   datatypes.JSON            `gorm:"type:json"`
	SourceTransport     string                    `gorm:"size:64;index"`
	LastWorkerRequestID string                    `gorm:"size:128;index"`
	LastWorkerTraceID   string                    `gorm:"size:128;index"`
}

func (DomainEventDelivery) TableName() string { return "domain_event_delivery" }
