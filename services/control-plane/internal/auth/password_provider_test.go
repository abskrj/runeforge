package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/auth"
	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// mockPasswordStore is a simple in-memory implementation of PasswordStore for testing.
type mockPasswordStore struct {
	users    map[string]*models.User    // keyed by email
	usersID  map[string]*models.User    // keyed by id
	sessions map[string]*models.Session // keyed by tokenHash
	deleted  []string
}

func newMockPasswordStore() *mockPasswordStore {
	return &mockPasswordStore{
		users:    make(map[string]*models.User),
		usersID:  make(map[string]*models.User),
		sessions: make(map[string]*models.Session),
	}
}

func (m *mockPasswordStore) CreateUser(_ context.Context, email, passwordHash string) (*models.User, error) {
	u := &models.User{
		ID:           "user-1",
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[email] = u
	m.usersID[u.ID] = u
	return u, nil
}

func (m *mockPasswordStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (m *mockPasswordStore) GetUserByID(_ context.Context, id string) (*models.User, error) {
	u, ok := m.usersID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (m *mockPasswordStore) CreateSession(_ context.Context, userID, tokenHash string, expiresAt time.Time) (*models.Session, error) {
	sess := &models.Session{
		ID:        "sess-1",
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	m.sessions[tokenHash] = sess
	return sess, nil
}

func (m *mockPasswordStore) GetSessionByTokenHash(_ context.Context, tokenHash string) (*models.Session, error) {
	sess, ok := m.sessions[tokenHash]
	if !ok {
		return nil, errors.New("session not found")
	}
	return sess, nil
}

func (m *mockPasswordStore) DeleteSession(_ context.Context, tokenHash string) error {
	m.deleted = append(m.deleted, tokenHash)
	delete(m.sessions, tokenHash)
	return nil
}

// TestPasswordProvider_CreateAndAuthenticate verifies that a user can be created
// and then authenticated with the correct password.
func TestPasswordProvider_CreateAndAuthenticate(t *testing.T) {
	store := newMockPasswordStore()
	provider := auth.NewPasswordProvider(store)
	ctx := context.Background()

	user, err := provider.CreateUser(ctx, "alice@example.com", "secret123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", user.Email)
	}

	sess, err := provider.Authenticate(ctx, "alice@example.com", "secret123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if sess.Token == "" {
		t.Error("expected non-empty session token")
	}
	if sess.ExpiresAt.IsZero() {
		t.Error("expected non-zero expires_at")
	}
}

// TestPasswordProvider_WrongPassword verifies that an incorrect password returns an error.
func TestPasswordProvider_WrongPassword(t *testing.T) {
	store := newMockPasswordStore()
	provider := auth.NewPasswordProvider(store)
	ctx := context.Background()

	_, _ = provider.CreateUser(ctx, "bob@example.com", "correctpass")

	_, err := provider.Authenticate(ctx, "bob@example.com", "wrongpass")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

// TestPasswordProvider_ValidateSession verifies that a valid session token returns the user.
func TestPasswordProvider_ValidateSession(t *testing.T) {
	store := newMockPasswordStore()
	provider := auth.NewPasswordProvider(store)
	ctx := context.Background()

	_, _ = provider.CreateUser(ctx, "carol@example.com", "pass")
	sess, err := provider.Authenticate(ctx, "carol@example.com", "pass")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	user, err := provider.ValidateSession(ctx, sess.Token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if user.Email != "carol@example.com" {
		t.Errorf("expected carol@example.com, got %q", user.Email)
	}
}

// TestPasswordProvider_ExpiredSession verifies that an expired session returns an error.
func TestPasswordProvider_ExpiredSession(t *testing.T) {
	store := newMockPasswordStore()
	provider := auth.NewPasswordProvider(store)
	ctx := context.Background()

	_, _ = provider.CreateUser(ctx, "dave@example.com", "pass")
	sess, err := provider.Authenticate(ctx, "dave@example.com", "pass")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	// Back-date the stored session to simulate expiry.
	stored := store.sessions[sess.TokenHash]
	stored.ExpiresAt = time.Now().Add(-1 * time.Hour)

	_, err = provider.ValidateSession(ctx, sess.Token)
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
}

// TestPasswordProvider_InvalidateSession verifies that InvalidateSession calls DeleteSession.
func TestPasswordProvider_InvalidateSession(t *testing.T) {
	store := newMockPasswordStore()
	provider := auth.NewPasswordProvider(store)
	ctx := context.Background()

	_, _ = provider.CreateUser(ctx, "eve@example.com", "pass")
	sess, _ := provider.Authenticate(ctx, "eve@example.com", "pass")

	err := provider.InvalidateSession(ctx, sess.Token)
	if err != nil {
		t.Fatalf("InvalidateSession: %v", err)
	}
	if len(store.deleted) == 0 {
		t.Error("expected DeleteSession to be called")
	}
}
