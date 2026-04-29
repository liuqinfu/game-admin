package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"game-admin/backend/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type Runtime struct {
	enabled  bool
	provider *sdktrace.TracerProvider
}

var otlpHTTPClient = http.DefaultClient

func Init(ctx context.Context, cfg config.Config) (*Runtime, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Telemetry.Enabled || strings.EqualFold(strings.TrimSpace(cfg.Telemetry.Exporter), "none") {
		return &Runtime{}, nil
	}

	exporterName := strings.TrimSpace(strings.ToLower(cfg.Telemetry.Exporter))
	if exporterName != "otlp" {
		return nil, fmt.Errorf("unsupported telemetry exporter %q", cfg.Telemetry.Exporter)
	}

	options := []otlptracehttp.Option{}
	if endpoint := strings.TrimSpace(cfg.Telemetry.OTLPEndpoint); endpoint != "" {
		options = append(options, otlptracehttp.WithEndpointURL(endpoint))
	}
	if cfg.Telemetry.OTLPInsecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	if otlpHTTPClient != nil {
		options = append(options, otlptracehttp.WithHTTPClient(otlpHTTPClient))
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("init otlp trace exporter: %w", err)
	}

	resource, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.Service.Name),
			semconv.ServiceVersion(cfg.Telemetry.ServiceVersion),
			attribute.String("deployment.environment.name", cfg.Env),
		),
		sdkresource.WithProcess(),
		sdkresource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("build telemetry resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)
	otel.SetTracerProvider(provider)

	return &Runtime{
		enabled:  true,
		provider: provider,
	}, nil
}

func (r *Runtime) Enabled() bool {
	return r != nil && r.enabled
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil || r.provider == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.provider.Shutdown(shutdownCtx)
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(strings.TrimSpace(name))
}
