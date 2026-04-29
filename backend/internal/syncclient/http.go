package syncclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	service    string
	baseURL    string
	httpClient *http.Client
}

type Options struct {
	HTTPClient *http.Client
	Timeout    time.Duration
}

type HTTPStatusError struct {
	Service    string
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("%s %s %s returned status %d", strings.TrimSpace(e.Service), e.Method, e.URL, e.StatusCode)
	}
	return fmt.Sprintf("%s %s %s returned status %d: %s", strings.TrimSpace(e.Service), e.Method, e.URL, e.StatusCode, strings.TrimSpace(e.Body))
}

func New(service, baseURL string, options Options) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		timeout := options.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		service:    strings.TrimSpace(service),
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: httpClient,
	}
}

func (c *Client) Do(ctx context.Context, method, requestPath string, requestBody any, responseBody any) error {
	if c == nil {
		return fmt.Errorf("sync client is nil")
	}
	endpoint := c.baseURL + "/" + strings.TrimLeft(strings.TrimSpace(requestPath), "/")
	var bodyReader io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, strings.TrimSpace(method), endpoint, bodyReader)
	if err != nil {
		return err
	}
	requestID, traceID := CorrelationFromContext(ctx)
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	if traceID != "" {
		req.Header.Set("X-Trace-ID", traceID)
		if traceparent := formatTraceparent(traceID); traceparent != "" {
			req.Header.Set("traceparent", traceparent)
		}
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s %s failed: %w", c.service, req.Method, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &HTTPStatusError{
			Service:    c.service,
			Method:     req.Method,
			URL:        endpoint,
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}
	if responseBody == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(responseBody)
}

func formatTraceparent(traceID string) string {
	traceID = strings.TrimSpace(strings.ToLower(traceID))
	if len(traceID) != 32 {
		return ""
	}
	if _, err := hex.DecodeString(traceID); err != nil {
		return ""
	}
	spanID := make([]byte, 8)
	if _, err := rand.Read(spanID); err != nil {
		return ""
	}
	return "00-" + traceID + "-" + hex.EncodeToString(spanID) + "-01"
}
