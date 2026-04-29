package http

import (
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/datatypes"
)

type settlementBillCreatePayload struct {
	AgentID     uint64  `json:"agentID"`
	PeriodStart string  `json:"periodStart"`
	PeriodEnd   string  `json:"periodEnd"`
	Currency    string  `json:"currency"`
	Remark      string  `json:"remark"`
	Adjustment  float64 `json:"adjustment"`
	Cycle       string  `json:"cycle"`
}

type settlementBillDetailItem struct {
	model.SettlementBillDetail
	RecordNo string `json:"recordNo,omitempty"`
	OrderNo  string `json:"orderNo,omitempty"`
}

type settlementBillListItem struct {
	model.SettlementBill
	AgentName string                     `json:"agentName"`
	Details   []settlementBillDetailItem `json:"details,omitempty"`
}

type recalculationTaskCreatePayload struct {
	TaskType         model.RecalculationTaskType `json:"taskType"`
	AgentID          *uint64                     `json:"agentID"`
	SettlementBillID *uint64                     `json:"settlementBillID"`
	PeriodStart      string                      `json:"periodStart"`
	PeriodEnd        string                      `json:"periodEnd"`
	Operator         string                      `json:"operator"`
	Reason           string                      `json:"reason"`
	Scope            string                      `json:"scope"`
	Remark           string                      `json:"remark"`
}

type recalculationTaskListItem struct {
	model.RecalculationTask
	AgentName string `json:"agentName,omitempty"`
	BillNo    string `json:"billNo,omitempty"`
}

type agentAccountLedgerCreatePayload struct {
	AgentID        uint64           `json:"agentID"`
	ReferenceType  string           `json:"referenceType"`
	ReferenceID    string           `json:"referenceID"`
	LedgerType     model.LedgerType `json:"ledgerType"`
	Amount         float64          `json:"amount"`
	Currency       string           `json:"currency"`
	OccurredAt     string           `json:"occurredAt"`
	IdempotencyKey string           `json:"idempotencyKey"`
	Remark         string           `json:"remark"`
}

type withdrawalCreatePayload struct {
	TenantCode      string  `json:"tenantCode"`
	TenantID        *uint64 `json:"tenantID"`
	BrandID         *uint64 `json:"brandID"`
	AgentID         uint64  `json:"agentID"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	Channel         string  `json:"channel"`
	BankAccountName string  `json:"bankAccountName"`
	BankAccountNo   string  `json:"bankAccountNo"`
	BankName        string  `json:"bankName"`
	IdempotencyKey  string  `json:"idempotencyKey"`
	Remark          string  `json:"remark"`
}

type withdrawalReviewPayload struct {
	Action string `json:"action"`
	Remark string `json:"remark"`
}

type withdrawalPayoutPayload struct {
	Action         string         `json:"action"`
	Remark         string         `json:"remark"`
	Reference      string         `json:"reference"`
	ReceiptPayload map[string]any `json:"receiptPayload"`
}

type withdrawalListItem struct {
	model.WithdrawalRequest
	AgentName string `json:"agentName"`
}

type riskCasePayload struct {
	CaseNo            string  `json:"caseNo"`
	TenantID          *uint64 `json:"tenantID"`
	BrandID           *uint64 `json:"brandID"`
	AgentID           uint64  `json:"agentID"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	Reason            string  `json:"reason"`
	Freeze            bool    `json:"freeze"`
	FreezeIdempotency string  `json:"freezeIdempotencyKey"`
	Remark            string  `json:"remark"`
}

type riskCaseReviewPayload struct {
	Action string `json:"action"`
	Remark string `json:"remark"`
}

type riskCaseResponse struct {
	CaseNo       string                    `json:"caseNo"`
	Status       string                    `json:"status"`
	Frozen       bool                      `json:"frozen,omitempty"`
	FrozenLedger *model.AgentAccountLedger `json:"frozenLedger,omitempty"`
	Action       string                    `json:"action,omitempty"`
	Ledger       *model.AgentAccountLedger `json:"ledger,omitempty"`
}

type riskCaseListItem struct {
	CaseNo                    string    `json:"caseNo"`
	Status                    string    `json:"status"`
	AgentID                   uint64    `json:"agentID"`
	AgentName                 string    `json:"agentName"`
	Currency                  string    `json:"currency"`
	RiskLevel                 string    `json:"riskLevel"`
	Reason                    string    `json:"reason"`
	FreezeRequested           bool      `json:"freezeRequested"`
	FrozenBalance             float64   `json:"frozenBalance"`
	WithdrawableAmount        float64   `json:"withdrawableAmount"`
	FrozenRatio               float64   `json:"frozenRatio"`
	LatestWithdrawalRequestID *uint64   `json:"latestWithdrawalRequestID,omitempty"`
	LatestWithdrawalRequestNo string    `json:"latestWithdrawalRequestNo,omitempty"`
	LatestWithdrawalAmount    *float64  `json:"latestWithdrawalAmount,omitempty"`
	RiskNote                  string    `json:"riskNote,omitempty"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

type platformConfigPayload struct {
	TenantID    *uint64        `json:"tenantID"`
	BrandID     *uint64        `json:"brandID"`
	Key         string         `json:"key"`
	Value       map[string]any `json:"value"`
	Description string         `json:"description"`
}

type platformConfigListItem struct {
	ID          uint64         `json:"id"`
	TenantID    *uint64        `json:"tenantID,omitempty"`
	TenantCode  string         `json:"tenantCode,omitempty"`
	BrandID     *uint64        `json:"brandID,omitempty"`
	BrandCode   string         `json:"brandCode,omitempty"`
	Key         string         `json:"key"`
	Value       datatypes.JSON `json:"value"`
	Description string         `json:"description"`
}

type agentAccountRiskListItem struct {
	model.AgentAccount
	AgentName   string  `json:"agentName"`
	RiskLevel   string  `json:"riskLevel"`
	FrozenRatio float64 `json:"frozenRatio"`
}

type agentPerformanceReportItem struct {
	AgentID            uint64  `json:"agentID"`
	AgentName          string  `json:"agentName"`
	Currency           string  `json:"currency"`
	Balance            float64 `json:"balance"`
	FrozenBalance      float64 `json:"frozenBalance"`
	WithdrawableAmount float64 `json:"withdrawableAmount"`
	TotalEntries       int64   `json:"totalEntries"`
	IncomeAmount       float64 `json:"incomeAmount"`
	FreezeAmount       float64 `json:"freezeAmount"`
	UnfreezeAmount     float64 `json:"unfreezeAmount"`
	DebitAmount        float64 `json:"debitAmount"`
	ReverseAmount      float64 `json:"reverseAmount"`
	AdjustAmount       float64 `json:"adjustAmount"`
}

type gameSettlementReportItem struct {
	Currency              string  `json:"currency"`
	TotalBills            int64   `json:"totalBills"`
	ConfirmedBills        int64   `json:"confirmedBills"`
	PendingBills          int64   `json:"pendingBills"`
	GeneratedBills        int64   `json:"generatedBills"`
	CancelledBills        int64   `json:"cancelledBills"`
	TotalCommissionAmount float64 `json:"totalCommissionAmount"`
	TotalAdjustmentAmount float64 `json:"totalAdjustmentAmount"`
	TotalPayableAmount    float64 `json:"totalPayableAmount"`
}

type teamPerformanceReportItem struct {
	AgentID                 uint64  `json:"agentID"`
	AgentName               string  `json:"agentName"`
	Level                   uint32  `json:"level"`
	Currency                string  `json:"currency"`
	DirectDescendants       int64   `json:"directDescendants"`
	TotalDescendants        int64   `json:"totalDescendants"`
	ActiveDescendants       int64   `json:"activeDescendants"`
	LeafDescendants         int64   `json:"leafDescendants"`
	BoundPlayers            int64   `json:"boundPlayers"`
	TotalTeamBalance        float64 `json:"totalTeamBalance"`
	TotalTeamFrozenBalance  float64 `json:"totalTeamFrozenBalance"`
	TotalWithdrawableAmount float64 `json:"totalWithdrawableAmount"`
	PendingWithdrawals      int64   `json:"pendingWithdrawals"`
	PendingWithdrawalAmount float64 `json:"pendingWithdrawalAmount"`
	ConfirmedBills          int64   `json:"confirmedBills"`
	ConfirmedBillAmount     float64 `json:"confirmedBillAmount"`
}

type settlementProgressReportItem struct {
	Currency                string  `json:"currency"`
	TotalBills              int64   `json:"totalBills"`
	GeneratedBills          int64   `json:"generatedBills"`
	ConfirmedBills          int64   `json:"confirmedBills"`
	CancelledBills          int64   `json:"cancelledBills"`
	ProgressPercent         float64 `json:"progressPercent"`
	PendingCommissionAmount float64 `json:"pendingCommissionAmount"`
	PendingPayableAmount    float64 `json:"pendingPayableAmount"`
	CompletedPayableAmount  float64 `json:"completedPayableAmount"`
	LastGeneratedAt         string  `json:"lastGeneratedAt,omitempty"`
	LastConfirmedAt         string  `json:"lastConfirmedAt,omitempty"`
}

type dataPlatformLayerItem struct {
	Layer        string `json:"layer"`
	Status       string `json:"status"`
	SyncMode     string `json:"syncMode"`
	Description  string `json:"description"`
	TableCount   int    `json:"tableCount"`
	RecordCount  int64  `json:"recordCount"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
}

type dataPlatformMetricItem struct {
	Key         string  `json:"key"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit,omitempty"`
	Description string  `json:"description,omitempty"`
}

type profitabilityConfig struct {
	GrossMarginRate        float64 `json:"grossMarginRate"`
	PaymentChannelCostRate float64 `json:"paymentChannelCostRate"`
	TargetNetProfitRate    float64 `json:"targetNetProfitRate"`
}

type riskIntelligenceItem struct {
	SourceType        string  `json:"sourceType"`
	SourceID          string  `json:"sourceID"`
	AgentID           uint64  `json:"agentID,omitempty"`
	AgentName         string  `json:"agentName,omitempty"`
	RiskLevel         string  `json:"riskLevel"`
	Score             float64 `json:"score"`
	Reason            string  `json:"reason"`
	RecommendedAction string  `json:"recommendedAction"`
	Intercepted       bool    `json:"intercepted"`
	Status            string  `json:"status,omitempty"`
	CreatedAt         string  `json:"createdAt"`
}

type agentHierarchyNode struct {
	AncestorAgentID   uint64               `json:"ancestorAgentID,omitempty"`
	DescendantAgentID uint64               `json:"descendantAgentID,omitempty"`
	Depth             uint32               `json:"depth"`
	RelationType      model.RelationType   `json:"relationType"`
	Status            model.RelationStatus `json:"status"`
	ViaDirectParentID *uint64              `json:"viaDirectParentID,omitempty"`
}

type agentTeamStatsResponse struct {
	AgentID            uint64                   `json:"agentID"`
	DirectDescendants  int64                    `json:"directDescendants"`
	TotalDescendants   int64                    `json:"totalDescendants"`
	TotalAncestors     int64                    `json:"totalAncestors"`
	MaxDescendantDepth uint32                   `json:"maxDescendantDepth"`
	MaxAncestorDepth   uint32                   `json:"maxAncestorDepth"`
	LeafDescendants    int64                    `json:"leafDescendants"`
	DepthBreakdown     []agentDepthStatResponse `json:"depthBreakdown,omitempty"`
}

type agentDepthStatResponse struct {
	Depth uint32 `json:"depth"`
	Count int64  `json:"count"`
}
