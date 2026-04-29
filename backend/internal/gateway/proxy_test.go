package gateway

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"game-admin/backend/internal/config"
	gametrics "game-admin/backend/internal/metrics"
	"game-admin/backend/internal/routing"
	"game-admin/backend/internal/servicedef"
	"github.com/stretchr/testify/require"
)

func TestNewHandlerRoutesBySharedMetadata(t *testing.T) {
	t.Parallel()

	handler := newMux(map[routing.ServiceKey]http.Handler{
		routing.ServiceIdentity:   backendHandler("identity"),
		routing.ServiceTenant:     backendHandler("tenant"),
		routing.ServiceAgent:      backendHandler("agent"),
		routing.ServiceRelation:   backendHandler("relation"),
		routing.ServiceGame:       backendHandler("game"),
		routing.ServiceRule:       backendHandler("rule"),
		routing.ServiceActivity:   backendHandler("activity"),
		routing.ServiceRecharge:   backendHandler("recharge"),
		routing.ServiceSettlement: backendHandler("settlement"),
		routing.ServiceAccount:    backendHandler("account"),
		routing.ServiceWithdrawal: backendHandler("withdrawal"),
		routing.ServiceRisk:       backendHandler("risk"),
		routing.ServiceReport:     backendHandler("report"),
		routing.ServiceAudit:      backendHandler("audit"),
	}, nil, muxOptions{})

	tests := []struct {
		name          string
		path          string
		wantStatus    int
		wantBody      string
		wantRequestID bool
	}{
		{name: "health", path: "/healthz", wantStatus: http.StatusOK, wantRequestID: true},
		{name: "ready", path: "/readyz", wantStatus: http.StatusOK, wantRequestID: true},
		{name: "metrics", path: "/metrics", wantStatus: http.StatusOK, wantRequestID: true},
		{name: "identity", path: "/api/auth/me", wantStatus: http.StatusOK, wantBody: "identity"},
		{name: "tenant shared enum", path: "/api/enum-dictionaries", wantStatus: http.StatusOK, wantBody: "tenant"},
		{name: "agent", path: "/api/agents", wantStatus: http.StatusOK, wantBody: "agent"},
		{name: "relation", path: "/api/agents/1/ancestors", wantStatus: http.StatusOK, wantBody: "relation"},
		{name: "recharge shared enum", path: "/openapi/game/enum-dictionaries", wantStatus: http.StatusOK, wantBody: "recharge"},
		{name: "recharge callback", path: "/openapi/game/recharge/callback", wantStatus: http.StatusOK, wantBody: "recharge"},
		{name: "unknown", path: "/not-found", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantRequestID && rec.Header().Get("X-Request-ID") == "" {
				t.Fatalf("missing X-Request-ID header")
			}
		})
	}
}

func TestGatewayReadyzProbesUpstreamsAndReturnsDependencyDetails(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	var gotTraceID string
	cfg := gatewayConfigForTests("http://healthy.internal")
	cfg.Gateway.RiskURL = "http://risk.internal"
	handler := newMux(nil, nil, muxOptions{
		serviceName:  "gateway-service",
		readyTargets: gatewayReadyTargets(cfg),
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequestID = req.Header.Get("X-Request-ID")
			gotTraceID = req.Header.Get("X-Trace-ID")
			status := http.StatusOK
			if req.URL.Host == "risk.internal" {
				status = http.StatusServiceUnavailable
			}
			return jsonResponse(req, status, `{"status":"ok"}`), nil
		})},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set("X-Request-ID", "req-ready-gateway")
	req.Header.Set("X-Trace-ID", "trace-ready-gateway")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "req-ready-gateway", rec.Header().Get("X-Request-ID"))
	require.Equal(t, "trace-ready-gateway", rec.Header().Get("X-Trace-ID"))
	require.Equal(t, "req-ready-gateway", gotRequestID)
	require.Equal(t, "trace-ready-gateway", gotTraceID)

	var payload struct {
		Status       string `json:"status"`
		RequestID    string `json:"requestID"`
		TraceID      string `json:"traceID"`
		Dependencies []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"dependencies"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "degraded", payload.Status)
	require.Equal(t, "req-ready-gateway", payload.RequestID)
	require.Equal(t, "trace-ready-gateway", payload.TraceID)
	require.NotEmpty(t, payload.Dependencies)

	deps := make(map[string]struct {
		Status  string
		Message string
	}, len(payload.Dependencies))
	for _, dependency := range payload.Dependencies {
		deps[dependency.Name] = struct {
			Status  string
			Message string
		}{Status: dependency.Status, Message: dependency.Message}
	}
	require.Equal(t, "ok", deps["identity-service"].Status)
	require.Equal(t, "error", deps["risk-service"].Status)
	require.Contains(t, deps["risk-service"].Message, "readyz returned status 503")
}

func TestGatewayReadyzMarksTimeoutAsDependencyError(t *testing.T) {
	t.Parallel()

	handler := newMux(nil, nil, muxOptions{
		serviceName: "gateway-service",
		readyTargets: []readyTarget{
			{name: "slow-service", target: "http://slow.internal"},
		},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: req.Method, URL: req.URL.String(), Err: errors.New("i/o timeout")}
		})},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "slow-service")
	require.Contains(t, strings.ToLower(rec.Body.String()), "timeout")
}

func TestGatewayProxyNormalizesCorrelationHeaders(t *testing.T) {
	t.Parallel()

	handler := newMux(map[routing.ServiceKey]http.Handler{
		routing.ServiceIdentity: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.Header.Get("X-Request-ID")+"|"+r.Header.Get("X-Trace-ID"))
		}),
	}, nil, muxOptions{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	parts := strings.Split(rec.Body.String(), "|")
	require.Len(t, parts, 2)
	require.NotEmpty(t, parts[0])
	require.NotEmpty(t, parts[1])
}

func TestGatewayMetricsEndpointExposesPrometheus(t *testing.T) {
	t.Parallel()

	handler := newMux(map[routing.ServiceKey]http.Handler{
		routing.ServiceIdentity: backendHandler("identity"),
	}, nil, muxOptions{
		metrics: gametrics.NewRuntime("gateway-service", "gateway"),
	})

	proxyReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	proxyResp := httptest.NewRecorder()
	handler.ServeHTTP(proxyResp, proxyReq)
	require.Equal(t, http.StatusOK, proxyResp.Code)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_http_requests_total")
	require.Contains(t, resp.Body.String(), "/proxy/auth")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_ready_status")
}

func backendHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = io.WriteString(w, name)
	})
}

func gatewayConfigForTests(url string) config.Config {
	cfg := config.Config{}
	for _, service := range servicedef.HTTPList() {
		switch service.RoutingKey {
		case routing.ServiceIdentity:
			cfg.Gateway.IdentityURL = url
		case routing.ServiceTenant:
			cfg.Gateway.TenantURL = url
		case routing.ServiceAgent:
			cfg.Gateway.AgentURL = url
		case routing.ServiceRelation:
			cfg.Gateway.RelationURL = url
		case routing.ServiceGame:
			cfg.Gateway.GameURL = url
		case routing.ServiceRule:
			cfg.Gateway.RuleURL = url
		case routing.ServiceActivity:
			cfg.Gateway.ActivityURL = url
		case routing.ServiceRecharge:
			cfg.Gateway.RechargeURL = url
		case routing.ServiceSettlement:
			cfg.Gateway.SettlementURL = url
		case routing.ServiceAccount:
			cfg.Gateway.AccountURL = url
		case routing.ServiceWithdrawal:
			cfg.Gateway.WithdrawalURL = url
		case routing.ServiceRisk:
			cfg.Gateway.RiskURL = url
		case routing.ServiceReport:
			cfg.Gateway.ReportURL = url
		case routing.ServiceAudit:
			cfg.Gateway.AuditURL = url
		}
	}
	return cfg
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
