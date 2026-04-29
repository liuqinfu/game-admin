package shared

import (
	"errors"
	"strconv"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

func EnsureAgentAccount(tx *gorm.DB, agentID uint64, currency string) (*model.AgentAccount, error) {
	var account model.AgentAccount
	if err := tx.Where("agent_id = ?", agentID).First(&account).Error; err == nil {
		return &account, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var agent model.Agent
	if err := tx.First(&agent, agentID).Error; err != nil {
		return nil, err
	}

	account = model.AgentAccount{
		TenantID:           agent.TenantID,
		BrandID:            agent.BrandID,
		AgentID:            agentID,
		AccountNo:          BuildReferenceNo("ACC", strconv.FormatUint(agentID, 10), 0),
		Status:             model.AccountStatusActive,
		Currency:           DefaultCurrency(currency),
		Balance:            0,
		AvailableBalance:   0,
		FrozenBalance:      0,
		WithdrawableAmount: 0,
		TotalIncome:        0,
		TotalReversed:      0,
		Version:            1,
	}
	if err := tx.Create(&account).Error; err != nil {
		if IsUniqueConstraintError(err) {
			if retryErr := tx.Where("agent_id = ?", agentID).First(&account).Error; retryErr == nil {
				return &account, nil
			}
		}
		return nil, err
	}
	return &account, nil
}
