package account

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateLedgerReturnsExistingOnDuplicateIdempotencyKey(t *testing.T) {
	t.Parallel()

	db := openAccountTestDB(t)
	agent := model.Agent{AgentNo: "AG-ACC-1", Name: "Account Agent", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	account := model.AgentAccount{
		AgentID:            agent.ID,
		AccountNo:          "ACC-TEST-1",
		Status:             model.AccountStatusActive,
		Currency:           "CNY",
		Balance:            120,
		AvailableBalance:   120,
		WithdrawableAmount: 120,
		Version:            1,
	}
	require.NoError(t, db.Create(&account).Error)

	service := NewService(db)
	first, err := service.CreateLedger(sharedsvc.Scope{}, LedgerCreateInput{
		AgentID:        agent.ID,
		ReferenceType:  "risk_case",
		ReferenceID:    "RC-ACC-1",
		LedgerType:     model.LedgerTypeFreeze,
		Amount:         20,
		Currency:       "CNY",
		IdempotencyKey: "dup-ledger-1",
	})
	require.NoError(t, err)

	second, err := service.CreateLedger(sharedsvc.Scope{}, LedgerCreateInput{
		AgentID:        agent.ID,
		ReferenceType:  "risk_case",
		ReferenceID:    "RC-ACC-1",
		LedgerType:     model.LedgerTypeFreeze,
		Amount:         20,
		Currency:       "CNY",
		IdempotencyKey: "dup-ledger-1",
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
}

func openAccountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "account.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
