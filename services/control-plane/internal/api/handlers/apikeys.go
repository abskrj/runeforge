package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runeforge/control-plane/internal/audit"
	"github.com/runeforge/control-plane/internal/models"
	"go.uber.org/zap"
)

// APIKeysStore is the subset of *postgres.Store that API key management handlers need.
type APIKeysStore interface {
	GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error)
	ListAPIKeys(ctx context.Context, tenantID string) ([]*models.APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
}

// APIKeysHandler handles API key list and revoke endpoints.
type APIKeysHandler struct {
	store   APIKeysStore
	log     *zap.Logger
	auditor *audit.Logger
}

// NewAPIKeysHandler constructs an APIKeysHandler.
func NewAPIKeysHandler(store APIKeysStore, log *zap.Logger) *APIKeysHandler {
	return &APIKeysHandler{store: store, log: log}
}

// WithAuditor attaches an audit logger to the APIKeysHandler.
func (h *APIKeysHandler) WithAuditor(a *audit.Logger) *APIKeysHandler {
	h.auditor = a
	return h
}

// safeAPIKey is the public representation of an API key — no raw key or hash.
type safeAPIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	KeyPrefix  string     `json:"key_prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ListAPIKeys handles GET /v1/tenants/{slug}/api-keys.
// Returns key metadata only — no raw key values.
func (h *APIKeysHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	tenant, err := h.store.GetTenantBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	keys, err := h.store.ListAPIKeys(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list api keys failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}

	result := make([]safeAPIKey, 0, len(keys))
	for _, k := range keys {
		result = append(result, safeAPIKey{
			ID:         k.ID,
			Name:       k.Name,
			Scopes:     k.Scopes,
			KeyPrefix:  k.KeyPrefix,
			LastUsedAt: k.LastUsedAt,
			CreatedAt:  k.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteAPIKey handles DELETE /v1/tenants/{slug}/api-keys/{id}.
func (h *APIKeysHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	keyID := chi.URLParam(r, "keyID")

	tenant, err := h.store.GetTenantBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	if err := h.store.DeleteAPIKey(r.Context(), keyID); err != nil {
		h.log.Error("delete api key failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to delete api key")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "api_key_revoke",
			ResourceID: keyID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
