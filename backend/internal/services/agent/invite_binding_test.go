package agent

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInviteBindingAndRelationPublishEvents(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "invite-binding.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))

	service := NewService(db)
	scope := sharedsvc.Scope{}

	inviter := model.Agent{AgentNo: "AG-INV-1", Name: "Inviter", Status: model.AgentStatusActive, Currency: "CNY"}
	applicant := model.Agent{AgentNo: "AG-INV-2", Name: "Applicant", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&inviter).Error)
	require.NoError(t, db.Create(&applicant).Error)

	invite, err := service.CreateInviteCode(scope, InviteCodeCreateInput{
		AgentID: inviter.ID,
		Code:    "INV-1",
		Status:  model.InviteCodeStatusActive,
	})
	require.NoError(t, err)
	require.NotNil(t, invite)

	player, binding, err := service.RegisterWithInvite(scope, RegisterWithInviteInput{
		PlatformUserID: "u-1",
		InviteCode:     "INV-1",
	})
	require.NoError(t, err)
	require.NotNil(t, player)
	require.NotNil(t, binding)

	application, err := service.SubmitInviteApplication(scope, InviteApplicationCreateInput{
		ApplicantAgentID: applicant.ID,
		InviteCode:       "INV-1",
	})
	require.NoError(t, err)
	require.NotNil(t, application)

	audited, _, err := service.AuditInviteApplication(scope, application.ID, InviteApplicationAuditInput{
		Status: model.AgentInviteApplicationStatusApproved,
	}, "tester")
	require.NoError(t, err)
	require.Equal(t, model.AgentInviteApplicationStatusApproved, audited.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 6)
	require.Equal(t, eventbus.EventInviteCodeCreated, events[0].EventType)
	require.Equal(t, eventbus.EventBindingCreated, events[1].EventType)
	require.Equal(t, eventbus.EventPlayerRegisteredWithInvite, events[2].EventType)
	require.Equal(t, eventbus.EventAgentInviteApplied, events[3].EventType)
	require.Equal(t, eventbus.EventAgentRelationCreated, events[4].EventType)
	require.Equal(t, eventbus.EventAgentInviteAudited, events[5].EventType)
}
