package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/worker"
	"go.uber.org/zap"
)

func TestWebhookClient_Deliver(t *testing.T) {
	var (
		capturedMethod   string
		capturedBody     []byte
		capturedCTHeader string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedCTHeader = r.Header.Get("Content-Type")
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wc := worker.NewWebhookClientForTest(zap.NewNop())

	payload := worker.WebhookPayload{
		InvocationID: "inv-abc",
		Status:       "completed",
		Output:       `{"ok":true}`,
		DurationMs:   42,
	}

	wc.Deliver(context.Background(), srv.URL+"/callback", payload)

	if capturedMethod != http.MethodPost {
		t.Errorf("method = %q; want POST", capturedMethod)
	}
	if capturedCTHeader != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", capturedCTHeader)
	}

	var got worker.WebhookPayload
	if err := json.Unmarshal(capturedBody, &got); err != nil {
		t.Fatalf("unmarshal body: %v\nbody: %s", err, capturedBody)
	}
	if got.InvocationID != payload.InvocationID {
		t.Errorf("InvocationID = %q; want %q", got.InvocationID, payload.InvocationID)
	}
	if got.Status != payload.Status {
		t.Errorf("Status = %q; want %q", got.Status, payload.Status)
	}
	if got.Output != payload.Output {
		t.Errorf("Output = %q; want %q", got.Output, payload.Output)
	}
	if got.DurationMs != payload.DurationMs {
		t.Errorf("DurationMs = %d; want %d", got.DurationMs, payload.DurationMs)
	}
}

func TestWebhookClient_DeliverNonOK(t *testing.T) {
	// Verify Deliver does not panic on non-2xx response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	wc := worker.NewWebhookClientForTest(zap.NewNop())
	// Must not panic.
	wc.Deliver(context.Background(), srv.URL+"/bad", worker.WebhookPayload{
		InvocationID: "inv-1",
		Status:       "failed",
	})
}

func TestWebhookClient_DeliverUnreachable(t *testing.T) {
	wc := worker.NewWebhookClientForTest(zap.NewNop())
	// Must not panic or block indefinitely (client has a 10s timeout).
	wc.Deliver(context.Background(), "http://127.0.0.1:1/unreachable", worker.WebhookPayload{
		InvocationID: "inv-1",
		Status:       "completed",
	})
}
