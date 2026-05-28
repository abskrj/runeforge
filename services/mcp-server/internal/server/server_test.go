package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abskrj/velane/services/mcp-server/internal/controlplane"
	"github.com/abskrj/velane/services/mcp-server/internal/protocol"
	"github.com/abskrj/velane/services/mcp-server/internal/server"
	"github.com/abskrj/velane/services/mcp-server/internal/tools"
)

func TestInitialize(t *testing.T) {
	srv := server.New(tools.NewRegistry(controlplane.New("http://localhost:1")))
	resp := srv.HandleRequest(context.Background(), "Bearer test", protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestToolsList(t *testing.T) {
	srv := server.New(tools.NewRegistry(controlplane.New("http://localhost:1")))
	resp := srv.HandleRequest(context.Background(), "Bearer test", protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(raw), "list_snippets") {
		t.Fatalf("expected tool list to include list_snippets: %s", string(raw))
	}
}

func TestToolsCallListSnippets(t *testing.T) {
	cp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test" {
			http.Error(w, `{"error":"bad auth"}`, http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/v1/snippets" {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"sn_1","slug":"hello"}]`))
	}))
	defer cp.Close()

	srv := server.New(tools.NewRegistry(controlplane.New(cp.URL)))
	params := map[string]any{
		"name":      "list_snippets",
		"arguments": map[string]any{},
	}
	pb, _ := json.Marshal(params)
	resp := srv.HandleRequest(context.Background(), "Bearer test", protocol.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  pb,
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(b), "structuredContent") {
		t.Fatalf("expected structuredContent in result: %s", string(b))
	}
}

func TestHandleJSONRPCEndpoint(t *testing.T) {
	cp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer cp.Close()

	srv := server.New(tools.NewRegistry(controlplane.New(cp.URL)))
	httpSrv := httptest.NewServer(srv.Router())
	defer httpSrv.Close()

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	req, _ := http.NewRequest(http.MethodPost, httpSrv.URL+"/mcp", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
}
