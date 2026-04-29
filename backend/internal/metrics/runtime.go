package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "game_admin"
	subsystem = "runtime"
)

type Runtime struct {
	service   string
	component string
	registry  *prometheus.Registry
	handler   http.Handler

	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
	readyStatus      prometheus.Gauge
	workerPolls      *prometheus.CounterVec
	workerDeliveries *prometheus.CounterVec
	workerLocks      *prometheus.CounterVec
}

func NewRuntime(service, component string) *Runtime {
	service = strings.TrimSpace(service)
	if service == "" {
		service = "game-admin-backend"
	}
	component = strings.TrimSpace(component)
	if component == "" {
		component = "http"
	}

	labels := prometheus.Labels{
		"service":   service,
		"component": component,
	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "http_requests_total",
		Help:        "Total HTTP requests handled by the runtime.",
		ConstLabels: labels,
	}, []string{"method", "route", "status_class"})
	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "http_request_duration_seconds",
		Help:        "HTTP request duration in seconds.",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	}, []string{"method", "route", "status_class"})
	readyStatus := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "ready_status",
		Help:        "Current readiness status. 1 means ready, 0 means not ready.",
		ConstLabels: labels,
	})
	workerPolls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "worker_polls_total",
		Help:        "Total worker poll attempts.",
		ConstLabels: labels,
	}, []string{"consumer", "mode", "result"})
	workerDeliveries := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "worker_deliveries_total",
		Help:        "Total worker delivery outcomes.",
		ConstLabels: labels,
	}, []string{"consumer", "mode", "result"})
	workerLocks := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "worker_lock_acquisitions_total",
		Help:        "Total worker lock acquisition attempts.",
		ConstLabels: labels,
	}, []string{"consumer", "stage", "result"})

	registry.MustRegister(httpRequests, httpDuration, readyStatus, workerPolls, workerDeliveries, workerLocks)
	readyStatus.Set(0)

	return &Runtime{
		service:          service,
		component:        component,
		registry:         registry,
		handler:          promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		httpRequests:     httpRequests,
		httpDuration:     httpDuration,
		readyStatus:      readyStatus,
		workerPolls:      workerPolls,
		workerDeliveries: workerDeliveries,
		workerLocks:      workerLocks,
	}
}

func (r *Runtime) Handler() http.Handler {
	if r == nil || r.handler == nil {
		return http.NotFoundHandler()
	}
	return r.handler
}

func (r *Runtime) InstrumentHTTP(next http.Handler, route func(*http.Request) string) http.Handler {
	if r == nil || next == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		recorder := &statusCapturingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, req)
		routeLabel := req.URL.Path
		if route != nil {
			routeLabel = route(req)
		}
		r.ObserveHTTPRequest(req.Method, routeLabel, recorder.statusCode, time.Since(startedAt))
	})
}

func (r *Runtime) ObserveHTTPRequest(method, route string, statusCode int, duration time.Duration) {
	if r == nil {
		return
	}
	route = normalizeRoute(route)
	statusClass := strconv.Itoa(statusCode/100) + "xx"
	r.httpRequests.WithLabelValues(normalizeMethod(method), route, statusClass).Inc()
	r.httpDuration.WithLabelValues(normalizeMethod(method), route, statusClass).Observe(duration.Seconds())
}

func (r *Runtime) SetReady(isReady bool) {
	if r == nil {
		return
	}
	if isReady {
		r.readyStatus.Set(1)
		return
	}
	r.readyStatus.Set(0)
}

func (r *Runtime) RecordWorkerPoll(consumer, mode, result string) {
	if r == nil {
		return
	}
	r.workerPolls.WithLabelValues(normalizeLabel(consumer, "default"), normalizeLabel(mode, "default"), normalizeLabel(result, "unknown")).Inc()
}

func (r *Runtime) RecordWorkerDelivery(consumer, mode, result string) {
	if r == nil {
		return
	}
	r.workerDeliveries.WithLabelValues(normalizeLabel(consumer, "default"), normalizeLabel(mode, "default"), normalizeLabel(result, "unknown")).Inc()
}

func (r *Runtime) RecordWorkerLock(consumer, stage, result string) {
	if r == nil {
		return
	}
	r.workerLocks.WithLabelValues(normalizeLabel(consumer, "default"), normalizeLabel(stage, "default"), normalizeLabel(result, "unknown")).Inc()
}

func normalizeMethod(method string) string {
	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return "GET"
	}
	return method
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "/"
	}
	return route
}

func normalizeLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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
