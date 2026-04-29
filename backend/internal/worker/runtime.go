package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	gahttp "game-admin/backend/internal/http"
	gametrics "game-admin/backend/internal/metrics"
	"game-admin/backend/internal/servicedef"
	"game-admin/backend/internal/syncclient"
	"game-admin/backend/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gorm.io/gorm"
)

type RuntimeOptions struct {
	RequireDatabase bool
	RequireRedis    bool
	RequireMQ       bool
	ConsumerName    string
	ConsumerHandler eventbus.Handler
	ConsumerBatch   int
	PollInterval    time.Duration
	LockTTL         time.Duration
	Metrics         *gametrics.Runtime
}

func Run(defaultName, defaultPort string, options RuntimeOptions) {
	applyDefaultEnv("SERVICE_NAME", defaultName)
	applyDefaultEnv("HTTP_PORT", defaultPort)
	applyDefaultEnv("METRICS_PORT", defaultPort)

	cfg := config.Load()
	telemetryRuntime, err := telemetry.Init(context.Background(), cfg)
	if err != nil {
		log.Fatalf("init telemetry: %v", err)
	}
	defer func() { _ = telemetryRuntime.Shutdown(context.Background()) }()
	var db *gorm.DB
	var redisRuntime *app.RedisRuntime
	if options.Metrics == nil {
		options.Metrics = gametrics.NewRuntime(cfg.Service.Name, "worker")
	}

	if options.RequireDatabase {
		db, err = app.OpenDatabase(cfg.Database)
		if err != nil {
			app.Logger(app.LogFields{"component": "worker"}).Error("open database failed", app.LogFields{"error": err.Error()})
			log.Fatalf("open database: %v", err)
		}
		if sqlDB, err := db.DB(); err == nil {
			defer sqlDB.Close()
		}
	}
	if options.RequireRedis {
		var err error
		redisRuntime, err = app.NewRedisRuntime(cfg.Redis)
		if err != nil {
			app.Logger(app.LogFields{"component": "worker"}).Error("init redis runtime failed", app.LogFields{"error": err.Error()})
			log.Fatalf("init redis runtime: %v", err)
		}
		if err := redisRuntime.Ping(context.Background()); err != nil {
			app.Logger(app.LogFields{"component": "worker"}).Error("ping redis runtime failed", app.LogFields{"error": err.Error()})
			log.Fatalf("ping redis runtime: %v", err)
		}
	}
	_ = redisRuntime
	if options.RequireMQ {
		if err := app.VerifyMQ(cfg.MQ); err != nil {
			app.Logger(app.LogFields{"component": "worker"}).Error("verify mq failed", app.LogFields{"error": err.Error()})
			log.Fatalf("verify mq: %v", err)
		}
	}
	if strings.TrimSpace(options.ConsumerName) != "" {
		if db == nil {
			log.Fatalf("consumer %s requires database", options.ConsumerName)
		}
		if cfg.MQ.Enabled {
			transport, err := eventbus.NewTransport(cfg.MQ, eventbus.TransportOptions{
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
			})
			if err != nil {
				if errors.Is(err, eventbus.ErrUnsupportedMQDriver) {
					log.Fatalf("unsupported mq transport driver %q", cfg.MQ.Driver)
				}
				app.Logger(app.LogFields{"component": "worker"}).Error("initialize mq transport failed", app.LogFields{"error": err.Error(), "consumer": options.ConsumerName})
				log.Fatalf("initialize mq transport: %v", err)
			}
			go runMQConsumerLoop(context.Background(), db, redisRuntime, transport, cfg.Service.Name, options)
		} else {
			go runConsumerLoop(context.Background(), db, redisRuntime, cfg.Service.Name, options)
		}
	}

	healthRuntime := app.NewHealthRuntime(app.HealthStore{
		Service:      cfg.Service.Name,
		Version:      "dev",
		Dependencies: workerDependencies(cfg, options),
	})
	mux := newRuntimeMux(healthRuntime, options)

	app.Logger(app.LogFields{"component": "worker"}).Info("service starting", app.LogFields{"address": cfg.Address(), "consumer": options.ConsumerName})
	if err := http.ListenAndServe(cfg.Address(), mux); err != nil {
		app.Logger(app.LogFields{"component": "worker"}).Error("run worker failed", app.LogFields{"error": err.Error(), "address": cfg.Address(), "consumer": options.ConsumerName})
		log.Fatalf("run worker: %v", err)
	}
}

func newRuntimeMux(healthRuntime app.HealthRuntime, options RuntimeOptions) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		_ = json.NewEncoder(w).Encode(healthRuntime.Live(time.Now(), requestID, traceID))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		status := healthRuntime.Ready(time.Now(), requestID, traceID)
		options.Metrics.SetReady(status.Status == "ok")
		if status.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(status)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		requestID, traceID := gahttp.NormalizeHeadersForHTTP(r.Header)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Trace-ID", traceID)
		options.Metrics.Handler().ServeHTTP(w, r)
	})
	return options.Metrics.InstrumentHTTP(mux, func(r *http.Request) string {
		return r.URL.Path
	})
}

func RunNamed(name string, options RuntimeOptions) {
	def := servicedef.MustWorker(name)
	options.ConsumerName = strings.TrimSpace(sharedValue(options.ConsumerName, def.ConsumerName))
	Run(def.Name, def.DefaultPort, options)
}

func sharedValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func runConsumerLoop(ctx context.Context, db *gorm.DB, redisRuntime *app.RedisRuntime, workerName string, options RuntimeOptions) {
	consumerName := strings.TrimSpace(options.ConsumerName)
	if consumerName == "" {
		return
	}
	handler := options.ConsumerHandler
	if handler == nil {
		handler = func(context.Context, eventbus.Delivery) (map[string]any, error) {
			return map[string]any{"status": "ignored"}, nil
		}
	}
	batchSize := options.ConsumerBatch
	if batchSize <= 0 {
		batchSize = 20
	}
	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	process := func() {
		runWithConsumerLock(ctx, redisRuntime, consumerLockConfig{
			ConsumerName: consumerName,
			WorkerName:   workerName,
			TTL:          resolveLockTTL(options, pollInterval),
			Stage:        "consume",
			Metrics:      options.Metrics,
		}, func() {
			now := time.Now().UTC()
			claimRequestID, claimTraceID := workerAttemptCorrelation(workerName, consumerName, eventbus.Delivery{})
			deliveries, err := eventbus.ClaimPendingDeliveriesWithCorrelation(db, consumerName, workerName, claimRequestID, claimTraceID, batchSize, now)
			if err != nil {
				options.Metrics.RecordWorkerPoll(consumerName, "db", "error")
				app.Logger(app.LogFields{"component": "worker"}).Error("claim deliveries failed", app.LogFields{"consumer": consumerName, "requestID": claimRequestID, "traceID": claimTraceID, "error": err.Error()})
				return
			}
			options.Metrics.RecordWorkerPoll(consumerName, "db", "ok")
			for _, delivery := range deliveries {
				requestID, traceID := workerAttemptCorrelation(workerName, consumerName, delivery)
				handlerCtx, span := telemetry.Tracer("worker.consume").Start(syncclient.WithCorrelation(ctx, requestID, traceID), "db consume "+delivery.Event.EventType)
				span.SetAttributes(
					attribute.String("worker.consumer", consumerName),
					attribute.String("event.type", delivery.Event.EventType),
					attribute.String("http.request_id", requestID),
					attribute.String("http.trace_id", traceID),
				)
				result, err := handler(handlerCtx, delivery)
				if err != nil {
					outcome, markErr := eventbus.MarkFailed(db, delivery.ID, err, 0, result, time.Now().UTC())
					if outcome.Status == model.DomainEventDeliveryStatusDeadLetter {
						options.Metrics.RecordWorkerDelivery(consumerName, "db", "dead_letter")
					} else {
						options.Metrics.RecordWorkerDelivery(consumerName, "db", "failed")
					}
					if markErr != nil {
						app.Logger(app.LogFields{"component": "worker"}).Error("mark delivery failed state failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusInternalServerError, "error": markErr.Error()})
						span.RecordError(markErr)
						span.SetStatus(codes.Error, markErr.Error())
						span.End()
						continue
					}
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.SetAttributes(attribute.String("delivery.status", string(outcome.Status)))
					logWorkerFailure("db_consume", consumerName, delivery, requestID, traceID, err, outcome)
					span.End()
					continue
				}
				options.Metrics.RecordWorkerDelivery(consumerName, "db", "succeeded")
				if err := eventbus.MarkDelivered(db, delivery.ID, result, time.Now().UTC()); err != nil {
					app.Logger(app.LogFields{"component": "worker"}).Error("mark delivered failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusInternalServerError, "error": err.Error()})
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.End()
					continue
				}
				span.SetStatus(codes.Ok, "")
				span.SetAttributes(attribute.String("delivery.status", string(model.DomainEventDeliveryStatusSucceeded)))
				span.End()
			}
		})
	}

	process()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}

func runMQConsumerLoop(ctx context.Context, db *gorm.DB, redisRuntime *app.RedisRuntime, transport eventbus.Transport, workerName string, options RuntimeOptions) {
	consumerName := strings.TrimSpace(options.ConsumerName)
	if consumerName == "" {
		return
	}
	defer func() {
		if transport != nil {
			_ = transport.Close()
		}
	}()
	handler := options.ConsumerHandler
	if handler == nil {
		handler = func(context.Context, eventbus.Delivery) (map[string]any, error) {
			return map[string]any{"status": "ignored"}, nil
		}
	}
	batchSize := options.ConsumerBatch
	if batchSize <= 0 {
		batchSize = 20
	}
	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	dispatchAndConsume := func() {
		runWithConsumerLock(ctx, redisRuntime, consumerLockConfig{
			ConsumerName: consumerName,
			WorkerName:   workerName,
			TTL:          resolveLockTTL(options, pollInterval),
			Stage:        "mq",
			Metrics:      options.Metrics,
		}, func() {
			now := time.Now().UTC()
			dispatchRequestID, dispatchTraceID := workerAttemptCorrelation(workerName+"-dispatch", consumerName, eventbus.Delivery{})
			deliveries, err := eventbus.ClaimPendingDeliveriesWithCorrelation(db, consumerName, workerName+"-dispatch", dispatchRequestID, dispatchTraceID, batchSize, now)
			if err != nil {
				options.Metrics.RecordWorkerPoll(consumerName, "mq_dispatch", "error")
				app.Logger(app.LogFields{"component": "worker"}).Error("claim mq dispatch deliveries failed", app.LogFields{"consumer": consumerName, "requestID": dispatchRequestID, "traceID": dispatchTraceID, "error": err.Error()})
				return
			}
			options.Metrics.RecordWorkerPoll(consumerName, "mq_dispatch", "ok")
			for _, delivery := range deliveries {
				requestID, traceID := workerAttemptCorrelation(workerName+"-dispatch", consumerName, delivery)
				publishCtx, span := telemetry.Tracer("worker.publish").Start(syncclient.WithCorrelation(ctx, requestID, traceID), "mq publish "+delivery.Event.EventType)
				span.SetAttributes(
					attribute.String("worker.consumer", consumerName),
					attribute.String("event.type", delivery.Event.EventType),
					attribute.String("http.request_id", requestID),
					attribute.String("http.trace_id", traceID),
				)
				err := transport.Publish(publishCtx, consumerName, delivery)
				if err != nil {
					outcome, markErr := eventbus.MarkFailed(db, delivery.ID, err, 0, map[string]any{"stage": "publish"}, time.Now().UTC())
					if outcome.Status == model.DomainEventDeliveryStatusDeadLetter {
						options.Metrics.RecordWorkerDelivery(consumerName, "mq_publish", "dead_letter")
					} else {
						options.Metrics.RecordWorkerDelivery(consumerName, "mq_publish", "failed")
					}
					if markErr != nil {
						app.Logger(app.LogFields{"component": "worker"}).Error("mark mq publish failure failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusBadGateway, "error": markErr.Error()})
						span.RecordError(markErr)
						span.SetStatus(codes.Error, markErr.Error())
						span.End()
						continue
					}
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.SetAttributes(attribute.String("delivery.status", string(outcome.Status)))
					logWorkerFailure("mq_publish", consumerName, delivery, requestID, traceID, err, outcome)
					span.End()
					continue
				}
				options.Metrics.RecordWorkerDelivery(consumerName, "mq_publish", "queued")
				if err := eventbus.MarkQueued(db, delivery.ID, map[string]any{"stage": "queued", "transport": "rabbitmq"}, time.Now().UTC()); err != nil {
					app.Logger(app.LogFields{"component": "worker"}).Error("mark queued failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusInternalServerError, "error": err.Error()})
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.End()
					continue
				}
				span.SetStatus(codes.Ok, "")
				span.SetAttributes(attribute.String("delivery.status", string(model.DomainEventDeliveryStatusQueued)))
				span.End()
			}

			queued, err := transport.Consume(ctx, consumerName, batchSize)
			if err != nil {
				options.Metrics.RecordWorkerPoll(consumerName, "mq_consume", "error")
				log.Printf("%s consume rabbitmq failed: %v", consumerName, err)
				return
			}
			options.Metrics.RecordWorkerPoll(consumerName, "mq_consume", "ok")
			for _, delivery := range queued {
				requestID, traceID := workerAttemptCorrelation(workerName, consumerName, delivery)
				acquired, beginErr := eventbus.BeginQueuedDelivery(db, delivery.ID, workerName, requestID, traceID, time.Now().UTC())
				if beginErr != nil {
					app.Logger(app.LogFields{"component": "worker"}).Error("begin queued delivery failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusInternalServerError, "error": beginErr.Error()})
					continue
				}
				if !acquired {
					options.Metrics.RecordWorkerDelivery(consumerName, "mq_consume", "duplicate")
					continue
				}
				handlerCtx, span := telemetry.Tracer("worker.consume").Start(syncclient.WithCorrelation(ctx, requestID, traceID), "mq consume "+delivery.Event.EventType)
				span.SetAttributes(
					attribute.String("worker.consumer", consumerName),
					attribute.String("event.type", delivery.Event.EventType),
					attribute.String("http.request_id", requestID),
					attribute.String("http.trace_id", traceID),
				)
				result, err := handler(handlerCtx, delivery)
				if err != nil {
					outcome, markErr := eventbus.MarkFailed(db, delivery.ID, err, 0, result, time.Now().UTC())
					if outcome.Status == model.DomainEventDeliveryStatusDeadLetter {
						options.Metrics.RecordWorkerDelivery(consumerName, "mq_consume", "dead_letter")
					} else {
						options.Metrics.RecordWorkerDelivery(consumerName, "mq_consume", "failed")
					}
					if markErr != nil {
						app.Logger(app.LogFields{"component": "worker"}).Error("mark mq consume failure failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusInternalServerError, "error": markErr.Error()})
						span.RecordError(markErr)
						span.SetStatus(codes.Error, markErr.Error())
						span.End()
						continue
					}
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.SetAttributes(attribute.String("delivery.status", string(outcome.Status)))
					logWorkerFailure("mq_consume", consumerName, delivery, requestID, traceID, err, outcome)
					span.End()
					continue
				}
				options.Metrics.RecordWorkerDelivery(consumerName, "mq_consume", "succeeded")
				if err := eventbus.MarkDelivered(db, delivery.ID, result, time.Now().UTC()); err != nil {
					app.Logger(app.LogFields{"component": "worker"}).Error("mark delivered failed", app.LogFields{"consumer": consumerName, "requestID": requestID, "traceID": traceID, "path": delivery.Event.EventType, "status": http.StatusInternalServerError, "error": err.Error()})
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					span.End()
					continue
				}
				span.SetStatus(codes.Ok, "")
				span.SetAttributes(attribute.String("delivery.status", string(model.DomainEventDeliveryStatusSucceeded)))
				span.End()
			}
		})
	}

	dispatchAndConsume()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatchAndConsume()
		}
	}
}

func applyDefaultEnv(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, value)
}

type consumerLockConfig struct {
	ConsumerName string
	WorkerName   string
	TTL          time.Duration
	Stage        string
	Metrics      *gametrics.Runtime
}

func runWithConsumerLock(ctx context.Context, redisRuntime *app.RedisRuntime, cfg consumerLockConfig, run func()) {
	if redisRuntime == nil {
		run()
		return
	}
	lockKey := "worker-lock:" + strings.TrimSpace(cfg.ConsumerName) + ":" + strings.TrimSpace(cfg.Stage)
	lockToken := strings.TrimSpace(cfg.WorkerName) + ":" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + ":" + strconv.Itoa(rand.Intn(100000))
	locked, err := redisRuntime.AcquireLock(ctx, lockKey, lockToken, cfg.TTL)
	if err != nil {
		cfg.Metrics.RecordWorkerLock(cfg.ConsumerName, cfg.Stage, "error")
		app.Logger(app.LogFields{"component": "worker"}).Error("acquire redis consumer lock failed", app.LogFields{"consumer": cfg.ConsumerName, "path": cfg.Stage, "status": http.StatusInternalServerError, "error": err.Error()})
		run()
		return
	}
	if !locked {
		cfg.Metrics.RecordWorkerLock(cfg.ConsumerName, cfg.Stage, "miss")
		return
	}
	cfg.Metrics.RecordWorkerLock(cfg.ConsumerName, cfg.Stage, "ok")
	defer func() {
		if err := redisRuntime.ReleaseLock(ctx, lockKey, lockToken); err != nil {
			app.Logger(app.LogFields{"component": "worker"}).Error("release redis consumer lock failed", app.LogFields{"consumer": cfg.ConsumerName, "path": cfg.Stage, "status": http.StatusInternalServerError, "error": err.Error()})
		}
	}()
	run()
}

func resolveLockTTL(options RuntimeOptions, pollInterval time.Duration) time.Duration {
	if options.LockTTL > 0 {
		return options.LockTTL
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	return pollInterval*3 + time.Second
}

func workerAttemptCorrelation(workerName, consumerName string, delivery eventbus.Delivery) (string, string) {
	requestID, traceID := eventbus.DeliveryCorrelation(delivery)
	if requestID == "" {
		requestID = "req-" + strings.TrimSpace(workerName) + "-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	}
	if traceID == "" {
		traceID = firstNonEmptyTrace(requestID, consumerName)
	}
	return requestID, traceID
}

func firstNonEmptyTrace(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "trace-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func logWorkerFailure(stage, consumer string, delivery eventbus.Delivery, requestID, traceID string, cause error, outcome eventbus.FailureOutcome) {
	fields := app.LogFields{
		"component":   "worker",
		"requestID":   requestID,
		"traceID":     traceID,
		"method":      stage,
		"path":        delivery.Event.EventType,
		"status":      deliveryStatusCode(outcome.Status),
		"duration_ms": 0,
		"consumer":    consumer,
		"deliveryID":  delivery.ID,
	}
	if cause != nil {
		fields["error"] = cause.Error()
	}
	if !outcome.NextAttemptAt.IsZero() {
		fields["next_attempt_at"] = outcome.NextAttemptAt.Format(time.RFC3339)
	}
	if outcome.RetryDelay > 0 {
		fields["retry_delay_ms"] = outcome.RetryDelay.Milliseconds()
	}
	app.Logger().Error("worker delivery failed", fields)
}

func deliveryStatusCode(status model.DomainEventDeliveryStatus) int {
	if status == model.DomainEventDeliveryStatusDeadLetter {
		return http.StatusGone
	}
	return http.StatusServiceUnavailable
}

func workerDependencies(cfg config.Config, options RuntimeOptions) []app.DependencyHealth {
	dependencies := make([]app.DependencyHealth, 0, 3)
	if options.RequireDatabase {
		dependencies = append(dependencies, app.CheckDependency("database", false, func() error {
			_, err := app.OpenDatabase(cfg.Database)
			return err
		}))
	}
	if options.RequireRedis && cfg.Redis.Enabled {
		dependencies = append(dependencies, app.CheckDependency("redis", false, func() error {
			return app.VerifyRedis(cfg.Redis)
		}))
	}
	if options.RequireMQ && cfg.MQ.Enabled {
		dependencies = append(dependencies, app.CheckDependency("mq", false, func() error {
			return app.VerifyMQ(cfg.MQ)
		}))
	}
	return dependencies
}
