package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// WebhookClient delivers invocation completion events to caller-supplied URLs.
type WebhookClient struct {
	httpClient *http.Client
	log        *zap.Logger
}

// newWebhookClient creates a WebhookClient with a sensible timeout.
func newWebhookClient(log *zap.Logger) *WebhookClient {
	return &WebhookClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		log:        log,
	}
}

// NewWebhookClientForTest creates a WebhookClient suitable for use in tests.
func NewWebhookClientForTest(log *zap.Logger) *WebhookClient {
	return newWebhookClient(log)
}

// WebhookPayload is the JSON body posted to the callback URL.
type WebhookPayload struct {
	InvocationID string `json:"invocation_id"`
	Status       string `json:"status"`
	Output       string `json:"output,omitempty"`
	Error        string `json:"error,omitempty"`
	DurationMs   int    `json:"duration_ms"`
}

// Deliver POSTs the payload to the callback URL. Best-effort — failures are
// logged but not retried in Phase 2.
func (w *WebhookClient) Deliver(ctx context.Context, url string, payload WebhookPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		w.log.Error("webhook: marshal payload", zap.String("url", url), zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		w.log.Error("webhook: build request", zap.String("url", url), zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.log.Warn("webhook: delivery failed", zap.String("url", url), zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		w.log.Warn("webhook: non-2xx response",
			zap.String("url", url),
			zap.Int("status", resp.StatusCode),
		)
		return
	}

	w.log.Debug("webhook: delivered", zap.String("url", url), zap.Int("status", resp.StatusCode))
}
