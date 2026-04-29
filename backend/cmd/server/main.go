package main

import (
	"log"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
)

func main() {
	cfg := config.Load()
	application, err := app.Bootstrap(cfg)
	if err != nil {
		log.Fatalf("bootstrap application: %v", err)
	}

	log.Printf("backend listening on %s", cfg.Address())
	if err := application.Run(); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
