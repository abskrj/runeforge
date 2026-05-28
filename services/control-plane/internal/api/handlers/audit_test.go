package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/handlers"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
	"go.uber.org/zap"
)

// mockAPIKeyAuditStore satisfies both AuditStore and AuthStore interfaces.
type mockAPIKeyAuditStore struct {
	tenant  *models.Tenant
	apiKey  *models.APIKey
	entries []*models.AuditEntry
	err     error
}

func (m *mockAPIKeyAuditStore) GetTenantBySlug(_ context.Context, slug string) (*models.Tenant, error) {
	if m.tenant == nil || m.tenant.Slug != slug {
		return nil, fmt.Errorf("tenant not found")
	}
	return m.tenant, nil
}

func (m *mockAPIKeyAuditStore) GetTenantByID(_ context.Context, id string) (*models.Tenant, error) {
	if m.tenant == nil || m.tenant.ID != id {
		return nil, fmt.Errorf("tenant not found")
	}
	return m.tenant, nil
}

func (m *mockAPIKeyAuditStore) ValidateAPIKey(_ context.Context, plain string) (*models.APIKey, error) {
	if m.apiKey == nil {
		return nil, fmt.Errorf("invalid api key")
	}
	return m.apiKey, nil
}

func (m *mockAPIKeyAuditStore) ListAuditLog(_ context.Context, tenantID string, opts postgres.AuditQueryOpts) ([]*models.AuditEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.entries, nil
}

// buildAuditRequest creates an HTTP request with the tenant and optional API key injected into context.
func buildAuditRequest(method, path string, authTenant *models.Tenant, apiKey *models.APIKey) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := req.Context()
	if authTenant != nil {
		ctx = context.WithValue(ctx, middleware.ExportedTenantKey(), authTenant)
	}
	if apiKey != nil {
		ctx = context.WithValue(ctx, middleware.ExportedAPIKeyKey(), apiKey)
	}
	return req.WithContext(ctx)
}

// TestListAuditLog_AdminOnly verifies that a request from a different tenant gets 403.
func TestListAuditLog_AdminOnly(t *testing.T) {
	tenant := &models.Tenant{ID: "t1", Slug: "acme", Name: "Acme"}
	store := &mockAPIKeyAuditStore{tenant: tenant}
	log, _ := zap.NewDevelopment()
	h := handlers.NewAuditHandler(store, log)

	// Build router with chi param.
	r := chi.NewRouter()
	r.Get("/v1/tenants/{slug}/audit-log", h.ListAuditLog)

	// Inject a DIFFERENT tenant in context — should get 403.
	otherTenant := &models.Tenant{ID: "t2", Slug: "other", Name: "Other"}
	req := httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/audit-log", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ExportedTenantKey(), otherTenant))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestListAuditLog_ReturnsEntries verifies that the correct tenant gets its entries.
func TestListAuditLog_ReturnsEntries(t *testing.T) {
	tenant := &models.Tenant{ID: "t1", Slug: "acme", Name: "Acme"}
	entries := []*models.AuditEntry{
		{
			ID:        "entry-1",
			TenantID:  "t1",
			ActorID:   "user-1",
			ActorType: "user",
			Action:    "publish",
			CreatedAt: time.Now(),
		},
	}
	store := &mockAPIKeyAuditStore{tenant: tenant, entries: entries}
	log, _ := zap.NewDevelopment()
	h := handlers.NewAuditHandler(store, log)

	r := chi.NewRouter()
	r.Get("/v1/tenants/{slug}/audit-log", h.ListAuditLog)

	// Inject the SAME tenant in context.
	req := httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/audit-log", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ExportedTenantKey(), tenant))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var result []*models.AuditEntry
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result))
	}
	if result[0].Action != "publish" {
		t.Errorf("expected action 'publish', got %q", result[0].Action)
	}
}
