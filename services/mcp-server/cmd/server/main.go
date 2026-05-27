package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/runeforge/mcp-server/internal/controlplane"
	"github.com/runeforge/mcp-server/internal/server"
	"github.com/runeforge/mcp-server/internal/tools"
)

func main() {
	addr := getenv("PORT", "8090")
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}
	controlPlaneURL := getenv("CONTROL_PLANE_URL", "http://localhost:8080")

	client := controlplane.New(controlPlaneURL)
	registry := tools.NewRegistry(client)
	srv := server.New(registry)

	log.Printf("mcp server listening on %s and proxying to %s", addr, controlPlaneURL)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
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
