package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// PasswordProvider implements Provider using bcrypt passwords and Postgres-backed sessions.
type PasswordProvider struct {
	store PasswordStore
}

type PasswordStore interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*models.Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	GetUserByID(ctx context.Context, id string) (*models.User, error)
}

func NewPasswordProvider(store PasswordStore) *PasswordProvider {
	return &PasswordProvider{store: store}
}

func (p *PasswordProvider) CreateUser(ctx context.Context, email, password string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return p.store.CreateUser(ctx, email, string(hash))
}

func (p *PasswordProvider) Authenticate(ctx context.Context, email, password string) (*models.Session, error) {
	user, err := p.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	raw, hash := generateSessionToken()
	sess, err := p.store.CreateSession(ctx, user.ID, hash, time.Now().Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	sess.Token = raw
	return sess, nil
}

func (p *PasswordProvider) ValidateSession(ctx context.Context, rawToken string) (*models.User, error) {
	hash := hashToken(rawToken)
	sess, err := p.store.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}
	return p.store.GetUserByID(ctx, sess.UserID)
}

func (p *PasswordProvider) InvalidateSession(ctx context.Context, rawToken string) error {
	return p.store.DeleteSession(ctx, hashToken(rawToken))
}

func generateSessionToken() (raw, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	hash = hashToken(raw)
	return
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
