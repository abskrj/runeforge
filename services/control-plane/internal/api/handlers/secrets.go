package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/audit"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// SecretsHandler bundles all secret-related HTTP handlers.
type SecretsHandler struct {
	store   *postgres.Store
	log     *zap.Logger
	encKey  []byte
	auditor *audit.Logger
}

// NewSecretsHandler constructs a SecretsHandler.
func NewSecretsHandler(store *postgres.Store, log *zap.Logger, encKey []byte) *SecretsHandler {
	return &SecretsHandler{store: store, log: log, encKey: encKey}
}

// WithAuditor attaches an audit logger to the SecretsHandler.
func (h *SecretsHandler) WithAuditor(a *audit.Logger) *SecretsHandler {
	h.auditor = a
	return h
}

// createSecretRequest is the expected POST body for secret/variable creation.
type createSecretRequest struct {
	Name         string   `json:"name"`
	Value        string   `json:"value"`
	IsSecret     *bool    `json:"is_secret,omitempty"` // defaults to true when absent
	SnippetID    *string  `json:"snippet_id,omitempty"`
	Environments []string `json:"environments,omitempty"`
}

// updateSecretRequest is the expected PATCH body for updating a secret/variable.
type updateSecretRequest struct {
	Name  *string `json:"name,omitempty"`
	Value *string `json:"value,omitempty"`
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

	isSecret := true
	if req.IsSecret != nil {
		isSecret = *req.IsSecret
	}

	sec, err := h.store.CreateSecret(r.Context(), tenant.ID, req.SnippetID, req.Name, req.Value, isSecret, req.Environments, h.encKey)
	if err != nil {
		h.log.Error("create secret failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create secret")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "secret_create",
			ResourceID: sec.ID,
			Metadata:   auditMeta(map[string]any{"name": req.Name}),
		})
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

	secrets, err := h.store.ListSecrets(r.Context(), tenant.ID, h.encKey)
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

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "secret_delete",
			ResourceID: secretID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateSecret handles PATCH /v1/secrets/{secretID}.
// Allows updating name and/or value. For credentials (is_secret=true),
// the new value replaces the existing one without ever exposing the old value.
func (h *SecretsHandler) UpdateSecret(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	secretID := chi.URLParam(r, "secretID")

	var req updateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == nil && req.Value == nil {
		writeError(w, http.StatusBadRequest, "name or value is required")
		return
	}

	sec, err := h.store.UpdateSecret(r.Context(), secretID, tenant.ID, req.Name, req.Value, h.encKey)
	if err != nil {
		h.log.Error("update secret failed", zap.String("id", secretID), zap.Error(err))
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	writeJSON(w, http.StatusOK, sec)
}
