package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/abskrj/velane/services/control-plane/internal/models"
	"go.uber.org/zap"
)

type EmbedAuthStore interface {
	ValidateEmbedToken(ctx context.Context, plain string) (*models.EmbedToken, error)
	GetTenantByID(ctx context.Context, id string) (*models.Tenant, error)
}

const embedCtxKey contextKey = "embed_ctx"
const invalidEmbedTokenMsg = "invalid embed token"

// EmbedContextFromContext returns embed context for embed-authenticated routes.
func EmbedContextFromContext(ctx context.Context) *models.EmbedContext {
	v, _ := ctx.Value(embedCtxKey).(*models.EmbedContext)
	return v
}

// AuthEmbed validates a bearer embed token and attaches tenant + embed context.
func AuthEmbed(store EmbedAuthStore, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plain, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}
			if !strings.HasPrefix(plain, "et_") {
				writeUnauthorized(w, invalidEmbedTokenMsg)
				return
			}

			token, err := store.ValidateEmbedToken(r.Context(), plain)
			if err != nil {
				log.Debug("embed token validation failed", zap.Error(err))
				writeUnauthorized(w, invalidEmbedTokenMsg)
				return
			}
			tenant, err := store.GetTenantByID(r.Context(), token.TenantID)
			if err != nil {
				log.Error("embed tenant lookup failed", zap.String("tenant_id", token.TenantID), zap.Error(err))
				writeUnauthorized(w, invalidEmbedTokenMsg)
				return
			}

			ctx := context.WithValue(r.Context(), tenantKey, tenant)
			ctx = context.WithValue(ctx, embedCtxKey, &models.EmbedContext{
				TokenID:           token.ID,
				TenantID:          token.TenantID,
				AllowedSnippetIDs: token.AllowedSnippetIDs,
				ExpiresAt:         token.ExpiresAt,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
