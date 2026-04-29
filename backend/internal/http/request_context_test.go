package http

import (
	"bytes"
	"encoding/json"
	"log"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestContextUsesTraceparentWhenTraceIDMissing(t *testing.T) {
	t.Setenv("SERVICE_NAME", "identity-service")
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(requestContextMiddleware())
	router.GET("/trace", func(c *gin.Context) {
		writeJSON(c, nethttp.StatusOK, gin.H{
			"requestID": requestIDFromContext(c),
			"traceID":   traceIDFromContext(c),
		})
	})

	req := httptest.NewRequest(nethttp.MethodGet, "/trace", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, nethttp.StatusOK, resp.Code)
	require.NotEmpty(t, resp.Header().Get("X-Request-ID"))
	require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", resp.Header().Get("X-Trace-ID"))
}

func TestRequestContextMiddlewareLogsStructuredFields(t *testing.T) {
	t.Setenv("SERVICE_NAME", "identity-service")
	gin.SetMode(gin.ReleaseMode)

	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetFlags(0)
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}()

	router := gin.New()
	router.Use(requestContextMiddleware())
	router.GET("/boom", func(c *gin.Context) {
		c.Error(nethttp.ErrAbortHandler)
		c.Status(nethttp.StatusInternalServerError)
	})

	req := httptest.NewRequest(nethttp.MethodGet, "/boom", nil)
	req.Header.Set("X-Request-ID", "req-http-1")
	req.Header.Set("X-Trace-ID", "trace-http-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.NotEmpty(t, lines)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(lines[len(lines)-1], &payload))
	require.Equal(t, "ERROR", payload["level"])
	require.Equal(t, "identity-service", payload["service"])
	require.Equal(t, "http", payload["component"])
	require.Equal(t, "req-http-1", payload["requestID"])
	require.Equal(t, "trace-http-1", payload["traceID"])
	require.Equal(t, "GET", payload["method"])
	require.Equal(t, "/boom", payload["path"])
	require.Equal(t, float64(500), payload["status"])
	_, ok := payload["duration_ms"]
	require.True(t, ok)
	require.NotEmpty(t, payload["error"])
}
