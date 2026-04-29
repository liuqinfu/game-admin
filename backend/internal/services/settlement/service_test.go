package settlement

import (
	"path/filepath"
	"testing"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSettlementGenerateAndConfirmPublishEvents(t *testing.T) {
	t.Parallel()

	db := openSettlementTestDB(t)
	agent := model.Agent{AgentNo: "AG-ST-1", Name: "Agent ST", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	now := time.Now().UTC()
	record := model.CommissionRecord{
		RecordNo:             "COM-ST-1",
		RechargeOrderID:      1,
		PlayerID:             1,
		AgentID:              agent.ID,
		GameID:               1,
		CommissionBaseAmount: 100,
		SettlementRate:       1,
		CommissionRate:       0.2,
		CommissionAmount:     20,
		Currency:             "CNY",
		Status:               model.CommissionStatusSettled,
		EstimatedAt:          now.Add(-time.Hour),
	}
	require.NoError(t, db.Create(&record).Error)

	service := NewService(db)
	scope := sharedsvc.Scope{}
	bill, err := service.GenerateBill(scope, CreateBillInput{
		AgentID:     agent.ID,
		PeriodStart: now.Add(-2 * time.Hour).Format(time.RFC3339),
		PeriodEnd:   now.Add(time.Hour).Format(time.RFC3339),
		Currency:    "CNY",
	})
	require.NoError(t, err)

	_, confirmed, err := service.ConfirmBill(scope, bill.ID, "tester")
	require.NoError(t, err)
	require.Equal(t, model.SettlementBillStatusConfirmed, confirmed.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, "settlement.bill.generated", events[0].EventType)
	require.Equal(t, "settlement.bill.confirmed", events[1].EventType)

	var deliveries int64
	require.NoError(t, db.Model(&model.DomainEventDelivery{}).Count(&deliveries).Error)
	require.Equal(t, int64(3), deliveries)
}

func openSettlementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settlement.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
