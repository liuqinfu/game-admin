package gateway

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
	"game-admin/backend/internal/servicedef"
	"game-admin/backend/internal/telemetry"
)

func Run(defaultName string) {
	defURL := servicedef.DefaultURL(defaultName)
	_ = defURL
	if strings.TrimSpace(os.Getenv("SERVICE_NAME")) == "" {
		_ = os.Setenv("SERVICE_NAME", defaultName)
	}
	if strings.TrimSpace(os.Getenv("HTTP_PORT")) == "" {
		_ = os.Setenv("HTTP_PORT", "8080")
	}

	cfg := config.Load()
	telemetryRuntime, err := telemetry.Init(context.Background(), cfg)
	if err != nil {
		log.Fatalf("init telemetry: %v", err)
	}
	defer func() { _ = telemetryRuntime.Shutdown(context.Background()) }()
	handler, err := NewHandler(cfg)
	if err != nil {
		log.Fatalf("build gateway handler: %v", err)
	}

	app.Logger(app.LogFields{"component": "gateway"}).Info("service starting", app.LogFields{"address": cfg.Address()})
	if err := http.ListenAndServe(cfg.Address(), handler); err != nil {
		app.Logger(app.LogFields{"component": "gateway"}).Error("run gateway failed", app.LogFields{"error": err.Error(), "address": cfg.Address()})
		log.Fatalf("run gateway: %v", err)
	}
}
