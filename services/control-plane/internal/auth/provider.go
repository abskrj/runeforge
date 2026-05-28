package auth

import (
	"context"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// Provider is the auth abstraction. Phase 8 ships PasswordProvider (bcrypt + Postgres sessions).
// Phase 9 will add OIDCProvider and SAMLProvider implementing this same interface.
type Provider interface {
	CreateUser(ctx context.Context, email, password string) (*models.User, error)
	Authenticate(ctx context.Context, email, password string) (*models.Session, error)
	ValidateSession(ctx context.Context, rawToken string) (*models.User, error)
	InvalidateSession(ctx context.Context, rawToken string) error
}
