package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// UsageStore is the subset of *postgres.Store that the usage handler needs.
type UsageStore interface {
	GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error)
	ListSnippets(ctx context.Context, tenantID string) ([]*models.Snippet, error)
	GetSnippetMetrics(ctx context.Context, snippetID, window string, since time.Time) (*postgres.SnippetMetrics, error)
}

// UsageHandler handles usage aggregation endpoints.
type UsageHandler struct {
	store UsageStore
	log   *zap.Logger
}

// NewUsageHandler constructs a UsageHandler.
func NewUsageHandler(store UsageStore, log *zap.Logger) *UsageHandler {
	return &UsageHandler{store: store, log: log}
}

type topSnippet struct {
	SnippetID   string  `json:"snippet_id"`
	Name        string  `json:"name"`
	Invocations int64   `json:"invocations"`
	P95Ms       float64 `json:"p95_ms"`
}

// GetUsage handles GET /v1/tenants/{slug}/usage?window=24h|7d|30d.
func (h *UsageHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	tenant, err := h.store.GetTenantBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	authTenant := middleware.TenantFromContext(r.Context())
	if authTenant == nil || authTenant.ID != tenant.ID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	window := r.URL.Query().Get("window")
	var since time.Time
	switch window {
	case "", "24h":
		window = "24h"
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	default:
		writeError(w, http.StatusBadRequest, "window must be one of: 24h, 7d, 30d")
		return
	}

	snippets, err := h.store.ListSnippets(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list snippets for usage failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}

	var (
		totalInvocations int64
		totalErrors      int64
		totalDurationMs  float64
		topSnippets      []topSnippet
	)

	for _, sn := range snippets {
		m, err := h.store.GetSnippetMetrics(r.Context(), sn.ID, window, since)
		if err != nil {
			h.log.Warn("metrics query failed for snippet", zap.String("snippet_id", sn.ID), zap.Error(err))
			continue
		}
		totalInvocations += int64(m.TotalCount)
		totalErrors += int64(m.Failed)
		if m.TotalCount > 0 {
			totalDurationMs += m.AvgDurationMs * float64(m.TotalCount)
		}
		if m.TotalCount > 0 {
			topSnippets = append(topSnippets, topSnippet{
				SnippetID:   sn.ID,
				Name:        sn.Name,
				Invocations: int64(m.TotalCount),
				P95Ms:       m.P95DurationMs,
			})
		}
	}

	var avgDurationMs float64
	if totalInvocations > 0 {
		avgDurationMs = totalDurationMs / float64(totalInvocations)
	}

	var errorRate float64
	if totalInvocations > 0 {
		errorRate = float64(totalErrors) / float64(totalInvocations)
	}

	// Sort top_snippets by invocations descending (simple bubble sort, small lists expected).
	for i := 0; i < len(topSnippets); i++ {
		for j := i + 1; j < len(topSnippets); j++ {
			if topSnippets[j].Invocations > topSnippets[i].Invocations {
				topSnippets[i], topSnippets[j] = topSnippets[j], topSnippets[i]
			}
		}
	}
	// Cap at top 10.
	if len(topSnippets) > 10 {
		topSnippets = topSnippets[:10]
	}
	if topSnippets == nil {
		topSnippets = []topSnippet{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":         tenant.ID,
		"window":            window,
		"total_invocations": totalInvocations,
		"error_rate":        errorRate,
		"avg_duration_ms":   avgDurationMs,
		"top_snippets":      topSnippets,
	})
}
