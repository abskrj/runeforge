package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/platformlibs"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

type LibrariesHandler struct {
	store    *postgres.Store
	log      *zap.Logger
	platLibs []platformlibs.PlatformLib
}

func NewLibrariesHandler(store *postgres.Store, log *zap.Logger, platLibs []platformlibs.PlatformLib) *LibrariesHandler {
	return &LibrariesHandler{store: store, log: log, platLibs: platLibs}
}

// ListAll handles GET /v1/libraries.
// Returns platform libs (read-only) and tenant libs merged, for the given
// ?language= filter (optional).
func (h *LibrariesHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	lang := r.URL.Query().Get("language")

	platSlice := make([]any, 0)
	for _, l := range h.platLibs {
		if lang == "" || l.Language == lang {
			lCopy := l
			lCopy.Code = "" // omit code from list responses
			platSlice = append(platSlice, lCopy)
		}
	}

	tenantLibs, err := h.store.ListLibraries(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list tenant libraries", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list libraries")
		return
	}

	type response struct {
		Platform []any `json:"platform"`
		Tenant   []any `json:"tenant"`
	}
	tenantSlice := make([]any, 0, len(tenantLibs))
	for _, l := range tenantLibs {
		tenantSlice = append(tenantSlice, l)
	}
	writeJSON(w, http.StatusOK, response{Platform: platSlice, Tenant: tenantSlice})
}

// Create handles POST /v1/libraries.
func (h *LibrariesHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Language    string `json:"language"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" || req.Language == "" {
		writeError(w, http.StatusBadRequest, "name, slug, and language are required")
		return
	}
	if req.Language != "bun" && req.Language != "python" {
		writeError(w, http.StatusBadRequest, "language must be 'bun' or 'python'")
		return
	}

	lib, err := h.store.CreateLibrary(r.Context(), tenant.ID, req.Slug, req.Language, req.Name, req.Description)
	if err != nil {
		h.log.Error("create library", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create library")
		return
	}
	writeJSON(w, http.StatusCreated, lib)
}

// Delete handles DELETE /v1/libraries/{libraryID}.
func (h *LibrariesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	id := chi.URLParam(r, "libraryID")
	if err := h.store.DeleteLibrary(r.Context(), id, tenant.ID); err != nil {
		h.log.Error("delete library", zap.String("id", id), zap.Error(err))
		writeError(w, http.StatusNotFound, "library not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListVersions handles GET /v1/libraries/{libraryID}/versions.
func (h *LibrariesHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	id := chi.URLParam(r, "libraryID")
	versions, err := h.store.ListLibraryVersions(r.Context(), id, tenant.ID)
	if err != nil {
		h.log.Error("list library versions", zap.String("library_id", id), zap.Error(err))
		writeError(w, http.StatusNotFound, "library not found")
		return
	}
	if versions == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// CreateVersion handles POST /v1/libraries/{libraryID}/versions.
func (h *LibrariesHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	libraryID := chi.URLParam(r, "libraryID")

	// Verify ownership.
	if _, err := h.store.GetLibraryByID(r.Context(), libraryID, tenant.ID); err != nil {
		writeError(w, http.StatusNotFound, "library not found")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	v, err := h.store.CreateLibraryVersion(r.Context(), libraryID, req.Code)
	if err != nil {
		h.log.Error("create library version", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create version")
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

// PublishVersion handles POST /v1/libraries/{libraryID}/versions/{num}/publish.
func (h *LibrariesHandler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	libraryID := chi.URLParam(r, "libraryID")
	numStr := chi.URLParam(r, "num")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	v, err := h.store.PublishLibraryVersion(r.Context(), libraryID, tenant.ID, num)
	if err != nil {
		h.log.Error("publish library version", zap.Error(err))
		writeError(w, http.StatusNotFound, "version not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}
