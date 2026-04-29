package settlement

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

func TestHTTPClientGenerateBillWithSourceTaskSuccess(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	var gotTraceID string
	client := NewHTTPClient("http://settlement.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequestID = req.Header.Get("X-Request-ID")
			gotTraceID = req.Header.Get("X-Trace-ID")
			return jsonResponse(req, http.StatusOK, `{"billID":11,"detailCount":3,"commissionAmount":88.5}`), nil
		})},
	})

	billID, detailCount, commissionAmount, err := client.GenerateBillWithSourceTask(
		syncclient.WithCorrelation(context.Background(), "req-settlement-1", "trace-settlement-1"),
		sharedsvc.Scope{TenantIDs: []uint64{1}},
		CreateBillInput{AgentID: 7, Currency: "CNY"},
		uint64Ptr(99),
	)
	require.NoError(t, err)
	require.Equal(t, uint64(11), billID)
	require.Equal(t, 3, detailCount)
	require.Equal(t, 88.5, commissionAmount)
	require.Equal(t, "req-settlement-1", gotRequestID)
	require.Equal(t, "trace-settlement-1", gotTraceID)
}

func TestHTTPClientGenerateBillWithSourceTaskReturnsStatusError(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://settlement.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(req, http.StatusBadRequest, `{"error":"bad request"}`), nil
		})},
	})

	_, _, _, err := client.GenerateBillWithSourceTask(context.Background(), sharedsvc.Scope{}, CreateBillInput{}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned status 400")
}

func TestHTTPClientGenerateBillWithSourceTaskWrapsTimeout(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://settlement.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("i/o timeout")
		})},
	})

	_, _, _, err := client.GenerateBillWithSourceTask(context.Background(), sharedsvc.Scope{}, CreateBillInput{}, nil)
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

func uint64Ptr(value uint64) *uint64 { return &value }
