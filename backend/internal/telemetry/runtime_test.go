package telemetry

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"game-admin/backend/internal/config"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestInitDisabledReturnsNoopRuntime(t *testing.T) {
	t.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
		otlpHTTPClient = http.DefaultClient
	})

	runtime, err := Init(context.Background(), config.Config{
		Service:   config.ServiceConfig{Name: "gateway-service"},
		Telemetry: config.TelemetryConfig{Enabled: false, Exporter: "none"},
	})
	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.False(t, runtime.Enabled())
	require.NoError(t, runtime.Shutdown(context.Background()))
}

func TestInitRejectsUnsupportedExporter(t *testing.T) {
	t.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
		otlpHTTPClient = http.DefaultClient
	})

	runtime, err := Init(context.Background(), config.Config{
		Service: config.ServiceConfig{Name: "gateway-service"},
		Telemetry: config.TelemetryConfig{
			Enabled:  true,
			Exporter: "zipkin",
		},
	})
	require.Nil(t, runtime)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported telemetry exporter")
}

func TestInitOTLPHTTPExporterFlushesSpanOnShutdown(t *testing.T) {
	t.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	var (
		mu       sync.Mutex
		readErr  error
		requests []string
		payloads [][]byte
	)
	otlpHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			_ = req.Body.Close()
			if err != nil {
				mu.Lock()
				readErr = err
				mu.Unlock()
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}

			mu.Lock()
			requests = append(requests, req.Method+" "+req.URL.Path)
			payloads = append(payloads, body)
			mu.Unlock()

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	t.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
		otlpHTTPClient = http.DefaultClient
	})

	runtime, err := Init(context.Background(), config.Config{
		Env:     "test",
		Service: config.ServiceConfig{Name: "gateway-service"},
		Telemetry: config.TelemetryConfig{
			Enabled:        true,
			Exporter:       "otlp",
			OTLPEndpoint:   "http://collector.internal",
			OTLPInsecure:   true,
			ServiceVersion: "2026.04.28",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.True(t, runtime.Enabled())

	ctx, span := Tracer("test.telemetry").Start(context.Background(), "gateway.readyz")
	span.SetAttributes(
		attribute.String("service.name", "gateway-service"),
		attribute.String("http.route", "/readyz"),
	)
	span.End()

	require.NoError(t, runtime.Shutdown(ctx))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(requests) > 0
	}, 3*time.Second, 50*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.NoError(t, readErr)
	require.Contains(t, requests[0], "POST ")
	require.Contains(t, requests[0], "/v1/traces")
	require.NotEmpty(t, payloads[0])
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
