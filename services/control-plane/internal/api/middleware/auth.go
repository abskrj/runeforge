package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/runeforge/control-plane/internal/models"
	"go.uber.org/zap"
)

// AuthStore is the subset of *postgres.Store that the auth middleware needs.
type AuthStore interface {
	ValidateAPIKey(ctx context.Context, plain string) (*models.APIKey, error)
	GetTenantByID(ctx context.Context, id string) (*models.Tenant, error)
}

// contextKey is a package-private type to avoid key collisions in context values.
type contextKey string

const (
	tenantKey contextKey = "tenant"
	apikeyKey contextKey = "apikey"
)

// TenantFromContext retrieves the authenticated tenant from the request context.
func TenantFromContext(ctx context.Context) *models.Tenant {
	v, _ := ctx.Value(tenantKey).(*models.Tenant)
	return v
}

// APIKeyFromContext retrieves the authenticated API key from the request context.
func APIKeyFromContext(ctx context.Context) *models.APIKey {
	v, _ := ctx.Value(apikeyKey).(*models.APIKey)
	return v
}

// ExportedTenantKey returns the context key used to store the authenticated tenant.
// Intended for use in tests that need to inject a tenant into the context.
func ExportedTenantKey() any { return tenantKey }

// ExportedAPIKeyKey returns the context key used to store the authenticated API key.
// Intended for use in tests that need to inject an API key into the context.
func ExportedAPIKeyKey() any { return apikeyKey }

// Auth returns a middleware that validates the Bearer token in the
// Authorization header and attaches the resolved Tenant and APIKey to the
// request context. Requests without a valid key receive 401.
func Auth(store AuthStore, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plain, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}

			key, err := store.ValidateAPIKey(r.Context(), plain)
			if err != nil {
				log.Debug("api key validation failed", zap.Error(err))
				writeUnauthorized(w, "invalid api key")
				return
			}

			tenant, err := store.GetTenantByID(r.Context(), key.TenantID)
			if err != nil {
				log.Error("tenant lookup failed", zap.String("tenant_id", key.TenantID), zap.Error(err))
				writeUnauthorized(w, "invalid api key")
				return
			}

			ctx := context.WithValue(r.Context(), tenantKey, tenant)
			ctx = context.WithValue(ctx, apikeyKey, key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope returns a middleware that enforces that the authenticated key
// carries the specified scope. Must be applied after Auth.
func RequireScope(scope string, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				writeUnauthorized(w, "unauthenticated")
				return
			}
			if !key.HasScope(scope) {
				http.Error(w, `{"error":"forbidden: missing scope `+scope+`"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(r *http.Request) (string, bool) {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return "", false
	}
	parts := strings.SplitN(hdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
