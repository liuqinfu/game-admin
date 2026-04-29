package http

import (
	"time"

	"game-admin/backend/internal/domain/model"
)

type activityRewardRulePayload struct {
	TenantID     *uint64                        `json:"tenantID"`
	BrandID      *uint64                        `json:"brandID"`
	Name         string                         `json:"name"`
	ActivityType string                         `json:"activityType"`
	RewardType   string                         `json:"rewardType"`
	Status       model.ActivityRewardRuleStatus `json:"status"`
	RewardValue  float64                        `json:"rewardValue"`
	Currency     string                         `json:"currency"`
	TriggerValue float64                        `json:"triggerValue"`
	DailyLimit   uint64                         `json:"dailyLimit"`
	TotalLimit   uint64                         `json:"totalLimit"`
	StartAt      string                         `json:"startAt"`
	EndAt        string                         `json:"endAt"`
	Remark       string                         `json:"remark"`
}

type activityRewardRuleStatusPayload struct {
	Status model.ActivityRewardRuleStatus `json:"status"`
}

type activityRewardRecordGeneratePayload struct {
	PlayerID      *uint64 `json:"playerID"`
	UserID        *uint64 `json:"userID"`
	AgentID       *uint64 `json:"agentID"`
	ReferenceType string  `json:"referenceType"`
	ReferenceID   string  `json:"referenceID"`
	Remark        string  `json:"remark"`
}

type activityRewardRecordReversePayload struct {
	Remark string `json:"remark"`
}

type activityRewardRecordView struct {
	ID           uint64                           `json:"id"`
	RecordNo     string                           `json:"recordNo"`
	RuleID       uint64                           `json:"ruleID"`
	RuleName     string                           `json:"ruleName,omitempty"`
	UserID       *uint64                          `json:"userID,omitempty"`
	PlayerID     *uint64                          `json:"playerID,omitempty"`
	AgentID      *uint64                          `json:"agentID,omitempty"`
	RewardType   string                           `json:"rewardType"`
	RewardValue  float64                          `json:"rewardValue"`
	Currency     string                           `json:"currency"`
	ActivityType string                           `json:"activityType,omitempty"`
	Status       model.ActivityRewardRecordStatus `json:"status"`
	GrantedAt    *time.Time                       `json:"grantedAt,omitempty"`
	CreatedAt    time.Time                        `json:"createdAt"`
	UpdatedAt    time.Time                        `json:"updatedAt"`
	Remark       string                           `json:"remark,omitempty"`
}

type gameStatusPayload struct {
	Status model.GameStatus `json:"status"`
}

type rulePayload struct {
	TenantID           *uint64          `json:"tenantID"`
	BrandID            *uint64          `json:"brandID"`
	RuleName           string           `json:"ruleName"`
	Scope              model.RuleScope  `json:"scope"`
	RuleType           model.RuleType   `json:"ruleType"`
	Status             model.RuleStatus `json:"status"`
	Priority           int32            `json:"priority"`
	Version            uint32           `json:"version"`
	AgentID            *uint64          `json:"agentID"`
	GameID             *uint64          `json:"gameID"`
	MaxSettlementDepth uint32           `json:"maxSettlementDepth"`
	SettlementRate     float64          `json:"settlementRate"`
	CommissionRate     float64          `json:"commissionRate"`
	FixedAmount        float64          `json:"fixedAmount"`
	MinAgentLevel      uint32           `json:"minAgentLevel"`
	RechargeTypes      []string         `json:"rechargeTypes"`
	ActivityTags       []string         `json:"activityTags"`
	CapAmount          float64          `json:"capAmount"`
	Currency           string           `json:"currency"`
	EffectiveFrom      string           `json:"effectiveFrom"`
	Remark             string           `json:"remark"`
}

type rulePublishPayload struct {
	PublishedBy string `json:"publishedBy"`
}

type rechargeCallbackPayload struct {
	OrderNo            string   `json:"orderNo"`
	ExternalOrderNo    string   `json:"externalOrderNo"`
	PlayerID           uint64   `json:"playerID"`
	GameID             uint64   `json:"gameID"`
	Amount             float64  `json:"amount"`
	PaidAmount         float64  `json:"paidAmount"`
	PaymentChannelCost *float64 `json:"paymentChannelCost"`
	GrossProfitAmount  *float64 `json:"grossProfitAmount"`
	Currency           string   `json:"currency"`
	Status             string   `json:"status"`
	PaidAt             string   `json:"paidAt"`
	CallbackAt         string   `json:"callbackAt"`
	Channel            string   `json:"channel"`
	RechargeType       string   `json:"rechargeType"`
	RequestID          string   `json:"requestID"`
	CallbackSource     string   `json:"callbackSource"`
	Signature          string   `json:"signature"`
	IdempotencyKey     string   `json:"idempotencyKey"`
	ActivityTags       []string `json:"activityTags"`
	Remark             string   `json:"remark"`
}

type orderProfitFactsPayload struct {
	PaidAmount         *float64 `json:"paidAmount"`
	PaymentChannelCost *float64 `json:"paymentChannelCost"`
	GrossProfitAmount  *float64 `json:"grossProfitAmount"`
	Remark             string   `json:"remark"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type rechargeCallbackResult struct {
	Order       model.RechargeOrder `json:"order"`
	Commissions int                 `json:"commissions"`
	Ledgers     int                 `json:"ledgers"`
	Duplicate   bool                `json:"duplicate"`
	Frozen      bool                `json:"frozen"`
}

type enumDictionaryItem struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	LabelEn     string `json:"labelEn,omitempty"`
	Description string `json:"description,omitempty"`
}

type enumDictionaryConfig struct {
	Strict bool                 `json:"strict"`
	Items  []enumDictionaryItem `json:"items"`
}

type enumDictionaryResponse struct {
	Code   string               `json:"code"`
	Key    string               `json:"key"`
	Strict bool                 `json:"strict"`
	Items  []enumDictionaryItem `json:"items"`
}
