package risk

import (
	"context"
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRiskCreateAndReviewPublishEvents(t *testing.T) {
	t.Parallel()

	db := openRiskTestDB(t)
	agent := model.Agent{AgentNo: "AG-RISK-1", Name: "Risk Agent", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	account := model.AgentAccount{
		AgentID:            agent.ID,
		AccountNo:          "ACC-RISK-1",
		Status:             model.AccountStatusActive,
		Currency:           "CNY",
		Balance:            200,
		AvailableBalance:   200,
		WithdrawableAmount: 200,
		Version:            1,
	}
	require.NoError(t, db.Create(&account).Error)

	service := NewService(db)
	scope := sharedsvc.Scope{}

	created, ledger, err := service.CreateCase(context.Background(), scope, CreateCaseInput{
		CaseNo:   "RC-1001",
		AgentID:  agent.ID,
		Amount:   50,
		Currency: "CNY",
		Reason:   "manual review",
		Freeze:   true,
	}, "tester")
	require.NoError(t, err)
	require.NotNil(t, ledger)

	reviewed, _, err := service.ReviewCase(context.Background(), scope, created.CaseNo, ReviewCaseInput{Action: "release", Remark: "ok"}, "reviewer")
	require.NoError(t, err)
	require.Equal(t, model.RiskCaseStatusReleased, reviewed.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, "risk.case.created", events[0].EventType)
	require.Equal(t, "risk.case.released", events[1].EventType)

	var deliveries int64
	require.NoError(t, db.Model(&model.DomainEventDelivery{}).Count(&deliveries).Error)
	require.Equal(t, int64(4), deliveries)
}

func TestRiskRestorePublishesEvent(t *testing.T) {
	t.Parallel()

	db := openRiskTestDB(t)
	agent := model.Agent{AgentNo: "AG-RISK-RESTORE-1", Name: "Risk Restore Agent", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	account := model.AgentAccount{
		AgentID:            agent.ID,
		AccountNo:          "ACC-RISK-RESTORE-1",
		Status:             model.AccountStatusActive,
		Currency:           "CNY",
		Balance:            300,
		AvailableBalance:   250,
		WithdrawableAmount: 250,
		FrozenBalance:      50,
		Version:            1,
	}
	require.NoError(t, db.Create(&account).Error)
	caseRecord := model.RiskCase{
		CaseNo:          "RC-RESTORE-1",
		AgentID:         agent.ID,
		Amount:          50,
		Currency:        "CNY",
		Reason:          "rollback payout",
		Status:          model.RiskCaseStatusReleased,
		FreezeRequested: true,
	}
	require.NoError(t, db.Create(&caseRecord).Error)

	service := NewService(db)
	count, err := service.RestorePendingCases(context.Background(), sharedsvc.Scope{}, []string{caseRecord.CaseNo}, "rollback payout", "finance")
	require.NoError(t, err)
	require.Equal(t, 1, count)

	var updated model.RiskCase
	require.NoError(t, db.First(&updated, caseRecord.ID).Error)
	require.Equal(t, model.RiskCaseStatusPending, updated.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "risk.case.restored", events[0].EventType)

	var deliveries int64
	require.NoError(t, db.Model(&model.DomainEventDelivery{}).Count(&deliveries).Error)
	require.Equal(t, int64(2), deliveries)
}

func openRiskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "risk.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
