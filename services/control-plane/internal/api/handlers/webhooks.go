package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"github.com/abskrj/velane/services/control-plane/internal/scheduler"
	"go.uber.org/zap"
)

// WebhookStore defines the store operations required by the webhook handler.
type WebhookStore interface {
	GetGitIntegrationBySnippetID(ctx context.Context, snippetID string) (*models.GitIntegration, error)
	GetSnippetByID(ctx context.Context, id string) (*models.Snippet, error)
	CreateVersion(ctx context.Context, snippetID, code, inputSchema, outputSchema, createdBy string, timeoutMs, maxMemoryMB, maxCPUPercent int) (*models.SnippetVersion, error)
	PublishVersion(ctx context.Context, versionID, env string) (*models.SnippetVersion, error)
}

// WebhookHandler handles inbound git provider webhook events.
type WebhookHandler struct {
	store WebhookStore
	sched *scheduler.Scheduler
	log   *zap.Logger
}

// NewWebhookHandler constructs a WebhookHandler.
func NewWebhookHandler(store WebhookStore, sched *scheduler.Scheduler, log *zap.Logger) *WebhookHandler {
	return &WebhookHandler{store: store, sched: sched, log: log}
}

// githubPushPayload is the minimal webhook payload we parse.
// snippet_code is a custom demo field — in production you would fetch the
// file content from the GitHub API using the commit SHA.
type githubPushPayload struct {
	Ref         string `json:"ref"`
	SnippetCode string `json:"snippet_code"`
}

// GitWebhook handles POST /v1/webhooks/git/{snippetID}.
//
// Push to main/master → publish to staging.
// Push tag v* → publish to prod.
// Push to any other branch → publish to dev.
func (h *WebhookHandler) GitWebhook(w http.ResponseWriter, r *http.Request) {
	snippetID := chi.URLParam(r, "snippetID")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	gi, err := h.store.GetGitIntegrationBySnippetID(r.Context(), snippetID)
	if err != nil {
		h.log.Debug("git integration not found", zap.String("snippet_id", snippetID), zap.Error(err))
		writeError(w, http.StatusNotFound, "git integration not found")
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !verifyGitHubSignature(body, gi.Secret, sig) {
		writeError(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if payload.SnippetCode == "" {
		writeError(w, http.StatusBadRequest, "snippet_code is required in payload")
		return
	}

	env := refToEnv(payload.Ref)

	snippet, err := h.store.GetSnippetByID(r.Context(), gi.SnippetID)
	if err != nil {
		h.log.Error("webhook: snippet not found", zap.String("snippet_id", gi.SnippetID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "snippet not found")
		return
	}

	version, err := h.store.CreateVersion(r.Context(),
		snippet.ID, payload.SnippetCode, "{}", "{}", "git-webhook", 30000, 128, 100,
	)
	if err != nil {
		h.log.Error("webhook: create version failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create version")
		return
	}

	published, err := h.store.PublishVersion(r.Context(), version.ID, env)
	if err != nil {
		h.log.Error("webhook: publish version failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to publish version")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"snippet_id":     snippet.ID,
		"version_number": published.VersionNumber,
		"env":            env,
		"ref":            payload.Ref,
		"status":         published.Status,
	})
}

// refToEnv maps a git ref to a deployment environment.
func refToEnv(ref string) string {
	if strings.HasPrefix(ref, "refs/tags/v") {
		return "prod"
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	if branch == "main" || branch == "master" {
		return "staging"
	}
	return "dev"
}

// verifyGitHubSignature verifies the HMAC-SHA256 signature from GitHub.
func verifyGitHubSignature(body []byte, secret, signatureHeader string) bool {
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}
