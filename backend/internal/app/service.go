package app

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"game-admin/backend/internal/config"
	"game-admin/backend/internal/servicedef"
	"game-admin/backend/internal/telemetry"
)

func RunHTTPService(defaultName, defaultPort string, defaultModules ...string) {
	applyDefaultEnv("SERVICE_NAME", defaultName)
	applyDefaultEnv("HTTP_PORT", defaultPort)
	if len(defaultModules) > 0 {
		applyDefaultEnv("SERVICE_MODULES", strings.Join(defaultModules, ","))
	}

	cfg := config.Load()
	telemetryRuntime, err := telemetry.Init(context.Background(), cfg)
	if err != nil {
		log.Fatalf("init telemetry: %v", err)
	}
	defer func() { _ = telemetryRuntime.Shutdown(context.Background()) }()
	application, err := Bootstrap(cfg)
	if err != nil {
		log.Fatalf("bootstrap application: %v", err)
	}

	logStartup(cfg)
	if err := application.Run(); err != nil {
		Logger(LogFields{"component": "runtime"}).Error("run application failed", LogFields{"error": err.Error(), "address": cfg.Address()})
		log.Fatalf("run application: %v", err)
	}
}

func logStartup(cfg config.Config) {
	modules := append([]string(nil), cfg.Service.Modules...)
	payload := map[string]any{
		"service": cfg.Service.Name,
		"address": cfg.Address(),
		"modules": modules,
	}
	if encoded, err := json.Marshal(payload); err == nil {
		Logger(LogFields{"component": "runtime"}).Info("service starting", LogFields{"startup": json.RawMessage(encoded)})
		return
	}
	Logger(LogFields{"component": "runtime"}).Info("service starting", LogFields{"address": cfg.Address(), "modules": modules})
}

func RunNamedHTTPService(name string) {
	def := servicedef.MustHTTP(name)
	RunHTTPService(def.Name, def.DefaultPort, def.Modules...)
}

func applyDefaultEnv(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, value)
}
