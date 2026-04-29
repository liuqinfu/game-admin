package main

import "game-admin/backend/internal/worker"

func main() {
	worker.RunNamed("notification-service", worker.RuntimeOptions{
		RequireDatabase: true,
		RequireRedis:    true,
		RequireMQ:       true,
		ConsumerHandler: worker.NotificationHandler,
	})
}
