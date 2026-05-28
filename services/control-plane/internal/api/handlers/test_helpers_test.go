package handlers_test

import (
	"context"
	"net/http"

	"github.com/abskrj/velane/services/control-plane/internal/api/middleware"
	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// setSessionUser injects a user into the context the same way SessionAuth middleware does.
// This is achieved by running a real SessionAuth middleware with a mock provider.
func setSessionUser(ctx context.Context, user *models.User) context.Context {
	// We create a one-shot provider that returns the user for any token.
	provider := &mockAuthProvider{
		validateFn: func(_ context.Context, _ string) (*models.User, error) {
			return user, nil
		},
	}
	// Run the middleware against a fake request to capture the enriched context.
	var enriched context.Context
	mw := middleware.SessionAuth(provider, (middleware.SessionStore)(nil), nil)
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		enriched = r.Context()
	}))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer faketoken")
	handler.ServeHTTP(nil, req)
	return enriched
}
