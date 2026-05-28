package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/audit"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

// MembersStore is the subset of *postgres.Store that member handlers need.
type MembersStore interface {
	GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error)
	ListMembers(ctx context.Context, tenantID string) ([]*models.TenantMember, error)
	RemoveMember(ctx context.Context, tenantID, userID string) error
	CreateInvite(ctx context.Context, tenantID, email, role, tokenHash string, expiresAt time.Time) (*models.InviteToken, error)
	ListPendingInvites(ctx context.Context, tenantID string) ([]*models.InviteToken, error)
}

// MembersHandler handles team member and invite endpoints.
type MembersHandler struct {
	store   MembersStore
	log     *zap.Logger
	auditor *audit.Logger
}

// NewMembersHandler constructs a MembersHandler.
func NewMembersHandler(store MembersStore, log *zap.Logger) *MembersHandler {
	return &MembersHandler{store: store, log: log}
}

// WithAuditor attaches an audit logger to the MembersHandler.
func (h *MembersHandler) WithAuditor(a *audit.Logger) *MembersHandler {
	h.auditor = a
	return h
}

// ListMembers handles GET /v1/tenants/{slug}/members.
func (h *MembersHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
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

	members, err := h.store.ListMembers(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list members failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	writeJSON(w, http.StatusOK, members)
}

type inviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// InviteMember handles POST /v1/tenants/{slug}/members/invite.
func (h *MembersHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
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

	var req inviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if req.Role == "" {
		req.Role = "manage"
	}
	switch req.Role {
	case "invoke", "manage", "admin":
	default:
		writeError(w, http.StatusBadRequest, "role must be one of: invoke, manage, admin")
		return
	}

	rawToken, tokenHash := generateInviteToken()
	expiresAt := time.Now().Add(72 * time.Hour)

	invite, err := h.store.CreateInvite(r.Context(), tenant.ID, req.Email, req.Role, tokenHash, expiresAt)
	if err != nil {
		h.log.Error("create invite failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "member_invite",
			ResourceID: invite.ID,
			Metadata:   auditMeta(map[string]any{"email": req.Email, "role": req.Role}),
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"invite_token": rawToken,
		"expires_at":   invite.ExpiresAt,
	})
}

// RemoveMember handles DELETE /v1/tenants/{slug}/members/{userID}.
func (h *MembersHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	userID := chi.URLParam(r, "userID")

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

	if err := h.store.RemoveMember(r.Context(), tenant.ID, userID); err != nil {
		h.log.Error("remove member failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	if h.auditor != nil {
		actorID, actorType := resolveActor(r)
		h.auditor.Log(r.Context(), models.AuditEntry{
			TenantID:   tenant.ID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     "member_remove",
			ResourceID: userID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListInvites handles GET /v1/tenants/{slug}/members/invites.
func (h *MembersHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
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

	invites, err := h.store.ListPendingInvites(r.Context(), tenant.ID)
	if err != nil {
		h.log.Error("list invites failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}

	writeJSON(w, http.StatusOK, invites)
}

func generateInviteToken() (raw, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return
}
