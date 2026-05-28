package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/api/handlers"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

// --- mock branding store ---

type mockBrandingStore struct {
	getTenantBySlugFn func(ctx context.Context, slug string) (*models.Tenant, error)
	getBrandingFn     func(ctx context.Context, tenantID string) (*models.Branding, error)
	updateBrandingFn  func(ctx context.Context, tenantID string, b models.Branding) error
}

func (m *mockBrandingStore) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	return m.getTenantBySlugFn(ctx, slug)
}
func (m *mockBrandingStore) GetBranding(ctx context.Context, tenantID string) (*models.Branding, error) {
	return m.getBrandingFn(ctx, tenantID)
}
func (m *mockBrandingStore) UpdateBranding(ctx context.Context, tenantID string, b models.Branding) error {
	return m.updateBrandingFn(ctx, tenantID, b)
}

// TestGetBranding verifies that GET branding returns the branding config.
func TestGetBranding(t *testing.T) {
	store := &mockBrandingStore{
		getTenantBySlugFn: func(_ context.Context, _ string) (*models.Tenant, error) { return fakeTenant(), nil },
		getBrandingFn: func(_ context.Context, _ string) (*models.Branding, error) {
			return &models.Branding{
				LogoURL:     "https://example.com/logo.png",
				AccentColor: "#6366f1",
			}, nil
		},
	}
	h := handlers.NewBrandingHandler(store, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/branding", nil)
	req = withChiParam(req, "tenantSlug", "acme")
	rr := httptest.NewRecorder()

	h.GetBranding(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "logo.png") {
		t.Errorf("expected logo URL in response, got: %s", body)
	}
}

// TestUpdateBranding_AdminOnly verifies that PUT branding updates and returns the branding.
func TestUpdateBranding_AdminOnly(t *testing.T) {
	updated := false
	store := &mockBrandingStore{
		getTenantBySlugFn: func(_ context.Context, _ string) (*models.Tenant, error) { return fakeTenant(), nil },
		updateBrandingFn: func(_ context.Context, _ string, b models.Branding) error {
			updated = true
			if b.AccentColor != "#ff5733" {
				t.Errorf("expected #ff5733, got %s", b.AccentColor)
			}
			return nil
		},
	}
	h := handlers.NewBrandingHandler(store, zap.NewNop())

	branding := models.Branding{AccentColor: "#ff5733", LogoURL: "https://acme.com/logo.png"}
	body, _ := json.Marshal(branding)
	req := httptest.NewRequest(http.MethodPut, "/v1/tenants/acme/branding", bytes.NewReader(body))
	req = withChiParam(req, "tenantSlug", "acme")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.UpdateBranding(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !updated {
		t.Error("expected UpdateBranding to be called")
	}
}
