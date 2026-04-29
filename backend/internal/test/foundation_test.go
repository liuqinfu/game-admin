package test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBackendFoundationHealthEndpoint(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Env:      "test",
		HTTPPort: "18080",
		Database: config.DatabaseConfig{DSN: filepath.Join(t.TempDir(), "integration.db")},
	}

	application, err := app.Bootstrap(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()

	application.Router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}
