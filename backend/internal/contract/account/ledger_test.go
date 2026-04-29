package account

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"game-admin/backend/internal/syncclient"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientCreateLedgerSuccess(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	client := NewHTTPClient("http://account.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequestID = req.Header.Get("X-Request-ID")
			return jsonResponse(req, http.StatusOK, `{"ledger":{"id":5,"accountID":3,"agentID":7,"ledgerType":"freeze","currency":"CNY"}}`), nil
		})},
	})

	ledger, err := client.CreateLedger(syncclient.WithCorrelation(context.Background(), "req-account-1", "trace-account-1"), sharedsvc.Scope{}, LedgerCreateInput{
		AgentID: 7, ReferenceType: "withdrawal_request", ReferenceID: "abc", LedgerType: model.LedgerTypeFreeze, Amount: 10, Currency: "CNY", IdempotencyKey: "k1",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(5), ledger.ID)
	require.Equal(t, "req-account-1", gotRequestID)
}

func TestHTTPClientCreateLedgerReturnsStatusError(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://account.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(req, http.StatusBadRequest, `{"error":"insufficient withdrawable amount"}`), nil
		})},
	})

	_, err := client.CreateLedger(context.Background(), sharedsvc.Scope{}, LedgerCreateInput{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned status 400")
}

func TestHTTPClientCreateLedgerWrapsTimeout(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://account.internal", syncclient.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("i/o timeout")
		})},
	})

	_, err := client.CreateLedger(context.Background(), sharedsvc.Scope{}, LedgerCreateInput{})
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
