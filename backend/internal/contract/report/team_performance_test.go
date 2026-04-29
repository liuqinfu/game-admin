package report

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientTeamPerformanceSuccess(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	var gotTraceID string
	client := NewHTTPClient("http://report.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequestID = req.Header.Get("X-Request-ID")
			gotTraceID = req.Header.Get("X-Trace-ID")
			return jsonResponse(req, http.StatusOK, `{"items":[{"agentID":7,"agentName":"A","currency":"CNY"}]}`), nil
		})},
	})

	items, err := client.TeamPerformance(syncclient.WithCorrelation(context.Background(), "req-report-1", "trace-report-1"), sharedsvc.Scope{TenantIDs: []uint64{1}}, "7", "CNY")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, uint64(7), items[0].AgentID)
	require.Equal(t, "req-report-1", gotRequestID)
	require.Equal(t, "trace-report-1", gotTraceID)
}

func TestHTTPClientTeamPerformanceReturnsStatusError(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://report.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(req, http.StatusBadGateway, `{"error":"upstream failed"}`), nil
		})},
	})

	_, err := client.TeamPerformance(context.Background(), sharedsvc.Scope{}, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned status 502")
}

func TestHTTPClientTeamPerformanceWrapsTimeout(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://report.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("i/o timeout")
		})},
	})

	_, err := client.TeamPerformance(context.Background(), sharedsvc.Scope{}, "", "")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "timeout")
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
