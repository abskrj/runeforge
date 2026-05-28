package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// MetricsHandler contains Phase 5 metrics endpoints.
type MetricsHandler struct {
	store *postgres.Store
	log   *zap.Logger
}

// NewMetricsHandler constructs a MetricsHandler.
func NewMetricsHandler(store *postgres.Store, log *zap.Logger) *MetricsHandler {
	return &MetricsHandler{store: store, log: log}
}

func parseMetricsWindow(window string) (time.Duration, string, bool) {
	switch window {
	case "", "24h":
		return 24 * time.Hour, "24h", true
	case "1h":
		return time.Hour, "1h", true
	case "7d":
		return 7 * 24 * time.Hour, "7d", true
	default:
		return 0, "", false
	}
}

// GetSnippetMetrics handles GET /v1/metrics/snippets/{snippetID}.
func (h *MetricsHandler) GetSnippetMetrics(w http.ResponseWriter, r *http.Request) {
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

	duration, normalized, ok := parseMetricsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeError(w, http.StatusBadRequest, "window must be one of: 1h, 24h, 7d")
		return
	}

	since := time.Now().Add(-duration)
	metrics, metricsErr := h.store.GetSnippetMetrics(r.Context(), snippetID, normalized, since)
	if metricsErr != nil {
		h.log.Error("get metrics failed", zap.Error(metricsErr), zap.String("snippet_id", snippetID))
		writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"snippet_id": snippetID,
		"window":     metrics.Window,
		"aggregates": map[string]any{
			"total_count":     metrics.TotalCount,
			"completed_count": metrics.Completed,
			"failed_count":    metrics.Failed,
			"avg_duration_ms": metrics.AvgDurationMs,
			"p50_duration_ms": metrics.P50DurationMs,
			"p95_duration_ms": metrics.P95DurationMs,
			"p99_duration_ms": metrics.P99DurationMs,
		},
		"series": metrics.Series,
	})
}
