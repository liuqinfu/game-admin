package main

import (
	"os"

	"game-admin/backend/internal/app"
)

func main() {
	app.RunMigrationServiceFromEnvOrArg(os.Args[1:])
}
