package app

import (
	"log"
	"os"
	"strings"

	"game-admin/backend/internal/config"
	"game-admin/backend/internal/servicedef"
)

func RunNamedMigrationService(name string) {
	def := servicedef.MustHTTP(name)
	applyDefaultEnv("SERVICE_NAME", def.Name)
	if len(def.Modules) > 0 {
		applyDefaultEnv("SERVICE_MODULES", strings.Join(def.Modules, ","))
	}

	cfg := config.Load()
	db, err := OpenDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	log.Printf("run scoped migrations for %s modules=%s", cfg.Service.Name, strings.Join(cfg.Service.Modules, ","))
	if err := AutoMigrateModules(db, cfg.Service.Modules); err != nil {
		log.Fatalf("run scoped migrations: %v", err)
	}
	log.Printf("scoped migrations finished for %s", cfg.Service.Name)
}

func RunMigrationServiceFromEnvOrArg(args []string) {
	serviceName := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		serviceName = strings.TrimSpace(args[0])
	}
	if serviceName == "" {
		log.Fatalf("service name is required, pass arg or set SERVICE_NAME")
	}
	RunNamedMigrationService(serviceName)
}
