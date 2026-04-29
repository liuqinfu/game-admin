package model

import (
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type AgentStatus string

type InviteCodeStatus string

type BindingStatus string

type BindingSource string

type AgentInviteApplicationStatus string

type RelationStatus string

type RelationType string

type GameStatus string

type GameIntegrationKeyStatus string

type AccessStatus string

type RuleScope string

type RuleStatus string

type RuleType string

type OrderStatus string

type CallbackStatus string

type CommissionStatus string

type AccountStatus string

type LedgerType string

type LedgerDirection string

type WithdrawalStatus string

type SettlementBillStatus string

type RecalculationTaskStatus string

type RecalculationTaskType string

type AuditModule string

type AuditResult string

type AdminUserStatus string

type TenantStatus string

type BrandStatus string

type DomainEventDeliveryStatus string

const (
	AgentStatusPending  AgentStatus = "pending"
	AgentStatusActive   AgentStatus = "active"
	AgentStatusFrozen   AgentStatus = "frozen"
	AgentStatusDisabled AgentStatus = "disabled"
	AgentStatusClosed   AgentStatus = "closed"
)

const (
	InviteCodeStatusPending  InviteCodeStatus = "pending"
	InviteCodeStatusActive   InviteCodeStatus = "active"
	InviteCodeStatusDisabled InviteCodeStatus = "disabled"
	InviteCodeStatusExpired  InviteCodeStatus = "expired"
)

const (
	BindingStatusBound    BindingStatus = "bound"
	BindingStatusPending  BindingStatus = "pending_change"
	BindingStatusChanged  BindingStatus = "changed"
	BindingStatusReleased BindingStatus = "released"
)

const (
	BindingSourceRegister BindingSource = "register"
	BindingSourceManual   BindingSource = "manual"
	BindingSourceBackfill BindingSource = "backfill"
)

const (
	AgentInviteApplicationStatusPending  AgentInviteApplicationStatus = "pending"
	AgentInviteApplicationStatusApproved AgentInviteApplicationStatus = "approved"
	AgentInviteApplicationStatusRejected AgentInviteApplicationStatus = "rejected"
)

const (
	RelationStatusActive   RelationStatus = "active"
	RelationStatusInactive RelationStatus = "inactive"
)

const (
	RelationTypeDirect  RelationType = "direct"
	RelationTypeClosure RelationType = "closure"
)

const (
	GameStatusDraft    GameStatus = "draft"
	GameStatusOnline   GameStatus = "online"
	GameStatusOffline  GameStatus = "offline"
	GameStatusArchived GameStatus = "archived"
)

const (
	GameIntegrationKeyStatusActive   GameIntegrationKeyStatus = "active"
	GameIntegrationKeyStatusDisabled GameIntegrationKeyStatus = "disabled"
	GameIntegrationKeyStatusRevoked  GameIntegrationKeyStatus = "revoked"
)

const (
	AccessStatusEnabled  AccessStatus = "enabled"
	AccessStatusDisabled AccessStatus = "disabled"
)

const (
	RuleScopePlatform  RuleScope = "platform"
	RuleScopeGame      RuleScope = "game"
	RuleScopeAgent     RuleScope = "agent"
	RuleScopeAgentGame RuleScope = "agent_game"
)

const (
	RuleStatusDraft     RuleStatus = "draft"
	RuleStatusPublished RuleStatus = "published"
	RuleStatusDisabled  RuleStatus = "disabled"
)

const (
	RuleTypeRatio        RuleType = "ratio"
	RuleTypeFixedShare   RuleType = "fixed_share"
	RuleTypeDifferential RuleType = "differential"
	RuleTypePoint        RuleType = "point"
	RuleTypeCapped       RuleType = "capped"
)

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusFailed    OrderStatus = "failed"
	OrderStatusRefunded  OrderStatus = "refunded"
	OrderStatusCancelled OrderStatus = "cancelled"
)

const (
	CallbackStatusReceived CallbackStatus = "received"
	CallbackStatusVerified CallbackStatus = "verified"
	CallbackStatusRejected CallbackStatus = "rejected"
)

const (
	CommissionStatusPending  CommissionStatus = "pending"
	CommissionStatusFrozen   CommissionStatus = "frozen"
	CommissionStatusSettled  CommissionStatus = "settled"
	CommissionStatusReversed CommissionStatus = "reversed"
	CommissionStatusVoided   CommissionStatus = "voided"
)

const (
	AccountStatusActive AccountStatus = "active"
	AccountStatusFrozen AccountStatus = "frozen"
	AccountStatusClosed AccountStatus = "closed"
)

const (
	LedgerTypeIncome   LedgerType = "income"
	LedgerTypeFreeze   LedgerType = "freeze"
	LedgerTypeUnfreeze LedgerType = "unfreeze"
	LedgerTypeDebit    LedgerType = "debit"
	LedgerTypeReverse  LedgerType = "reverse"
	LedgerTypeAdjust   LedgerType = "adjust"
)

const (
	LedgerDirectionCredit LedgerDirection = "credit"
	LedgerDirectionDebit  LedgerDirection = "debit"
)

const (
	WithdrawalStatusPending   WithdrawalStatus = "pending"
	WithdrawalStatusApproved  WithdrawalStatus = "approved"
	WithdrawalStatusPaying    WithdrawalStatus = "paying"
	WithdrawalStatusPaid      WithdrawalStatus = "paid"
	WithdrawalStatusFailed    WithdrawalStatus = "failed"
	WithdrawalStatusReturned  WithdrawalStatus = "returned"
	WithdrawalStatusRejected  WithdrawalStatus = "rejected"
	WithdrawalStatusClosed    WithdrawalStatus = "closed"
	WithdrawalStatusCancelled WithdrawalStatus = "cancelled"
)

const (
	SettlementBillStatusPending   SettlementBillStatus = "pending"
	SettlementBillStatusGenerated SettlementBillStatus = "generated"
	SettlementBillStatusConfirmed SettlementBillStatus = "confirmed"
	SettlementBillStatusCancelled SettlementBillStatus = "cancelled"
)

const (
	RecalculationTaskStatusPending    RecalculationTaskStatus = "pending"
	RecalculationTaskStatusProcessing RecalculationTaskStatus = "processing"
	RecalculationTaskStatusCompleted  RecalculationTaskStatus = "completed"
	RecalculationTaskStatusFailed     RecalculationTaskStatus = "failed"
)

const (
	RecalculationTaskTypeSettlementBill RecalculationTaskType = "settlement_bill"
	RecalculationTaskTypeCommission     RecalculationTaskType = "commission"
)

const (
	AuditModuleAgent      AuditModule = "agent"
	AuditModuleBinding    AuditModule = "binding"
	AuditModuleGame       AuditModule = "game"
	AuditModuleRule       AuditModule = "rule"
	AuditModuleCommission AuditModule = "commission"
	AuditModuleAccount    AuditModule = "account"
)

const (
	AuditResultSuccess AuditResult = "success"
	AuditResultFailed  AuditResult = "failed"
)

const (
	AdminUserStatusActive   AdminUserStatus = "active"
	AdminUserStatusDisabled AdminUserStatus = "disabled"
)

const (
	TenantStatusActive   TenantStatus = "active"
	TenantStatusDisabled TenantStatus = "disabled"
)

const (
	BrandStatusActive   BrandStatus = "active"
	BrandStatusDisabled BrandStatus = "disabled"
)

const (
	DomainEventDeliveryStatusPending    DomainEventDeliveryStatus = "pending"
	DomainEventDeliveryStatusProcessing DomainEventDeliveryStatus = "processing"
	DomainEventDeliveryStatusQueued     DomainEventDeliveryStatus = "queued"
	DomainEventDeliveryStatusSucceeded  DomainEventDeliveryStatus = "succeeded"
	DomainEventDeliveryStatusFailed     DomainEventDeliveryStatus = "failed"
	DomainEventDeliveryStatusDeadLetter DomainEventDeliveryStatus = "dead_letter"
)
