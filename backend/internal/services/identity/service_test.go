package identity

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRBACWritesPublishEvents(t *testing.T) {
	t.Parallel()

	db := openIdentityTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	permission := model.AdminPermission{Code: "agent:read", Name: "Agent Read"}
	require.NoError(t, db.Create(&permission).Error)

	role, err := service.CreateRole(scope, RoleInput{Code: "ops", Name: "Ops", Permissions: []string{"agent:read"}})
	require.NoError(t, err)
	_, updatedRole, err := service.UpdateRole(scope, role.ID, RoleInput{Name: "Ops2", Permissions: []string{"agent:read"}})
	require.NoError(t, err)
	require.Equal(t, "Ops2", updatedRole.Name)

	user, err := service.CreateUser(scope, UserInput{Username: "ops", Password: "123", DisplayName: "Ops", Roles: []string{"ops"}})
	require.NoError(t, err)
	_, updatedUser, err := service.UpdateUser(scope, user.ID, UserInput{DisplayName: "Ops User", Roles: []string{"ops"}})
	require.NoError(t, err)
	require.Equal(t, "Ops User", updatedUser.DisplayName)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 4)
	require.Equal(t, "rbac.role.created", events[0].EventType)
	require.Equal(t, "rbac.role.updated", events[1].EventType)
	require.Equal(t, "rbac.user.created", events[2].EventType)
	require.Equal(t, "rbac.user.updated", events[3].EventType)
}

func openIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "identity.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
