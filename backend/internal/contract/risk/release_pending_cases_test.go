package risk

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

func TestHTTPClientReleasePendingCasesSuccess(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	var gotTraceID string
	client := NewHTTPClient("http://risk.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequestID = req.Header.Get("X-Request-ID")
			gotTraceID = req.Header.Get("X-Trace-ID")
			return jsonResponse(req, http.StatusOK, `{"releasedCount":2,"caseNos":["RISK-1","RISK-2"]}`), nil
		})},
	})

	released, err := client.ReleasePendingCases(
		syncclient.WithCorrelation(context.Background(), "req-risk-1", "trace-risk-1"),
		sharedsvc.Scope{TenantIDs: []uint64{1}},
		7,
		"done",
		"finance",
	)
	require.NoError(t, err)
	require.Equal(t, []string{"RISK-1", "RISK-2"}, released)
	require.Equal(t, "req-risk-1", gotRequestID)
	require.Equal(t, "trace-risk-1", gotTraceID)
}

func TestHTTPClientReleasePendingCasesReturnsStatusError(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://risk.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(req, http.StatusBadRequest, `{"error":"invalid agentID"}`), nil
		})},
	})

	_, err := client.ReleasePendingCases(context.Background(), sharedsvc.Scope{}, 0, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned status 400")
}

func TestHTTPClientReleasePendingCasesWrapsTimeout(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://risk.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("i/o timeout")
		})},
	})

	_, err := client.ReleasePendingCases(context.Background(), sharedsvc.Scope{}, 7, "", "")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "timeout")
}

func TestHTTPClientRestorePendingCasesSuccess(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	var gotTraceID string
	client := NewHTTPClient("http://risk.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequestID = req.Header.Get("X-Request-ID")
			gotTraceID = req.Header.Get("X-Trace-ID")
			return jsonResponse(req, http.StatusOK, `{"restoredCount":2}`), nil
		})},
	})

	restored, err := client.RestorePendingCases(
		syncclient.WithCorrelation(context.Background(), "req-risk-restore-1", "trace-risk-restore-1"),
		sharedsvc.Scope{TenantIDs: []uint64{1}},
		[]string{"RISK-1", "RISK-2"},
		"rollback payout",
		"finance",
	)
	require.NoError(t, err)
	require.Equal(t, 2, restored)
	require.Equal(t, "req-risk-restore-1", gotRequestID)
	require.Equal(t, "trace-risk-restore-1", gotTraceID)
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
