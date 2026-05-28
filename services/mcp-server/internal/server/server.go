package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/mcp-server/internal/protocol"
	"github.com/abskrj/velane/services/mcp-server/internal/tools"
)

type Server struct {
	registry *tools.Registry
}

func New(registry *tools.Registry) *Server {
	return &Server{registry: registry}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/mcp", s.handleJSONRPC)
	r.Get("/mcp/sse", s.handleSSE)
	return r
}

func (s *Server) HandleRequest(ctx context.Context, authHeader string, req protocol.Request) protocol.Response {
	if req.JSONRPC != "2.0" {
		return protocol.Error(req.ID, -32600, "invalid jsonrpc version", nil)
	}
	switch req.Method {
	case "initialize":
		return protocol.Success(req.ID, map[string]any{
			"serverInfo": map[string]any{
				"name":    "velane-mcp-server",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
		})
	case "ping":
		return protocol.Success(req.ID, map[string]any{"ok": true})
	case "tools/list":
		return protocol.Success(req.ID, map[string]any{
			"tools": s.registry.List(),
		})
	case "tools/call":
		return s.handleToolsCall(ctx, authHeader, req)
	default:
		return protocol.Error(req.ID, -32601, "method not found", map[string]any{"method": req.Method})
	}
}

func (s *Server) handleToolsCall(ctx context.Context, authHeader string, req protocol.Request) protocol.Response {
	if strings.TrimSpace(authHeader) == "" {
		return protocol.Error(req.ID, -32001, "authorization header is required", nil)
	}

	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.Error(req.ID, -32602, "invalid tools/call params", err.Error())
	}
	if params.Name == "" {
		return protocol.Error(req.ID, -32602, "tool name is required", nil)
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	result, err := s.registry.Call(ctx, authHeader, params.Name, params.Arguments)
	if err != nil {
		return protocol.Success(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": err.Error()},
			},
			"isError": true,
		})
	}

	return protocol.Success(req.ID, map[string]any{
		"structuredContent": result,
		"content": []map[string]any{
			{
				"type": "text",
				"text": fmt.Sprintf("%s completed successfully", params.Name),
			},
		},
		"isError": false,
	})
}

func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req protocol.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.Error(nil, -32700, "parse error", err.Error()))
		return
	}
	resp := s.HandleRequest(r.Context(), r.Header.Get("Authorization"), req)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
