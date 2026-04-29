package activity

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestActivityRuleAndRecordPublishEvents(t *testing.T) {
	t.Parallel()

	db := openActivityTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	rule, err := service.CreateRule(scope, RuleCreateInput{
		Name:         "Invite Bonus",
		ActivityType: "invite",
		RewardType:   "cash",
		Status:       model.ActivityRewardRuleStatusActive,
		RewardValue:  18.8,
		Currency:     "CNY",
	})
	require.NoError(t, err)

	before, activeRule, err := service.UpdateRuleStatus(scope, rule.ID, model.ActivityRewardRuleStatusActive)
	require.NoError(t, err)
	require.Equal(t, model.ActivityRewardRuleStatusActive, before.Status)
	require.Equal(t, model.ActivityRewardRuleStatusActive, activeRule.Status)

	player := uint64(1001)
	record, code, err := service.GenerateRecord(scope, rule.ID, RecordGenerateInput{
		PlayerID:      &player,
		ReferenceType: "manual",
		ReferenceID:   "REF-1",
	})
	require.NoError(t, err)
	require.Equal(t, 201, code)

	_, reversed, code, err := service.ReverseRecord(scope, record.ID, RecordReverseInput{Remark: "rollback"})
	require.NoError(t, err)
	require.Equal(t, 200, code)
	require.Equal(t, model.ActivityRewardRecordStatusReversed, reversed.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 4)
	require.Equal(t, "activity.rule.created", events[0].EventType)
	require.Equal(t, "activity.rule.status_changed", events[1].EventType)
	require.Equal(t, "activity.record.granted", events[2].EventType)
	require.Equal(t, "activity.record.reversed", events[3].EventType)
}

func openActivityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "activity.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
