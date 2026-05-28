package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// SnippetsHandler bundles all snippet-related HTTP handlers.
type SnippetsHandler struct {
	store *postgres.Store
	log   *zap.Logger
}

// NewSnippetsHandler constructs a SnippetsHandler.
func NewSnippetsHandler(store *postgres.Store, log *zap.Logger) *SnippetsHandler {
	return &SnippetsHandler{store: store, log: log}
}

// ListSnippets handles GET /v1/snippets.
func (h *SnippetsHandler) ListSnippets(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippets, err := h.store.ListSnippets(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list snippets failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list snippets")
		return
	}

	if snippets == nil {
		snippets = []*models.Snippet{}
	}
	writeJSON(w, http.StatusOK, snippets)
}

// createSnippetRequest is the expected POST body.
type createSnippetRequest struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Language string `json:"language"`
}

// CreateSnippet handles POST /v1/snippets.
func (h *SnippetsHandler) CreateSnippet(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	// Resolve the creator ID from whichever auth path is active.
	createdBy := ""
	if key := middleware.APIKeyFromContext(r.Context()); key != nil {
		createdBy = key.ID
	} else if user := middleware.SessionUserFromContext(r.Context()); user != nil {
		createdBy = user.ID
	}

	var req createSnippetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" || req.Language == "" {
		writeError(w, http.StatusBadRequest, "name, slug, and language are required")
		return
	}

	if req.Language != string(models.LanguageBun) && req.Language != string(models.LanguagePython) {
		writeError(w, http.StatusBadRequest, "language must be 'bun' or 'python'")
		return
	}

	snippet, err := h.store.CreateSnippet(r.Context(), tenant.ID, req.Name, req.Slug, req.Language, createdBy)
	if err != nil {
		h.log.Error("create snippet failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create snippet")
		return
	}

	writeJSON(w, http.StatusCreated, snippet)
}

// GetSnippet handles GET /v1/snippets/{snippetID}.
func (h *SnippetsHandler) GetSnippet(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")
	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	// Enforce tenant isolation.
	if snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	writeJSON(w, http.StatusOK, snippet)
}

// DeleteSnippet handles DELETE /v1/snippets/{snippetID}.
func (h *SnippetsHandler) DeleteSnippet(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	// Verify ownership before deletion.
	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}
	if snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	if err := h.store.DeleteSnippet(r.Context(), snippetID); err != nil {
		h.log.Error("delete snippet failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to delete snippet")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
