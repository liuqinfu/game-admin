package main

import "game-admin/backend/internal/worker"

func main() {
	worker.RunNamed("data-platform-sync-service", worker.RuntimeOptions{
		RequireDatabase: true,
		RequireRedis:    true,
		RequireMQ:       true,
		ConsumerHandler: worker.DataPlatformSyncHandler,
	})
}
