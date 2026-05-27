package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/runeforge/control-plane/internal/api/middleware"
	"github.com/runeforge/control-plane/internal/scheduler"
	"github.com/runeforge/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// ReplayHandler contains Phase 5 replay endpoints.
type ReplayHandler struct {
	store *postgres.Store
	sched *scheduler.Scheduler
	log   *zap.Logger
}

// NewReplayHandler constructs a ReplayHandler.
func NewReplayHandler(store *postgres.Store, sched *scheduler.Scheduler, log *zap.Logger) *ReplayHandler {
	return &ReplayHandler{store: store, sched: sched, log: log}
}

// ReplayInvocation handles POST /v1/invocations/{id}/replay.
func (h *ReplayHandler) ReplayInvocation(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.TenantFromContext(r.Context())
	if tenant == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	invID := chi.URLParam(r, "id")
	inv, err := h.store.GetInvocation(r.Context(), invID)
	if err != nil || inv.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "invocation not found")
		return
	}

	if !tenant.ReplayEnabled {
		writeError(w, http.StatusForbidden, "replay is disabled for this tenant")
		return
	}

	snippet, snErr := h.store.GetSnippetByID(r.Context(), inv.SnippetID)
	if snErr != nil || snippet.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}

	version, verErr := h.store.GetVersion(r.Context(), inv.VersionID)
	if verErr != nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	replayed, replayErr := h.sched.Invoke(r.Context(), scheduler.InvokeRequest{
		TenantID:      tenant.ID,
		SnippetSlug:   snippet.Slug,
		Env:           inv.Environment,
		Input:         inv.InputPayload,
		PinnedVersion: version.VersionNumber,
	})
	if replayErr != nil {
		h.log.Error("replay invoke failed", zap.String("invocation_id", invID), zap.Error(replayErr))
		writeError(w, http.StatusBadRequest, replayErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"original_invocation_id": invID,
		"replay_invocation_id":   replayed.ID,
		"status":                 replayed.Status,
		"duration_ms":            replayed.DurationMs,
		"error":                  replayed.Error,
	})
}
