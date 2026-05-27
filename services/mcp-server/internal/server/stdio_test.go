package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/runeforge/mcp-server/internal/controlplane"
	"github.com/runeforge/mcp-server/internal/server"
	"github.com/runeforge/mcp-server/internal/tools"
)

func TestRunStdio(t *testing.T) {
	srv := server.New(tools.NewRegistry(controlplane.New("http://localhost:1")))
	in := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	out := &bytes.Buffer{}

	if err := server.RunStdio(context.Background(), srv, in, out, "Bearer test"); err != nil {
		t.Fatalf("RunStdio failed: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if resp["result"] == nil {
		t.Fatalf("expected result in stdio response: %s", out.String())
	}
}
