package remote_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runeforge/control-plane/internal/executor"
	"github.com/runeforge/control-plane/internal/executor/remote"
)

// mockExecutorServer starts an httptest server that responds with the given
// runResponse JSON on POST /run. Close the returned server after the test.
func mockExecutorServer(t *testing.T, status int, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/run" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRemoteExecutor_Run_BunSuccess(t *testing.T) {
	srv := mockExecutorServer(t, http.StatusOK, map[string]any{
		"output":         `{"result":"hello"}`,
		"stderr":         "",
		"duration_ms":    42,
		"peak_memory_mb": 12,
		"exit_code":      0,
		"error":          "",
	})
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	result := exec.Run(context.Background(), executor.RunSpec{
		Language:    "bun",
		Code:        `export default async () => ({ result: "hello" })`,
		Input:       `{}`,
		TimeoutMs:   5000,
		MaxMemoryMB: 128,
	})

	if result.Error != "" {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Output != `{"result":"hello"}` {
		t.Errorf("output = %q; want %q", result.Output, `{"result":"hello"}`)
	}
	if result.DurationMs != 42 {
		t.Errorf("duration_ms = %d; want 42", result.DurationMs)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit_code = %d; want 0", result.ExitCode)
	}
}

func TestRemoteExecutor_Run_PythonSuccess(t *testing.T) {
	srv := mockExecutorServer(t, http.StatusOK, map[string]any{
		"output":         `{"tokens":10}`,
		"stderr":         "",
		"duration_ms":    80,
		"peak_memory_mb": 30,
		"exit_code":      0,
		"error":          "",
	})
	defer srv.Close()

	exec := remote.New("http://unused", srv.URL)
	result := exec.Run(context.Background(), executor.RunSpec{
		Language:    "python",
		Code:        "async def handler(req): return {'tokens': 10}",
		Input:       `{}`,
		TimeoutMs:   5000,
		MaxMemoryMB: 128,
	})

	if result.Error != "" {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Output != `{"tokens":10}` {
		t.Errorf("output = %q; want %q", result.Output, `{"tokens":10}`)
	}
}

func TestRemoteExecutor_Run_TimeoutPropagated(t *testing.T) {
	// Server that sleeps longer than the context allows.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":"","stderr":"","duration_ms":0,"peak_memory_mb":0,"exit_code":0,"error":""}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	exec := remote.New(srv.URL, "http://unused")
	result := exec.Run(ctx, executor.RunSpec{
		Language:    "bun",
		Code:        "",
		Input:       `{}`,
		TimeoutMs:   50,
		MaxMemoryMB: 128,
	})

	if result.Error == "" {
		t.Error("expected an error due to context timeout, got none")
	}
}

func TestRemoteExecutor_Run_ServerError(t *testing.T) {
	srv := mockExecutorServer(t, http.StatusInternalServerError, map[string]any{
		"error": "executor crashed",
	})
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	result := exec.Run(context.Background(), executor.RunSpec{
		Language: "bun",
		Input:    `{}`,
	})

	if result.Error == "" {
		t.Error("expected error for 500 response, got none")
	}
	if result.ExitCode != -1 {
		t.Errorf("exit_code = %d; want -1 for server errors", result.ExitCode)
	}
}

func TestRemoteExecutor_Run_UnsupportedLanguage(t *testing.T) {
	exec := remote.New("http://bun", "http://python")
	result := exec.Run(context.Background(), executor.RunSpec{
		Language: "ruby",
		Input:    `{}`,
	})

	if result.Error == "" {
		t.Error("expected error for unsupported language, got none")
	}
	if result.ExitCode != -1 {
		t.Errorf("exit_code = %d; want -1", result.ExitCode)
	}
}

func TestRemoteExecutor_Run_ExecutorReturnsTimeout(t *testing.T) {
	srv := mockExecutorServer(t, http.StatusOK, map[string]any{
		"output":         "",
		"stderr":         "Killed: 9",
		"duration_ms":    5000,
		"peak_memory_mb": 0,
		"exit_code":      1,
		"error":          "timeout",
	})
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	result := exec.Run(context.Background(), executor.RunSpec{
		Language:  "bun",
		Input:     `{}`,
		TimeoutMs: 5000,
	})

	if result.Error != "timeout" {
		t.Errorf("error = %q; want %q", result.Error, "timeout")
	}
}

func TestRemoteExecutor_Run_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	result := exec.Run(context.Background(), executor.RunSpec{
		Language: "bun",
		Input:    `{}`,
	})

	if result.Error == "" {
		t.Error("expected error for invalid JSON response, got none")
	}
}

// --- RunStream tests ---

// mockStreamServer creates an httptest server that serves SSE events on
// POST /run/stream.
func mockStreamServer(t *testing.T, chunks []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/run/stream" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
}

func TestRemoteExecutor_RunStream_SingleChunk(t *testing.T) {
	chunks := []map[string]any{
		{"data": `{"result":"hello"}`, "done": true},
	}
	srv := mockStreamServer(t, chunks)
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	ch, err := exec.RunStream(context.Background(), executor.RunSpec{
		Language:    "bun",
		Code:        `export default async () => "hello"`,
		Input:       `{}`,
		TimeoutMs:   5000,
		MaxMemoryMB: 128,
	})

	if err != nil {
		t.Fatalf("RunStream error: %v", err)
	}

	var received []executor.StreamChunk
	for chunk := range ch {
		received = append(received, chunk)
	}

	if len(received) == 0 {
		t.Fatal("expected at least one chunk, got none")
	}
	last := received[len(received)-1]
	if !last.Done {
		t.Error("last chunk should have Done=true")
	}
}

func TestRemoteExecutor_RunStream_MultipleChunks(t *testing.T) {
	chunks := []map[string]any{
		{"data": "chunk1", "done": false},
		{"data": "chunk2", "done": false},
		{"data": "chunk3", "done": true},
	}
	srv := mockStreamServer(t, chunks)
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	ch, err := exec.RunStream(context.Background(), executor.RunSpec{
		Language: "bun",
		Input:    `{}`,
	})
	if err != nil {
		t.Fatalf("RunStream error: %v", err)
	}

	var received []executor.StreamChunk
	for chunk := range ch {
		received = append(received, chunk)
	}

	if len(received) != 3 {
		t.Errorf("got %d chunks; want 3", len(received))
	}
}

func TestRemoteExecutor_RunStream_ContextCancel(t *testing.T) {
	// Server that streams slowly; respects request context cancellation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/run/stream" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			data, _ := json.Marshal(map[string]any{"data": fmt.Sprintf("chunk%d", i), "done": false})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	exec := remote.New(srv.URL, "http://unused")
	ch, err := exec.RunStream(ctx, executor.RunSpec{
		Language: "bun",
		Input:    `{}`,
	})
	if err != nil {
		t.Fatalf("RunStream error: %v", err)
	}

	// Read one chunk then cancel.
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first chunk")
	}
	cancel()

	// Drain the channel; it should close promptly after cancel.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()

	select {
	case <-done:
		// Good — channel closed after cancel.
	case <-time.After(3 * time.Second):
		t.Error("channel did not close within 3s after context cancel")
	}
}

func TestRemoteExecutor_RunStream_UnsupportedLanguage(t *testing.T) {
	exec := remote.New("http://bun", "http://python")
	_, err := exec.RunStream(context.Background(), executor.RunSpec{
		Language: "ruby",
		Input:    `{}`,
	})
	if err == nil {
		t.Error("expected error for unsupported language, got nil")
	}
}

func TestRemoteExecutor_RunStream_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"executor crashed"}`))
	}))
	defer srv.Close()

	exec := remote.New(srv.URL, "http://unused")
	_, err := exec.RunStream(context.Background(), executor.RunSpec{
		Language: "bun",
		Input:    `{}`,
	})
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}
