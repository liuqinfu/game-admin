package app_test

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"testing"

	"game-admin/backend/internal/app"
	"github.com/stretchr/testify/require"
)

func TestServiceLoggerJSONOutput(t *testing.T) {
	t.Setenv("SERVICE_NAME", "identity-service")
	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetFlags(0)
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}()

	app.Logger(app.LogFields{"component": "bootstrap"}).Info("service starting", app.LogFields{"port": "8081"})

	line := bytes.TrimSpace(buf.Bytes())
	require.NotEmpty(t, line)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(line, &payload))
	require.Equal(t, "INFO", payload["level"])
	require.Equal(t, "service starting", payload["message"])
	require.Equal(t, "identity-service", payload["service"])
	require.Equal(t, "bootstrap", payload["component"])
	require.Equal(t, "8081", payload["port"])
}

func TestServiceLoggerOmitsBlankService(t *testing.T) {
	require.NoError(t, os.Unsetenv("SERVICE_NAME"))
	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetFlags(0)
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}()

	app.Logger().Error("boom", app.LogFields{"reason": "test"})

	var payload map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &payload))
	_, exists := payload["service"]
	require.False(t, exists)
	require.Equal(t, "ERROR", payload["level"])
	require.Equal(t, "boom", payload["message"])
	require.Equal(t, "test", payload["reason"])
}
