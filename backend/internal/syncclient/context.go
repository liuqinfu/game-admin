package syncclient

import "context"

type correlationContextKey string

const (
	requestIDContextKey correlationContextKey = "request_id"
	traceIDContextKey   correlationContextKey = "trace_id"
)

func WithCorrelation(ctx context.Context, requestID, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if requestID != "" {
		ctx = context.WithValue(ctx, requestIDContextKey, requestID)
	}
	if traceID != "" {
		ctx = context.WithValue(ctx, traceIDContextKey, traceID)
	}
	return ctx
}

func CorrelationFromContext(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	traceID, _ := ctx.Value(traceIDContextKey).(string)
	return requestID, traceID
}
