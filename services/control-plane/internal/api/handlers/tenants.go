package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runeforge/control-plane/internal/api/middleware"
	"github.com/runeforge/control-plane/internal/audit"
	"github.com/runeforge/control-plane/internal/models"
	"github.com/runeforge/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// TenantsHandler bundles all tenant-related HTTP handlers.
type TenantsHandler struct {
	store   *postgres.Store
	log     *zap.Logger
	auditor *audit.Logger
}

// NewTenantsHandler constructs a TenantsHandler.
func NewTenantsHandler(store *postgres.Store, log *zap.Logger) *TenantsHandler {
	return &TenantsHandler{store: store, log: log}
}

// WithAuditor attaches an audit logger to the TenantsHandler.
func (h *TenantsHandler) WithAuditor(a *audit.Logger) *TenantsHandler {
	h.auditor = a
	return h
}

// createTenantRequest is the expected POST body.
type createTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// CreateTenant handles POST /v1/tenants.
//
// NOTE: This endpoint is intentionally unauthenticated to allow bootstrapping
// a fresh installation. In production deployments it should be protected by a
// network-level control or an admin secret header before being exposed publicly.
func (h *TenantsHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	tenant, err := h.store.CreateTenant(r.Context(), req.Name, req.Slug)
	if err != nil {
		h.log.Error("create tenant failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create tenant")
		return
	}

	writeJSON(w, http.StatusCreated, tenant)
}

// createAPIKeyRequest is the expected POST body for key creation.
type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// CreateAPIKey handles POST /v1/tenants/{tenantSlug}/api-keys.
// Requires "admin" scope on the calling key.
func (h *TenantsHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	tenant, err := h.store.GetTenantBySlug(r.Context(), tenantSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Scopes) == 0 {
		req.Scopes = []string{"invoke"}
	}

	key, plain, err := h.store.CreateAPIKeyWithPlain(r.Context(), tenant.ID, req.Name, req.Scopes)
	if err != nil {
		h.log.Error("create api key failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	// Return the plain key only once — it cannot be recovered after this.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"tenant_id":  key.TenantID,
		"name":       key.Name,
		"scopes":     key.Scopes,
		"key":        plain, // one-time reveal
		"key_prefix": key.KeyPrefix,
		"created_at": key.CreatedAt,
	})
}

// updateEgressPolicyRequest is the expected PUT body for egress policy updates.
type updateEgressPolicyRequest struct {
	BlockedCIDRs   []string `json:"blocked_cidrs"`
	BlockedDomains []string `json:"blocked_domains"`
}

// GetEgressPolicy handles GET /v1/tenants/{tenantSlug}/egress.
func (h *TenantsHandler) GetEgressPolicy(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	tenant, err := h.store.GetTenantBySlug(r.Context(), tenantSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	// Enforce tenant isolation: the authenticated tenant must match.
	authTenant := middleware.TenantFromContext(r.Context())
	if authTenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if authTenant.ID != tenant.ID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, tenant.EgressPolicy)
}

// UpdateEgressPolicy handles PUT /v1/tenants/{tenantSlug}/egress.
// Requires admin scope.
func (h *TenantsHandler) UpdateEgressPolicy(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	tenant, err := h.store.GetTenantBySlug(r.Context(), tenantSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	// Enforce tenant isolation.
	authTenant := middleware.TenantFromContext(r.Context())
	if authTenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if authTenant.ID != tenant.ID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req updateEgressPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BlockedCIDRs == nil {
		req.BlockedCIDRs = []string{}
	}
	if req.BlockedDomains == nil {
		req.BlockedDomains = []string{}
	}

	policy := models.EgressPolicy{
		BlockedCIDRs:   req.BlockedCIDRs,
		BlockedDomains: req.BlockedDomains,
	}

	updated, err := h.store.UpdateEgressPolicy(r.Context(), tenant.ID, policy)
	if err != nil {
		h.log.Error("update egress policy failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to update egress policy")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "egress_update",
			ResourceID: tenant.ID,
			Metadata: auditMeta(map[string]any{
				"blocked_cidrs":   req.BlockedCIDRs,
				"blocked_domains": req.BlockedDomains,
			}),
		})
	}

	writeJSON(w, http.StatusOK, updated.EgressPolicy)
}
