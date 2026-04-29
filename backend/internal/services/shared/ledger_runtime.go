package shared

import (
	"errors"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

type LedgerReversePolicy string

const (
	LedgerReversePolicyConditional LedgerReversePolicy = "conditional"
	LedgerReversePolicyAlways      LedgerReversePolicy = "always"
)

type AgentLedgerInput struct {
	AgentID        uint64
	ReferenceType  string
	ReferenceID    string
	LedgerType     model.LedgerType
	Amount         float64
	Currency       string
	OccurredAt     string
	IdempotencyKey string
	Remark         string
	ReversePolicy  LedgerReversePolicy
}

func CreateAgentAccountLedger(db *gorm.DB, input AgentLedgerInput) (model.AgentAccountLedger, error) {
	if input.AgentID == 0 {
		return model.AgentAccountLedger{}, errors.New("agentID is required")
	}
	if strings.TrimSpace(input.ReferenceType) == "" {
		return model.AgentAccountLedger{}, errors.New("referenceType is required")
	}
	if strings.TrimSpace(input.ReferenceID) == "" {
		return model.AgentAccountLedger{}, errors.New("referenceID is required")
	}
	if input.Amount <= 0 {
		return model.AgentAccountLedger{}, errors.New("amount must be greater than 0")
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return model.AgentAccountLedger{}, errors.New("idempotencyKey is required")
	}
	if input.LedgerType == "" {
		return model.AgentAccountLedger{}, errors.New("ledgerType is required")
	}
	if err := ValidateAgentLedgerType(input.LedgerType); err != nil {
		return model.AgentAccountLedger{}, err
	}
	occurredAt, err := ParseOptionalTime(input.OccurredAt)
	if err != nil {
		return model.AgentAccountLedger{}, errors.New("invalid occurredAt")
	}
	if occurredAt == nil {
		now := time.Now().UTC()
		occurredAt = &now
	}
	policy := input.ReversePolicy
	if policy == "" {
		policy = LedgerReversePolicyConditional
	}

	var ledger model.AgentAccountLedger
	err = db.Transaction(func(tx *gorm.DB) error {
		var account model.AgentAccount
		if err := tx.Where("agent_id = ?", input.AgentID).First(&account).Error; err != nil {
			return err
		}
		currency := strings.TrimSpace(input.Currency)
		if currency == "" {
			currency = account.Currency
		}
		if currency == "" {
			currency = "CNY"
		}
		ledger = model.AgentAccountLedger{
			TenantID:       account.TenantID,
			BrandID:        account.BrandID,
			AccountID:      account.ID,
			AgentID:        input.AgentID,
			ReferenceType:  strings.TrimSpace(input.ReferenceType),
			ReferenceID:    strings.TrimSpace(input.ReferenceID),
			LedgerType:     input.LedgerType,
			Amount:         input.Amount,
			Currency:       currency,
			OccurredAt:     *occurredAt,
			IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
			Remark:         strings.TrimSpace(input.Remark),
			BalanceBefore:  account.Balance,
			FrozenBefore:   account.FrozenBalance,
		}
		switch input.LedgerType {
		case model.LedgerTypeIncome:
			ledger.Direction = model.LedgerDirectionCredit
			account.Balance += input.Amount
			account.TotalIncome += input.Amount
		case model.LedgerTypeFreeze:
			ledger.Direction = model.LedgerDirectionDebit
			if account.WithdrawableAmount < input.Amount {
				return errors.New("insufficient withdrawable amount")
			}
			account.FrozenBalance += input.Amount
		case model.LedgerTypeUnfreeze:
			ledger.Direction = model.LedgerDirectionCredit
			if account.FrozenBalance < input.Amount {
				return errors.New("insufficient frozen balance")
			}
			account.FrozenBalance -= input.Amount
		case model.LedgerTypeReverse:
			ledger.Direction = model.LedgerDirectionDebit
			if shouldApplyReverseBalanceChange(policy, input.ReferenceType) {
				if account.Balance < input.Amount {
					return errors.New("insufficient balance")
				}
				account.Balance -= input.Amount
				account.TotalReversed += input.Amount
			}
		default:
			return errors.New("unsupported ledgerType")
		}
		account.WithdrawableAmount = account.Balance - account.FrozenBalance
		account.AvailableBalance = account.Balance
		ledger.BalanceAfter = account.Balance
		ledger.FrozenAfter = account.FrozenBalance
		account.Version++
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		return tx.Create(&ledger).Error
	})
	return ledger, err
}

func shouldApplyReverseBalanceChange(policy LedgerReversePolicy, referenceType string) bool {
	switch policy {
	case LedgerReversePolicyAlways:
		return true
	case LedgerReversePolicyConditional:
		referenceType = strings.TrimSpace(referenceType)
		return referenceType == "withdrawal_request" || referenceType == "commission_record" || referenceType == "recalculation_task"
	default:
		return false
	}
}
