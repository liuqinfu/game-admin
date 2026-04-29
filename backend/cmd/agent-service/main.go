package main

import "game-admin/backend/internal/app"

func main() {
	app.RunNamedHTTPService("agent-service")
}
