package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runeforge/control-plane/internal/api/middleware"
	"github.com/runeforge/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// SecretsHandler bundles all secret-related HTTP handlers.
type SecretsHandler struct {
	store  *postgres.Store
	log    *zap.Logger
	encKey []byte
}

// NewSecretsHandler constructs a SecretsHandler.
func NewSecretsHandler(store *postgres.Store, log *zap.Logger, encKey []byte) *SecretsHandler {
	return &SecretsHandler{store: store, log: log, encKey: encKey}
}

// createSecretRequest is the expected POST body for secret creation.
type createSecretRequest struct {
	Name         string   `json:"name"`
	Value        string   `json:"value"`
	SnippetID    *string  `json:"snippet_id,omitempty"`
	Environments []string `json:"environments,omitempty"`
}

// CreateSecret handles POST /v1/secrets.
// Body: { name, value, snippet_id? (optional), environments? }
// Returns: Secret (without value)
func (h *SecretsHandler) CreateSecret(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req createSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Value == "" {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	if req.Environments == nil {
		req.Environments = []string{}
	}

	sec, err := h.store.CreateSecret(r.Context(), tenant.ID, req.SnippetID, req.Name, req.Value, req.Environments, h.encKey)
	if err != nil {
		h.log.Error("create secret failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create secret")
		return
	}

	writeJSON(w, http.StatusCreated, sec)
}

// ListSecrets handles GET /v1/secrets.
// Returns: []Secret (without values) for the authenticated tenant.
func (h *SecretsHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	secrets, err := h.store.ListSecrets(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list secrets failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list secrets")
		return
	}

	if secrets == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, secrets)
}

// DeleteSecret handles DELETE /v1/secrets/{secretID}.
func (h *SecretsHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	secretID := chi.URLParam(r, "secretID")
	if err := h.store.DeleteSecret(r.Context(), secretID, tenant.ID); err != nil {
		h.log.Error("delete secret failed", zap.String("id", secretID), zap.Error(err))
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
