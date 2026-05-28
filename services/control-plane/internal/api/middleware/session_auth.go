package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/abskrj/velane/services/control-plane/internal/auth"
	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

type sessionContextKey string

const (
	sessionUserKey sessionContextKey = "session_user"
	sessionRoleKey sessionContextKey = "session_role"
)

// SessionStore is the subset of the store needed by SessionAuth to resolve tenant membership.
type SessionStore interface {
	GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error)
	GetMemberRole(ctx context.Context, tenantID, userID string) (string, error)
}

// SessionAuth validates a Bearer JWT token (from admin portal login) and sets the user in
// context. When an X-Tenant header is present it also resolves the user's tenant membership
// role and stores it in context so RequireScope can grant access without an API key.
// Falls through to next if the token is absent or invalid, allowing API key auth to handle it.
func SessionAuth(provider auth.Provider, store SessionStore, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			raw := strings.TrimPrefix(header, "Bearer ")
			user, err := provider.ValidateSession(r.Context(), raw)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), sessionUserKey, user)

			// If X-Tenant is present, look up the user's membership role so RequireScope works.
			if slug := r.Header.Get("X-Tenant"); slug != "" && store != nil {
				if tenant, err := store.GetTenantBySlug(ctx, slug); err == nil {
					if role, err := store.GetMemberRole(ctx, tenant.ID, user.ID); err == nil {
						ctx = context.WithValue(ctx, sessionRoleKey, role)
						ctx = context.WithValue(ctx, tenantKey, tenant)
					}
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionUserFromContext retrieves the authenticated session user from the request context.
func SessionUserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(sessionUserKey).(*models.User)
	return u
}

// SessionRoleFromContext retrieves the tenant-scoped role for the session user.
func SessionRoleFromContext(ctx context.Context) string {
	r, _ := ctx.Value(sessionRoleKey).(string)
	return r
}
