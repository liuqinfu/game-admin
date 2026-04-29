package app_test

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenSQLiteAndAutoMigrate(t *testing.T) {
	t.Parallel()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := app.OpenSQLite(dsn)
	require.NoError(t, err)

	require.NoError(t, app.AutoMigrate(db))
	require.True(t, db.Migrator().HasTable(&app.SystemInfo{}))
}

func TestVerifyMQRejectsUnsupportedDriver(t *testing.T) {
	t.Parallel()

	err := app.VerifyMQ(config.MQConfig{
		Enabled: true,
		Driver:  "kafka",
		URL:     "kafka://broker:9092",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported mq driver")
}
