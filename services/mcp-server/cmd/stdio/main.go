package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/runeforge/mcp-server/internal/controlplane"
	"github.com/runeforge/mcp-server/internal/server"
	"github.com/runeforge/mcp-server/internal/tools"
)

func main() {
	controlPlaneURL := getenv("CONTROL_PLANE_URL", "http://localhost:8080")
	authHeader := strings.TrimSpace(os.Getenv("AUTHORIZATION"))

	client := controlplane.New(controlPlaneURL)
	registry := tools.NewRegistry(client)
	srv := server.New(registry)

	if err := server.RunStdio(context.Background(), srv, os.Stdin, os.Stdout, authHeader); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
