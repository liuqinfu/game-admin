package syncclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientDoSuccessPropagatesCorrelationHeaders(t *testing.T) {
	t.Parallel()

	var gotRequestID string
	var gotTraceID string
	var gotTraceparent string
	client := New("report-service", "http://report.internal", Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotRequestID = req.Header.Get("X-Request-ID")
				gotTraceID = req.Header.Get("X-Trace-ID")
				gotTraceparent = req.Header.Get("traceparent")
				return jsonResponse(req, http.StatusOK, `{"ok":true}`), nil
			}),
		},
	})

	var response map[string]bool
	err := client.Do(WithCorrelation(context.Background(), "req-sync-1", "trace-sync-1"), http.MethodGet, "/readyz", nil, &response)
	require.NoError(t, err)
	require.True(t, response["ok"])
	require.Equal(t, "req-sync-1", gotRequestID)
	require.Equal(t, "trace-sync-1", gotTraceID)
	require.Empty(t, gotTraceparent)
}

func TestClientDoPropagatesW3CTraceparentWhenTraceIDIsHex(t *testing.T) {
	t.Parallel()

	var gotTraceparent string
	client := New("report-service", "http://report.internal", Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotTraceparent = req.Header.Get("traceparent")
				return jsonResponse(req, http.StatusOK, `{"ok":true}`), nil
			}),
		},
	})

	err := client.Do(WithCorrelation(context.Background(), "req-sync-2", "4bf92f3577b34da6a3ce929d0e0e4736"), http.MethodGet, "/readyz", nil, nil)
	require.NoError(t, err)
	require.Contains(t, gotTraceparent, "00-4bf92f3577b34da6a3ce929d0e0e4736-")
}

func TestClientDoReturnsStatusError(t *testing.T) {
	t.Parallel()

	client := New("report-service", "http://report.internal", Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(req, http.StatusBadGateway, `{"error":"upstream failed"}`), nil
			}),
		},
	})

	err := client.Do(context.Background(), http.MethodGet, "/team-performance", nil, nil)
	require.Error(t, err)
	var statusErr *HTTPStatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, http.StatusBadGateway, statusErr.StatusCode)
	require.Contains(t, statusErr.Body, "upstream failed")
}

func TestClientDoWrapsTimeoutError(t *testing.T) {
	t.Parallel()

	client := New("report-service", "http://report.internal", Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("i/o timeout")
			}),
		},
	})

	err := client.Do(context.Background(), http.MethodGet, "/team-performance", nil, nil)
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

func TestNewUsesDefaultTimeout(t *testing.T) {
	t.Parallel()
	client := New("report-service", "http://report.internal", Options{})
	require.NotNil(t, client.httpClient)
	require.Equal(t, 5*time.Second, client.httpClient.Timeout)
}
