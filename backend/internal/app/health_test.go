package app_test

import (
	"errors"
	"testing"
	"time"

	"game-admin/backend/internal/app"
	"github.com/stretchr/testify/require"
)

func TestHealthStoreStatus(t *testing.T) {
	now := time.Date(2026, 4, 20, 9, 10, 0, 0, time.UTC)

	status := app.HealthStore{}.Status(now)

	require.Equal(t, "ok", status.Status)
	require.Equal(t, "game-admin-backend", status.Service)
	require.Equal(t, "dev", status.Version)
	require.Equal(t, now, status.Timestamp)
	require.Empty(t, status.RequestID)
	require.Empty(t, status.TraceID)
	require.Nil(t, status.Dependencies)
}

func TestHealthStoreStatusWithContextAndDependencies(t *testing.T) {
	now := time.Date(2026, 4, 20, 9, 10, 0, 0, time.UTC)
	store := app.HealthStore{
		Service: "identity-service",
		Version: "1.2.3",
		Dependencies: []app.DependencyHealth{
			{Name: "database", Status: "ok", CheckedAt: now.Format(time.RFC3339)},
			{Name: "redis", Status: "error", Message: "dial tcp timeout", CheckedAt: now.Format(time.RFC3339)},
		},
	}

	status := store.StatusWithContext(now, "req-1", "trace-1")

	require.Equal(t, "degraded", status.Status)
	require.Equal(t, "identity-service", status.Service)
	require.Equal(t, "1.2.3", status.Version)
	require.Equal(t, now, status.Timestamp)
	require.Equal(t, "req-1", status.RequestID)
	require.Equal(t, "trace-1", status.TraceID)
	require.Len(t, status.Dependencies, 2)
	require.Equal(t, "database", status.Dependencies[0].Name)
	require.Equal(t, "ok", status.Dependencies[0].Status)
	require.Equal(t, "redis", status.Dependencies[1].Name)
	require.Equal(t, "error", status.Dependencies[1].Status)
	require.Equal(t, "dial tcp timeout", status.Dependencies[1].Message)
}

func TestHealthRuntimeLiveAndReady(t *testing.T) {
	now := time.Date(2026, 4, 20, 9, 10, 0, 0, time.UTC)
	runtime := app.NewHealthRuntime(app.HealthStore{
		Service: "tenant-service",
		Dependencies: []app.DependencyHealth{{Name: "database", Status: "error", Message: "down"}},
	})

	live := runtime.Live(now, "req-live", "trace-live")
	ready := runtime.Ready(now, "req-ready", "trace-ready")

	require.Equal(t, "ok", live.Status)
	require.Nil(t, live.Dependencies)
	require.Equal(t, "req-live", live.RequestID)
	require.Equal(t, "trace-live", live.TraceID)

	require.Equal(t, "degraded", ready.Status)
	require.Len(t, ready.Dependencies, 1)
	require.Equal(t, "req-ready", ready.RequestID)
	require.Equal(t, "trace-ready", ready.TraceID)
}

func TestCheckDependencyAndRequireHealthy(t *testing.T) {
	ok := app.CheckDependency("database", false, func() error { return nil })
	errProbe := errors.New("probe failed")
	failed := app.CheckDependency("rabbitmq", false, func() error { return errProbe })
	optional := app.CheckDependency("redis", true, func() error { return errProbe })

	require.Equal(t, "ok", ok.Status)
	require.Equal(t, "error", failed.Status)
	require.Contains(t, failed.Message, errProbe.Error())
	require.Equal(t, "error", optional.Status)
	require.True(t, optional.Optional)

	err := app.RequireHealthy(app.HealthStatus{Status: "degraded", Dependencies: []app.DependencyHealth{ok, failed, optional}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rabbitmq")
	require.NotContains(t, err.Error(), "redis")

	require.NoError(t, app.RequireHealthy(app.HealthStatus{Status: "ok", Dependencies: []app.DependencyHealth{ok}}))
}
