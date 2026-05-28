package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/handlers"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

// --- mock members store ---

type mockMembersStore struct {
	getTenantBySlugFn  func(ctx context.Context, slug string) (*models.Tenant, error)
	listMembersFn      func(ctx context.Context, tenantID string) ([]*models.TenantMember, error)
	removeMemberFn     func(ctx context.Context, tenantID, userID string) error
	createInviteFn     func(ctx context.Context, tenantID, email, role, tokenHash string, expiresAt time.Time) (*models.InviteToken, error)
	listPendingInvites func(ctx context.Context, tenantID string) ([]*models.InviteToken, error)
}

func (m *mockMembersStore) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	return m.getTenantBySlugFn(ctx, slug)
}
func (m *mockMembersStore) ListMembers(ctx context.Context, tenantID string) ([]*models.TenantMember, error) {
	return m.listMembersFn(ctx, tenantID)
}
func (m *mockMembersStore) RemoveMember(ctx context.Context, tenantID, userID string) error {
	return m.removeMemberFn(ctx, tenantID, userID)
}
func (m *mockMembersStore) CreateInvite(ctx context.Context, tenantID, email, role, tokenHash string, expiresAt time.Time) (*models.InviteToken, error) {
	return m.createInviteFn(ctx, tenantID, email, role, tokenHash, expiresAt)
}
func (m *mockMembersStore) ListPendingInvites(ctx context.Context, tenantID string) ([]*models.InviteToken, error) {
	return m.listPendingInvites(ctx, tenantID)
}

func fakeTenant() *models.Tenant {
	return &models.Tenant{ID: "t1", Name: "Acme", Slug: "acme"}
}

func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestInvite_CreatesToken verifies that inviting a member creates and returns an invite token.
func TestInvite_CreatesToken(t *testing.T) {
	store := &mockMembersStore{
		getTenantBySlugFn: func(_ context.Context, _ string) (*models.Tenant, error) { return fakeTenant(), nil },
		createInviteFn: func(_ context.Context, _, _, _, _ string, expiresAt time.Time) (*models.InviteToken, error) {
			return &models.InviteToken{
				ID:        "inv1",
				TenantID:  "t1",
				Email:     "newuser@example.com",
				Role:      "manage",
				ExpiresAt: expiresAt,
				CreatedAt: time.Now(),
			}, nil
		},
	}
	h := handlers.NewMembersHandler(store, zap.NewNop())

	body, _ := json.Marshal(map[string]string{"email": "newuser@example.com", "role": "manage"})
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants/acme/members/invite", bytes.NewReader(body))
	req = withChiParam(req, "tenantSlug", "acme")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.InviteMember(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["invite_token"] == nil {
		t.Error("expected invite_token in response")
	}
	token, ok := resp["invite_token"].(string)
	if !ok || token == "" {
		t.Error("invite_token should be a non-empty string")
	}
}

// TestListMembers verifies that list members returns a JSON array.
func TestListMembers(t *testing.T) {
	store := &mockMembersStore{
		getTenantBySlugFn: func(_ context.Context, _ string) (*models.Tenant, error) { return fakeTenant(), nil },
		listMembersFn: func(_ context.Context, _ string) ([]*models.TenantMember, error) {
			return []*models.TenantMember{
				{TenantID: "t1", UserID: "u1", Email: "alice@example.com", Role: "admin", InvitedAt: time.Now()},
			}, nil
		},
	}
	h := handlers.NewMembersHandler(store, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/members", nil)
	req = withChiParam(req, "tenantSlug", "acme")
	rr := httptest.NewRecorder()

	h.ListMembers(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "alice@example.com") {
		t.Errorf("expected alice@example.com in response, got: %s", body)
	}
}

// TestRemoveMember verifies that remove member returns 204.
func TestRemoveMember(t *testing.T) {
	removed := false
	store := &mockMembersStore{
		getTenantBySlugFn: func(_ context.Context, _ string) (*models.Tenant, error) { return fakeTenant(), nil },
		removeMemberFn: func(_ context.Context, _, _ string) error {
			removed = true
			return nil
		},
	}
	h := handlers.NewMembersHandler(store, zap.NewNop())

	req := httptest.NewRequest(http.MethodDelete, "/v1/tenants/acme/members/u1", nil)
	req = withChiParam(req, "tenantSlug", "acme")
	req = withChiParam(req, "userID", "u1")
	rr := httptest.NewRecorder()

	h.RemoveMember(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
	if !removed {
		t.Error("expected RemoveMember to be called")
	}
}
