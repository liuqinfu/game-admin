package main

import (
	"fmt"
	"os"
	"slices"

	"game-admin/backend/internal/servicedef"
)

func main() {
	mode := "all"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	var items []string
	switch mode {
	case "http":
		items = servicedef.HTTPNames()
	case "worker":
		items = servicedef.WorkerNames()
	case "all":
		items = append(servicedef.HTTPNames(), servicedef.WorkerNames()...)
		items = append(items, "gateway-service", "migrate-service")
	default:
		fmt.Fprintf(os.Stderr, "unsupported mode: %s\n", mode)
		os.Exit(1)
	}

	slices.Sort(items)
	for _, item := range items {
		fmt.Println(item)
	}
}
