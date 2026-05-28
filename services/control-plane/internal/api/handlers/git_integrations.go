package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runeforge/control-plane/internal/api/middleware"
	"github.com/runeforge/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// GitIntegrationHandler bundles git integration HTTP handlers.
type GitIntegrationHandler struct {
	store *postgres.Store
	log   *zap.Logger
}

// NewGitIntegrationHandler constructs a GitIntegrationHandler.
func NewGitIntegrationHandler(store *postgres.Store, log *zap.Logger) *GitIntegrationHandler {
	return &GitIntegrationHandler{store: store, log: log}
}

type createGitIntegrationRequest struct {
	Provider string `json:"provider"`
	RepoURL  string `json:"repo_url"`
}

// Create handles POST /v1/snippets/{id}/git-integration.
// Generates a random HMAC secret, stores it, and returns it ONCE in the response.
func (h *GitIntegrationHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	// Verify the snippet exists and belongs to this tenant before creating.
	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	var req createGitIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Provider == "" || req.RepoURL == "" {
		writeError(w, http.StatusBadRequest, "provider and repo_url are required")
		return
	}

	if req.Provider != "github" && req.Provider != "gitlab" {
		writeError(w, http.StatusBadRequest, "provider must be 'github' or 'gitlab'")
		return
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		h.log.Error("generate webhook secret", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to generate secret")
		return
	}
	secret := hex.EncodeToString(secretBytes)

	gi, err := h.store.CreateGitIntegration(r.Context(), tenant.ID, snippetID, req.Provider, req.RepoURL, secret)
	if err != nil {
		h.log.Error("create git integration failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create git integration")
		return
	}

	// Return the secret once — it is never returned again.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         gi.ID,
		"tenant_id":  gi.TenantID,
		"snippet_id": gi.SnippetID,
		"provider":   gi.Provider,
		"repo_url":   gi.RepoURL,
		"secret":     secret,
		"created_at": gi.CreatedAt,
		"webhook_url": "/v1/webhooks/git/" + gi.SnippetID,
	})
}

// Get handles GET /v1/snippets/{id}/git-integration.
// Does NOT return the secret.
func (h *GitIntegrationHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	gi, err := h.store.GetGitIntegrationBySnippet(r.Context(), tenant.ID, snippetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "git integration not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          gi.ID,
		"tenant_id":   gi.TenantID,
		"snippet_id":  gi.SnippetID,
		"provider":    gi.Provider,
		"repo_url":    gi.RepoURL,
		"created_at":  gi.CreatedAt,
		"webhook_url": "/v1/webhooks/git/" + gi.SnippetID,
	})
}

// Delete handles DELETE /v1/snippets/{id}/git-integration.
func (h *GitIntegrationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	if err := h.store.DeleteGitIntegration(r.Context(), tenant.ID, snippetID); err != nil {
		h.log.Error("delete git integration failed", zap.Error(err))
		writeError(w, http.StatusNotFound, "git integration not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
