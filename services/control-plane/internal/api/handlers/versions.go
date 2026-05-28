package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/runeforge/control-plane/internal/api/middleware"
	"github.com/runeforge/control-plane/internal/audit"
	"github.com/runeforge/control-plane/internal/models"
	"github.com/runeforge/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// VersionsHandler bundles all snippet version HTTP handlers.
type VersionsHandler struct {
	store  *postgres.Store
	log    *zap.Logger
	auditor *audit.Logger
}

// NewVersionsHandler constructs a VersionsHandler.
func NewVersionsHandler(store *postgres.Store, log *zap.Logger) *VersionsHandler {
	return &VersionsHandler{store: store, log: log}
}

// WithAuditor attaches an audit logger to the VersionsHandler.
func (h *VersionsHandler) WithAuditor(a *audit.Logger) *VersionsHandler {
	h.auditor = a
	return h
}

// isValidEnv returns true if env is one of the permitted values.
func isValidEnv(env string) bool {
	return env == "dev" || env == "staging" || env == "prod"
}

// ListVersions handles GET /v1/snippets/{snippetID}/versions.
func (h *VersionsHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	// Verify snippet belongs to tenant.
	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	versions, err := h.store.ListVersions(r.Context(), snippetID)
	if err != nil {
		h.log.Error("list versions failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list versions")
		return
	}

	if versions == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// createVersionRequest is the expected POST body.
type createVersionRequest struct {
	Code          string `json:"code"`
	InputSchema   string `json:"input_schema"`
	OutputSchema  string `json:"output_schema"`
	TimeoutMs     int    `json:"timeout_ms"`
	MaxMemoryMB   int    `json:"max_memory_mb"`
	MaxCPUPercent int    `json:"max_cpu_percent"`
}

// CreateVersion handles POST /v1/snippets/{snippetID}/versions.
func (h *VersionsHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	key := middleware.APIKeyFromContext(r.Context())
	if tenant == nil || key == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	var req createVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	// Apply defaults.
	if req.InputSchema == "" {
		req.InputSchema = "{}"
	}
	if req.OutputSchema == "" {
		req.OutputSchema = "{}"
	}
	if req.TimeoutMs <= 0 {
		req.TimeoutMs = 30000
	}
	if req.MaxMemoryMB <= 0 {
		req.MaxMemoryMB = 128
	}
	if req.MaxCPUPercent <= 0 {
		req.MaxCPUPercent = 100
	}

	version, err := h.store.CreateVersion(r.Context(),
		snippetID, req.Code, req.InputSchema, req.OutputSchema,
		key.ID, req.TimeoutMs, req.MaxMemoryMB, req.MaxCPUPercent,
	)
	if err != nil {
		h.log.Error("create version failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create version")
		return
	}

	writeJSON(w, http.StatusCreated, version)
}

// GetVersion handles GET /v1/snippets/{snippetID}/versions/{num}.
func (h *VersionsHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")
	numStr := chi.URLParam(r, "num")

	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	version, err := h.store.GetVersionByNumber(r.Context(), snippetID, num)
	if err != nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	writeJSON(w, http.StatusOK, version)
}

// PublishVersion handles POST /v1/snippets/{snippetID}/versions/{num}/publish.
// Query param: ?env=dev|staging|prod (default: prod).
func (h *VersionsHandler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")
	numStr := chi.URLParam(r, "num")

	env := r.URL.Query().Get("env")
	if env == "" {
		env = "prod"
	}
	if !isValidEnv(env) {
		writeError(w, http.StatusBadRequest, "env must be 'dev', 'staging', or 'prod'")
		return
	}

	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	version, err := h.store.GetVersionByNumber(r.Context(), snippetID, num)
	if err != nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	published, err := h.store.PublishVersion(r.Context(), version.ID, env)
	if err != nil {
		h.log.Error("publish version failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to publish version")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "publish",
			ResourceID: snippetID,
			Metadata:   auditMeta(map[string]any{"version_num": num, "env": env}),
		})
	}

	writeJSON(w, http.StatusOK, published)
}

// setCanaryRequest is the expected POST body for canary configuration.
type setCanaryRequest struct {
	VersionID string `json:"version_id"`
	Percent   int    `json:"percent"`
}

// SetCanary handles POST /v1/snippets/{snippetID}/canary.
// Body: { version_id: string, percent: int (0-100) }
func (h *VersionsHandler) SetCanary(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	var req setCanaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.VersionID == "" {
		writeError(w, http.StatusBadRequest, "version_id is required")
		return
	}
	if req.Percent < 0 || req.Percent > 100 {
		writeError(w, http.StatusBadRequest, "percent must be between 0 and 100")
		return
	}

	// Verify the requested canary version belongs to this snippet.
	canaryVer, err := h.store.GetVersion(r.Context(), req.VersionID)
	if err != nil || canaryVer.SnippetID != snippetID {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	env := r.URL.Query().Get("env")
	if env == "" {
		env = "prod"
	}
	if !isValidEnv(env) {
		writeError(w, http.StatusBadRequest, "env must be 'dev', 'staging', or 'prod'")
		return
	}

	se, err := h.store.SetCanary(r.Context(), snippetID, env, req.VersionID, req.Percent)
	if err != nil {
		h.log.Error("set canary failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to set canary")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "canary_set",
			ResourceID: snippetID,
			Metadata:   auditMeta(map[string]any{"version_id": req.VersionID, "percent": req.Percent, "env": env}),
		})
	}

	writeJSON(w, http.StatusOK, se)
}

// ClearCanary handles DELETE /v1/snippets/{snippetID}/canary.
// Query: ?env=prod (default prod)
func (h *VersionsHandler) ClearCanary(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	snippetID := chi.URLParam(r, "snippetID")

	snippet, err := h.store.GetSnippetByID(r.Context(), snippetID)
	if err != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	env := r.URL.Query().Get("env")
	if env == "" {
		env = "prod"
	}
	if !isValidEnv(env) {
		writeError(w, http.StatusBadRequest, "env must be 'dev', 'staging', or 'prod'")
		return
	}

	if err := h.store.ClearCanary(r.Context(), snippetID, env); err != nil {
		h.log.Error("clear canary failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to clear canary")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "canary_clear",
			ResourceID: snippetID,
			Metadata:   auditMeta(map[string]any{"env": env}),
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
