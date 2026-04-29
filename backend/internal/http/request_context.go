package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	gametrics "game-admin/backend/internal/metrics"
	"game-admin/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	requestIDHeader   = "X-Request-ID"
	traceIDHeader     = "X-Trace-ID"
	traceparentHeader = "traceparent"
	requestIDKey      = "requestID"
	traceIDKey        = "traceID"
)

func NormalizeHeadersForHTTP(headers http.Header) (string, string) {
	requestID := normalizeCorrelationID(headers.Get(requestIDHeader), "req")
	if requestID == "" {
		requestID = newCorrelationID("req")
	}
	traceID := normalizeCorrelationID(headers.Get(traceIDHeader), "trace")
	if traceID == "" {
		traceID = traceIDFromTraceparent(headers.Get(traceparentHeader))
	}
	if traceID == "" {
		traceID = requestID
	}
	headers.Set(requestIDHeader, requestID)
	headers.Set(traceIDHeader, traceID)
	return requestID, traceID
}

func requestContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now().UTC()
		ensureRequestContext(c)
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagationHeaderCarrier(c.Request.Header))
		ctx, span := telemetry.Tracer("http.request").Start(ctx, c.Request.Method+" "+routePath(c))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		latency := time.Since(startedAt)
		c.Header("X-Response-Time-Ms", strconv.FormatInt(latency.Milliseconds(), 10))
		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(c.Request.Method),
			semconv.URLPath(routePath(c)),
			attribute.Int("http.response.status_code", c.Writer.Status()),
			attribute.String("http.request_id", requestIDFromContext(c)),
			attribute.String("http.trace_id", traceIDFromContext(c)),
		)
		fields := map[string]any{
			"level":       "INFO",
			"message":     "http request completed",
			"service":     currentServiceName(),
			"component":   "http",
			"requestID":   requestIDFromContext(c),
			"traceID":     traceIDFromContext(c),
			"method":      c.Request.Method,
			"path":        routePath(c),
			"status":      c.Writer.Status(),
			"duration_ms": latency.Milliseconds(),
		}
		if err := strings.TrimSpace(c.Errors.ByType(gin.ErrorTypeAny).String()); err != "" {
			fields["error"] = err
			span.RecordError(c.Errors.Last())
		}
		if c.Writer.Status() >= http.StatusInternalServerError || fields["error"] != nil {
			span.SetStatus(codes.Error, strings.TrimSpace(toString(fields["error"])))
			fields["level"] = "ERROR"
			writeStructuredLog(fields)
			span.End()
			return
		}
		writeStructuredLog(fields)
		span.SetStatus(codes.Ok, "")
		span.End()
	}
}

func requestMetricsMiddleware(runtime *gametrics.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		route := c.FullPath()
		if strings.TrimSpace(route) == "" {
			route = c.Request.URL.Path
		}
		runtime.ObserveHTTPRequest(c.Request.Method, route, c.Writer.Status(), time.Since(startedAt))
	}
}

func ensureRequestContext(c *gin.Context) {
	if c == nil {
		return
	}
	requestID := normalizeCorrelationID(c.GetHeader(requestIDHeader), "req")
	if requestID == "" {
		requestID = newCorrelationID("req")
	}
	traceID := normalizeCorrelationID(c.GetHeader(traceIDHeader), "trace")
	if traceID == "" {
		traceID = traceIDFromTraceparent(c.GetHeader(traceparentHeader))
	}
	if traceID == "" {
		traceID = requestID
	}
	c.Set(requestIDKey, requestID)
	c.Set(traceIDKey, traceID)
	c.Writer.Header().Set(requestIDHeader, requestID)
	c.Writer.Header().Set(traceIDHeader, traceID)
	c.Request.Header.Set(requestIDHeader, requestID)
	c.Request.Header.Set(traceIDHeader, traceID)
}

func requestIDFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get(requestIDKey); ok {
		if requestID, ok := value.(string); ok {
			return strings.TrimSpace(requestID)
		}
	}
	return strings.TrimSpace(c.GetHeader(requestIDHeader))
}

func traceIDFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get(traceIDKey); ok {
		if traceID, ok := value.(string); ok {
			return strings.TrimSpace(traceID)
		}
	}
	return strings.TrimSpace(c.GetHeader(traceIDHeader))
}

func newCorrelationID(prefix string) string {
	payload := make([]byte, 16)
	if _, err := rand.Read(payload); err != nil {
		now := time.Now().UTC().UnixNano()
		for i := range payload {
			payload[i] = byte(now >> ((i % 8) * 8))
		}
	}
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		trimmedPrefix = "cid"
	}
	return trimmedPrefix + "-" + hex.EncodeToString(payload)
}

func normalizeCorrelationID(raw, fallbackPrefix string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 128 {
		trimmed = trimmed[:128]
	}
	builder := strings.Builder{}
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.', r == ':':
			builder.WriteRune(r)
		}
	}
	normalized := strings.Trim(builder.String(), "-_.:")
	if normalized == "" {
		return newCorrelationID(fallbackPrefix)
	}
	return normalized
}

func requestContextFields(c *gin.Context) gin.H {
	return gin.H{
		"requestID": requestIDFromContext(c),
		"traceID":   traceIDFromContext(c),
	}
}

func withRequestContext(payload gin.H, c *gin.Context) gin.H {
	if payload == nil {
		payload = gin.H{}
	}
	for key, value := range requestContextFields(c) {
		payload[key] = value
	}
	return payload
}

func writeJSON(c *gin.Context, status int, payload any) {
	if c == nil {
		return
	}
	ensureRequestContext(c)
	c.Status(status)
	c.Header("Content-Type", "application/json")
	writer := c.Writer
	if body, ok := payload.(gin.H); ok {
		_ = jsonNewEncoder(writer).Encode(withRequestContext(body, c))
		return
	}
	_ = jsonNewEncoder(writer).Encode(payload)
}

func writeStatusJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = jsonNewEncoder(w).Encode(payload)
}

func currentServiceName() string {
	return strings.TrimSpace(os.Getenv("SERVICE_NAME"))
}

func safePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func routePath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "/"
	}
	if route := safePath(c.FullPath()); route != "/" || strings.TrimSpace(c.Request.URL.Path) == "" {
		return route
	}
	return safePath(c.Request.URL.Path)
}

func traceIDFromTraceparent(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 4 {
		return ""
	}
	traceID := strings.TrimSpace(parts[1])
	if len(traceID) != 32 {
		return ""
	}
	if _, err := hex.DecodeString(traceID); err != nil {
		return ""
	}
	if traceID == "00000000000000000000000000000000" {
		return ""
	}
	return strings.ToLower(traceID)
}

func writeStructuredLog(fields map[string]any) {
	if strings.TrimSpace(currentServiceName()) == "" {
		delete(fields, "service")
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		log.Printf("level=%s message=%q log_error=%q", fields["level"], fields["message"], err.Error())
		return
	}
	log.Print(string(encoded))
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

type propagationHeaderCarrier http.Header

func (c propagationHeaderCarrier) Get(key string) string {
	return http.Header(c).Get(key)
}

func (c propagationHeaderCarrier) Set(key string, value string) {
	http.Header(c).Set(key, value)
}

func (c propagationHeaderCarrier) Keys() []string {
	headers := http.Header(c)
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	return keys
}
