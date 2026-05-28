package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

// mockAuthStore satisfies middleware.AuthStore without a real database.
type mockAuthStore struct {
	validateAPIKey func(ctx context.Context, plain string) (*models.APIKey, error)
	getTenantByID  func(ctx context.Context, id string) (*models.Tenant, error)
}

func (m *mockAuthStore) ValidateAPIKey(ctx context.Context, plain string) (*models.APIKey, error) {
	return m.validateAPIKey(ctx, plain)
}
func (m *mockAuthStore) GetTenantByID(ctx context.Context, id string) (*models.Tenant, error) {
	return m.getTenantByID(ctx, id)
}

var nopLog = zap.NewNop()

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestAuth_MissingHeader(t *testing.T) {
	store := &mockAuthStore{}
	mw := middleware.Auth(store, nopLog)
	h := mw(http.HandlerFunc(okHandler))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestAuth_MalformedHeader(t *testing.T) {
	store := &mockAuthStore{}
	mw := middleware.Auth(store, nopLog)
	h := mw(http.HandlerFunc(okHandler))

	for _, hdr := range []string{"nobearer", "Basic abc", "Bearer"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", hdr)
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: status = %d; want 401", hdr, rec.Code)
		}
	}
}

func TestAuth_InvalidKey(t *testing.T) {
	store := &mockAuthStore{
		validateAPIKey: func(ctx context.Context, plain string) (*models.APIKey, error) {
			return nil, errors.New("invalid key")
		},
	}
	mw := middleware.Auth(store, nopLog)
	h := mw(http.HandlerFunc(okHandler))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer vl_badkey")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestAuth_ValidKey_AttachesTenantAndKey(t *testing.T) {
	key := &models.APIKey{ID: "key-1", TenantID: "tenant-1", Scopes: []string{"invoke"}}
	tenant := &models.Tenant{ID: "tenant-1", Name: "Acme", Slug: "acme"}

	store := &mockAuthStore{
		validateAPIKey: func(ctx context.Context, plain string) (*models.APIKey, error) {
			return key, nil
		},
		getTenantByID: func(ctx context.Context, id string) (*models.Tenant, error) {
			return tenant, nil
		},
	}
	mw := middleware.Auth(store, nopLog)

	var gotTenant *models.Tenant
	var gotKey *models.APIKey
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = middleware.TenantFromContext(r.Context())
		gotKey = middleware.APIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer vl_validkey12345")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if gotTenant == nil || gotTenant.ID != "tenant-1" {
		t.Errorf("tenant not attached correctly: %+v", gotTenant)
	}
	if gotKey == nil || gotKey.ID != "key-1" {
		t.Errorf("api key not attached correctly: %+v", gotKey)
	}
}

func TestAuth_TenantLookupFails(t *testing.T) {
	key := &models.APIKey{ID: "key-1", TenantID: "tenant-1", Scopes: []string{"invoke"}}

	store := &mockAuthStore{
		validateAPIKey: func(ctx context.Context, plain string) (*models.APIKey, error) {
			return key, nil
		},
		getTenantByID: func(ctx context.Context, id string) (*models.Tenant, error) {
			return nil, errors.New("tenant not found")
		},
	}
	mw := middleware.Auth(store, nopLog)
	h := mw(http.HandlerFunc(okHandler))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer vl_validkey12345")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestRequireScope_AllowsMatchingScope(t *testing.T) {
	key := &models.APIKey{ID: "key-1", TenantID: "tenant-1", Scopes: []string{"manage"}}
	tenant := &models.Tenant{ID: "tenant-1"}

	store := &mockAuthStore{
		validateAPIKey: func(ctx context.Context, plain string) (*models.APIKey, error) { return key, nil },
		getTenantByID:  func(ctx context.Context, id string) (*models.Tenant, error) { return tenant, nil },
	}

	h := middleware.Auth(store, nopLog)(
		middleware.RequireScope("manage", nopLog)(
			http.HandlerFunc(okHandler),
		),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer vl_valid")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
}

func TestRequireScope_BlocksMissingScope(t *testing.T) {
	key := &models.APIKey{ID: "key-1", TenantID: "tenant-1", Scopes: []string{"invoke"}}
	tenant := &models.Tenant{ID: "tenant-1"}

	store := &mockAuthStore{
		validateAPIKey: func(ctx context.Context, plain string) (*models.APIKey, error) { return key, nil },
		getTenantByID:  func(ctx context.Context, id string) (*models.Tenant, error) { return tenant, nil },
	}

	h := middleware.Auth(store, nopLog)(
		middleware.RequireScope("manage", nopLog)(
			http.HandlerFunc(okHandler),
		),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer vl_valid")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rec.Code)
	}
}

func TestRequireScope_AdminPassesAnyScope(t *testing.T) {
	key := &models.APIKey{ID: "key-1", TenantID: "tenant-1", Scopes: []string{"admin"}}
	tenant := &models.Tenant{ID: "tenant-1"}

	store := &mockAuthStore{
		validateAPIKey: func(ctx context.Context, plain string) (*models.APIKey, error) { return key, nil },
		getTenantByID:  func(ctx context.Context, id string) (*models.Tenant, error) { return tenant, nil },
	}

	for _, scope := range []string{"invoke", "manage", "admin"} {
		h := middleware.Auth(store, nopLog)(
			middleware.RequireScope(scope, nopLog)(
				http.HandlerFunc(okHandler),
			),
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", "Bearer vl_admin")
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("scope %q: status = %d; want 200 (admin should pass all)", scope, rec.Code)
		}
	}
}
