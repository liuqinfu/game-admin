package main

import "game-admin/backend/internal/app"

func main() {
	app.RunNamedHTTPService("audit-service")
}
