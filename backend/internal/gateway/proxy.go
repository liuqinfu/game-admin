package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/cache"
	"game-admin/backend/internal/config"
	gahttp "game-admin/backend/internal/http"
	gametrics "game-admin/backend/internal/metrics"
	"game-admin/backend/internal/routing"
	"game-admin/backend/internal/servicedef"
	"game-admin/backend/internal/syncclient"
	"game-admin/backend/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func NewHandler(cfg config.Config) (http.Handler, error) {
	redisRuntime, err := cache.NewRuntime(cfg.Redis)
	if err != nil {
		return nil, err
	}
	proxies := make(map[routing.ServiceKey]http.Handler, len(serviceTargets(cfg)))
	for serviceKey, target := range serviceTargets(cfg) {
		proxy, err := buildProxy(target)
		if err != nil {
			return nil, err
		}
		proxies[serviceKey] = proxy
	}
	return newMux(proxies, redisRuntime, muxOptions{
		serviceName:  "gateway-service",
		httpClient:   &http.Client{Timeout: 2 * time.Second},
		metrics:      gametrics.NewRuntime("gateway-service", "gateway"),
		readyTargets: gatewayReadyTargets(cfg),
	}), nil
}

type muxOptions struct {
	serviceName  string
	httpClient   *http.Client
	metrics      *gametrics.Runtime
	readyTargets []readyTarget
	now          func() time.Time
}

type readyTarget struct {
	name   string
	target string
}

type proxyContextKey string

const (
	proxyRequestIDContextKey proxyContextKey = "gateway_request_id"
	proxyTraceIDContextKey   proxyContextKey = "gateway_trace_id"
	proxyStartedAtContextKey proxyContextKey = "gateway_started_at"
)

func newMux(proxies map[routing.ServiceKey]http.Handler, redisRuntime *cache.Runtime, options muxOptions) http.Handler {
	limiter := cache.FixedWindowLimiter{
		Runtime: redisRuntime,
		Prefix:  "ratelimit:gateway",
		Limit:   200,
		Window:  time.Minute,
	}
	if strings.TrimSpace(options.serviceName) == "" {
		options.serviceName = "gateway-service"
	}
	if options.httpClient == nil {
		options.httpClient = &http.Client{Timeout: 2 * time.Second}
	}
	if options.metrics == nil {
		options.metrics = gametrics.NewRuntime(options.serviceName, "gateway")
	}
	if options.now == nil {
		options.now = time.Now
	}
	healthRuntime := app.NewHealthRuntime(app.HealthStore{
		Service: options.serviceName,
		Version: "dev",
		Dependencies: []app.DependencyHealth{
			app.CheckDependency("redis", true, func() error {
				if redisRuntime == nil {
					return nil
				}
				return redisRuntime.Ping(context.Background())
			}),
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		_ = json.NewEncoder(w).Encode(healthRuntime.Live(options.now(), requestID, traceID))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		now := options.now()
		status := app.NewHealthRuntime(app.HealthStore{
			Service:      options.serviceName,
			Version:      "dev",
			Dependencies: gatewayReadyDependencies(r.Context(), redisRuntime, options.readyTargets, options.httpClient, requestID, traceID),
		}).Ready(now, requestID, traceID)
		options.metrics.SetReady(status.Status == "ok")
		if status.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		_ = json.NewEncoder(w).Encode(status)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		options.metrics.Handler().ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		startedAt := options.now()
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		recordMetrics := func(route string, status int) {
			options.metrics.ObserveHTTPRequest(r.Method, route, status, options.now().Sub(startedAt))
		}
		subject := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
		if subject == "" {
			subject = r.RemoteAddr
		}
		allowed, _, err := limiter.Allow(r.Context(), subject+"|"+r.URL.Path)
		if err == nil && !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			recordMetrics("/ratelimit", http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    "too_many_requests",
				"message": "too many requests",
				"error":   "too many requests",
			})
			return
		}
		match := routing.MatchPath(r.URL.Path)
		if !match.Found {
			http.NotFound(w, r)
			recordMetrics("/not-found", http.StatusNotFound)
			return
		}
		if match.Shared && match.Service == routing.ServiceUnknown {
			http.NotFound(w, r)
			recordMetrics("/not-found", http.StatusNotFound)
			return
		}
		proxy := proxies[match.Service]
		if proxy == nil {
			http.NotFound(w, r)
			recordMetrics("/unmapped-upstream", http.StatusNotFound)
			return
		}
		recorder := &statusCapturingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		spanCtx, span := telemetry.Tracer("gateway.proxy").Start(r.Context(), r.Method+" "+r.URL.Path)
		span.SetAttributes(
			attribute.String("gateway.route", gatewayMetricsRoute(match, r.URL.Path)),
			attribute.String("http.request_id", requestID),
			attribute.String("http.trace_id", traceID),
		)
		r = r.WithContext(context.WithValue(context.WithValue(context.WithValue(spanCtx, proxyRequestIDContextKey, requestID), proxyTraceIDContextKey, traceID), proxyStartedAtContextKey, startedAt))
		proxy.ServeHTTP(recorder, r)
		recordMetrics(gatewayMetricsRoute(match, r.URL.Path), recorder.statusCode)
		span.SetAttributes(attribute.Int("http.response.status_code", recorder.statusCode))
		if recorder.statusCode >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(recorder.statusCode))
			app.Logger(app.LogFields{
				"component":   "gateway",
				"requestID":   requestID,
				"traceID":     traceID,
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      recorder.statusCode,
				"duration_ms": options.now().Sub(startedAt).Milliseconds(),
			}).Error("gateway proxy request failed")
		}
		if recorder.statusCode < http.StatusInternalServerError {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	})
	return mux
}

func buildProxy(target string) (*httputil.ReverseProxy, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		requestID, _ := r.Context().Value(proxyRequestIDContextKey).(string)
		traceID, _ := r.Context().Value(proxyTraceIDContextKey).(string)
		startedAt, _ := r.Context().Value(proxyStartedAtContextKey).(time.Time)
		fields := app.LogFields{
			"component": "gateway",
			"requestID": strings.TrimSpace(requestID),
			"traceID":   strings.TrimSpace(traceID),
			"method":    r.Method,
			"path":      r.URL.Path,
			"status":    http.StatusBadGateway,
			"error":     err.Error(),
		}
		if !startedAt.IsZero() {
			fields["duration_ms"] = time.Since(startedAt).Milliseconds()
		}
		app.Logger().Error("gateway upstream proxy failed", fields)
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}
	return proxy, nil
}

func serviceTargets(cfg config.Config) map[routing.ServiceKey]string {
	targets := make(map[routing.ServiceKey]string, len(servicedef.HTTPList()))
	for _, service := range sortedHTTPServices() {
		targets[service.RoutingKey] = cfg.Gateway.TargetFor(service.RoutingKey)
	}
	return targets
}

func gatewayReadyTargets(cfg config.Config) []readyTarget {
	services := sortedHTTPServices()
	targets := make([]readyTarget, 0, len(services))
	for _, service := range services {
		targets = append(targets, readyTarget{
			name:   service.Name,
			target: cfg.Gateway.TargetFor(service.RoutingKey),
		})
	}
	return targets
}

func sortedHTTPServices() []servicedef.HTTPService {
	services := servicedef.HTTPList()
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	return services
}

func gatewayReadyDependencies(ctx context.Context, redisRuntime *cache.Runtime, targets []readyTarget, client *http.Client, requestID, traceID string) []app.DependencyHealth {
	dependencies := make([]app.DependencyHealth, 0, len(targets)+1)
	dependencies = append(dependencies, app.CheckDependency("redis", true, func() error {
		if redisRuntime == nil {
			return nil
		}
		return redisRuntime.Ping(ctx)
	}))
	for _, target := range targets {
		target := target
		dependencies = append(dependencies, app.CheckDependency(target.name, false, func() error {
			return probeReady(ctx, client, target.target, requestID, traceID)
		}))
	}
	return dependencies
}

func probeReady(ctx context.Context, client *http.Client, target, requestID, traceID string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target is required")
	}
	spanCtx, span := telemetry.Tracer("gateway.ready_probe").Start(ctx, "GET /readyz")
	span.SetAttributes(
		attribute.String("upstream.target", target),
		attribute.String("http.request_id", requestID),
		attribute.String("http.trace_id", traceID),
	)
	defer span.End()
	readyClient := syncclient.New("gateway-ready-probe", target, syncclient.Options{
		HTTPClient: client,
		Timeout:    2 * time.Second,
	})
	if err := readyClient.Do(syncclient.WithCorrelation(spanCtx, requestID, traceID), http.MethodGet, "/readyz", nil, nil); err != nil {
		var statusErr *syncclient.HTTPStatusError
		if errors.As(err, &statusErr) {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			span.SetAttributes(attribute.Int("http.response.status_code", statusErr.StatusCode))
			return fmt.Errorf("readyz returned status %d", statusErr.StatusCode)
		}
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return err
	}
	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("http.response.status_code", http.StatusOK))
	return nil
}

func gatewayMetricsRoute(match routing.Match, path string) string {
	if match.Shared {
		switch path {
		case "/healthz", "/readyz", "/metrics", "/api/enum-dictionaries", "/openapi/game/enum-dictionaries":
			return path
		default:
			return "/shared"
		}
	}
	module := strings.TrimSpace(match.Module)
	if module == "" {
		return "/proxy"
	}
	return "/proxy/" + module
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusCapturingResponseWriter) Write(payload []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(payload)
}
