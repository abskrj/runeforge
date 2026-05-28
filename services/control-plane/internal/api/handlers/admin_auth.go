package handlers

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/auth"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

// AdminAuthStore is the subset of *postgres.Store that admin auth handlers need.
type AdminAuthStore interface {
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetInviteByTokenHash(ctx context.Context, hash string) (*models.InviteToken, error)
	AcceptInvite(ctx context.Context, id string) error
	AddMember(ctx context.Context, tenantID, userID, role string) (*models.TenantMember, error)
	GetUserPrimaryTenantSlug(ctx context.Context, userID string) (string, error)
}

// AdminAuthHandler handles email/password auth for the admin portal.
type AdminAuthHandler struct {
	provider auth.Provider
	store    AdminAuthStore
	log      *zap.Logger
	pubKey   *rsa.PublicKey // optional; used for JWKS endpoint
}

// NewAdminAuthHandler constructs an AdminAuthHandler.
func NewAdminAuthHandler(provider auth.Provider, store AdminAuthStore, log *zap.Logger) *AdminAuthHandler {
	return &AdminAuthHandler{provider: provider, store: store, log: log}
}

// WithPublicKey sets the RSA public key used to serve the JWKS endpoint.
// Call this when using JWTProvider.
func (h *AdminAuthHandler) WithPublicKey(pub *rsa.PublicKey) *AdminAuthHandler {
	h.pubKey = pub
	return h
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	InviteToken string `json:"invite_token,omitempty"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register handles POST /v1/admin/auth/register.
func (h *AdminAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.provider.CreateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		h.log.Error("create user failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// If an invite token was supplied, validate it and accept.
	if req.InviteToken != "" {
		inviteHash := hashInviteToken(req.InviteToken)
		invite, err := h.store.GetInviteByTokenHash(r.Context(), inviteHash)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid or expired invite token")
			return
		}
		if invite.AcceptedAt != nil {
			writeError(w, http.StatusBadRequest, "invite token already used")
			return
		}
		if invite.ExpiresAt.Before(time.Now()) {
			writeError(w, http.StatusBadRequest, "invite token expired")
			return
		}
		if err := h.store.AcceptInvite(r.Context(), invite.ID); err != nil {
			h.log.Error("accept invite failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to accept invite")
			return
		}
		if _, err := h.store.AddMember(r.Context(), invite.TenantID, user.ID, invite.Role); err != nil {
			h.log.Error("add member failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to add member")
			return
		}
	}

	// Authenticate immediately to return a session.
	sess, err := h.provider.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		h.log.Error("post-register authenticate failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	tenantSlug, _ := h.store.GetUserPrimaryTenantSlug(r.Context(), sess.UserID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":          user,
		"session_token": sess.Token,
		"expires_at":    sess.ExpiresAt,
		"tenant_slug":   tenantSlug,
	})
}

// Login handles POST /v1/admin/auth/login.
func (h *AdminAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	sess, err := h.provider.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	tenantSlug, _ := h.store.GetUserPrimaryTenantSlug(r.Context(), sess.UserID)

	writeJSON(w, http.StatusOK, map[string]any{
		"session_token": sess.Token,
		"expires_at":    sess.ExpiresAt,
		"tenant_slug":   tenantSlug,
	})
}

// Logout handles POST /v1/admin/auth/logout.
func (h *AdminAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeError(w, http.StatusBadRequest, "missing Authorization header")
		return
	}
	raw := strings.TrimPrefix(header, "Bearer ")

	if err := h.provider.InvalidateSession(r.Context(), raw); err != nil {
		h.log.Debug("invalidate session failed", zap.Error(err))
	}

	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /v1/admin/auth/me.
func (h *AdminAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.SessionUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// JWKS handles GET /.well-known/jwks.json.
// Returns the RS256 public key in JWK Set format so third parties can verify Velane JWTs.
func (h *AdminAuthHandler) JWKS(w http.ResponseWriter, r *http.Request) {
	if h.pubKey == nil {
		writeError(w, http.StatusNotFound, "JWKS not available — JWT auth not configured")
		return
	}

	// Encode the RSA public key modulus (N) and exponent (E) in base64url without padding.
	nBytes := h.pubKey.N.Bytes()
	nB64 := base64.RawURLEncoding.EncodeToString(nBytes)

	eBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(eBuf, uint32(h.pubKey.E))
	// Trim leading zero bytes.
	i := 0
	for i < len(eBuf)-1 && eBuf[i] == 0 {
		i++
	}
	eB64 := base64.RawURLEncoding.EncodeToString(eBuf[i:])

	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": "velane-1",
				"n":   nB64,
				"e":   eB64,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jwks)
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshToken handles POST /v1/admin/auth/refresh.
// Body: { "refresh_token": "..." }
// Returns a new AuthTokenPair. The old refresh token is revoked (rotation).
func (h *AdminAuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	jwtProvider, ok := h.provider.(*auth.JWTProvider)
	if !ok {
		writeError(w, http.StatusNotImplemented, "token refresh not supported by current auth provider")
		return
	}

	pair, err := jwtProvider.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		h.log.Debug("refresh token failed", zap.Error(err))
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	writeJSON(w, http.StatusOK, pair)
}

// hashInviteToken hashes a raw invite token using SHA-256 (same algorithm as auth package).
func hashInviteToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Ensure big.Int is imported via the RSA key operations (used indirectly via N.Bytes()).
var _ = new(big.Int)
