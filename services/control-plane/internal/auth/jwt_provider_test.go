package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/runeforge/control-plane/internal/auth"
	"github.com/runeforge/control-plane/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// mockJWTStore satisfies both auth.JWTStore and auth.PasswordStore.
type mockJWTStore struct {
	users         map[string]*models.User
	refreshTokens map[string]*models.RefreshToken
}

func newMockJWTStore() *mockJWTStore {
	return &mockJWTStore{
		users:         make(map[string]*models.User),
		refreshTokens: make(map[string]*models.RefreshToken),
	}
}

func (m *mockJWTStore) addUser(id, email, password string) *models.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	u := &models.User{ID: id, Email: email, PasswordHash: string(hash), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.users[email] = u
	m.users[id] = u
	return u
}

func (m *mockJWTStore) CreateUser(_ context.Context, email, passwordHash string) (*models.User, error) {
	u := &models.User{ID: "new-user", Email: email, PasswordHash: passwordHash, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.users[email] = u
	m.users[u.ID] = u
	return u, nil
}

func (m *mockJWTStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return u, nil
}

func (m *mockJWTStore) GetUserByID(_ context.Context, id string) (*models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return u, nil
}

func (m *mockJWTStore) CreateSession(_ context.Context, userID, tokenHash string, expiresAt time.Time) (*models.Session, error) {
	return &models.Session{ID: "sess-1", UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt}, nil
}

func (m *mockJWTStore) GetSessionByTokenHash(_ context.Context, tokenHash string) (*models.Session, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockJWTStore) DeleteSession(_ context.Context, tokenHash string) error {
	return nil
}

func (m *mockJWTStore) CreateRefreshToken(_ context.Context, userID, tokenHash string, expiresAt time.Time) (*models.RefreshToken, error) {
	rt := &models.RefreshToken{
		ID:        "rt-1",
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	m.refreshTokens[tokenHash] = rt
	return rt, nil
}

func (m *mockJWTStore) GetRefreshTokenByHash(_ context.Context, tokenHash string) (*models.RefreshToken, error) {
	rt, ok := m.refreshTokens[tokenHash]
	if !ok {
		return nil, fmt.Errorf("refresh token not found")
	}
	if rt.RevokedAt != nil {
		return nil, fmt.Errorf("refresh token has been revoked")
	}
	if rt.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("refresh token has expired")
	}
	return rt, nil
}

func (m *mockJWTStore) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	rt, ok := m.refreshTokens[tokenHash]
	if !ok {
		return nil
	}
	now := time.Now()
	rt.RevokedAt = &now
	return nil
}

func newTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key, &key.PublicKey
}

func hashForTest(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func TestJWTProvider_AuthenticateIssuesTokens(t *testing.T) {
	store := newMockJWTStore()
	store.addUser("user-1", "alice@example.com", "password123")

	privKey, _ := newTestKeyPair(t)
	p := auth.NewJWTProvider(store, privKey, "https://test.runeforge.io")

	sess, err := p.Authenticate(context.Background(), "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.Token == "" {
		t.Error("expected access token in session.Token")
	}
	if sess.ExpiresAt.IsZero() {
		t.Error("expected non-zero expires_at")
	}
}

func TestJWTProvider_ValidateAccessToken(t *testing.T) {
	store := newMockJWTStore()
	store.addUser("user-1", "alice@example.com", "password123")

	privKey, _ := newTestKeyPair(t)
	p := auth.NewJWTProvider(store, privKey, "https://test.runeforge.io")

	sess, err := p.Authenticate(context.Background(), "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	user, err := p.ValidateSession(context.Background(), sess.Token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", user.Email)
	}
}

func TestJWTProvider_ExpiredAccessToken(t *testing.T) {
	store := newMockJWTStore()
	store.addUser("user-1", "alice@example.com", "password123")

	privKey, _ := newTestKeyPair(t)
	p := auth.NewJWTProvider(store, privKey, "https://test.runeforge.io")

	// Manually create an already-expired token.
	type expiredClaims struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}
	claims := expiredClaims{
		Email: "alice@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "https://test.runeforge.io",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, _ := token.SignedString(privKey)

	_, err := p.ValidateSession(context.Background(), signed)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestJWTProvider_InvalidSignature(t *testing.T) {
	store := newMockJWTStore()
	store.addUser("user-1", "alice@example.com", "password123")

	privKey, _ := newTestKeyPair(t)
	wrongKey, _ := newTestKeyPair(t)
	p := auth.NewJWTProvider(store, privKey, "https://test.runeforge.io")

	// Sign with the wrong key; validate with the correct public key.
	type testClaims struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}
	claims := testClaims{
		Email: "alice@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "https://test.runeforge.io",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, _ := token.SignedString(wrongKey)

	_, err := p.ValidateSession(context.Background(), signed)
	if err == nil {
		t.Fatal("expected error for wrong signature, got nil")
	}
}

func TestJWTProvider_RefreshRotates(t *testing.T) {
	store := newMockJWTStore()
	u := store.addUser("user-1", "alice@example.com", "password123")

	privKey, _ := newTestKeyPair(t)
	p := auth.NewJWTProvider(store, privKey, "https://test.runeforge.io")

	// Inject a known raw refresh token directly.
	rawRefresh := "knownrawrefreshtoken0000000000000000000000000000000000000000000"
	hash := hashForTest(rawRefresh)
	store.refreshTokens[hash] = &models.RefreshToken{
		ID:        "rt-original",
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	pair, err := p.Refresh(context.Background(), rawRefresh)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("expected access token in pair")
	}
	if pair.RefreshToken == "" {
		t.Error("expected new refresh token in pair")
	}
	if pair.RefreshToken == rawRefresh {
		t.Error("new refresh token should be different from the old one")
	}

	// Old refresh token should now be revoked.
	_, err = p.Refresh(context.Background(), rawRefresh)
	if err == nil {
		t.Fatal("expected error for revoked refresh token, got nil")
	}
}

func TestJWTProvider_RevokedRefreshToken(t *testing.T) {
	store := newMockJWTStore()
	u := store.addUser("user-1", "carol@example.com", "password789")

	privKey, _ := newTestKeyPair(t)
	p := auth.NewJWTProvider(store, privKey, "https://test.runeforge.io")

	rawToken := "revokedrefreshtoken00000000000000000000000000000000000000000000"
	hash := hashForTest(rawToken)
	now := time.Now()
	store.refreshTokens[hash] = &models.RefreshToken{
		ID:        "rt-revoked",
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		RevokedAt: &now,
		CreatedAt: time.Now(),
	}

	_, err := p.Refresh(context.Background(), rawToken)
	if err == nil {
		t.Fatal("expected error for revoked refresh token, got nil")
	}
}
