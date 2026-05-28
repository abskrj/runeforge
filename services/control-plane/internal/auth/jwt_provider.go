package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/runeforge/control-plane/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// runeforgeClaims are the JWT claims embedded in access tokens.
type runeforgeClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// JWTStore is the store interface required by JWTProvider.
type JWTStore interface {
	PasswordStore // reuse: CreateUser, GetUserByEmail, GetUserByID
	CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*models.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

// AuthTokenPair is the response returned for login and token refresh.
type AuthTokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`  // access token expiry
	TokenType    string    `json:"token_type"`  // "Bearer"
}

// JWTProvider implements Provider using RS256 JWTs (access tokens) + Postgres refresh tokens.
// It embeds PasswordProvider for CreateUser and Authenticate (password check).
type JWTProvider struct {
	passwords *PasswordProvider
	store     JWTStore
	privKey   *rsa.PrivateKey
	pubKey    *rsa.PublicKey
	issuer    string
}

// NewJWTProvider creates a JWTProvider backed by the given store and RSA key pair.
func NewJWTProvider(store JWTStore, privKey *rsa.PrivateKey, issuer string) *JWTProvider {
	return &JWTProvider{
		passwords: NewPasswordProvider(store),
		store:     store,
		privKey:   privKey,
		pubKey:    &privKey.PublicKey,
		issuer:    issuer,
	}
}

// CreateUser delegates to PasswordProvider.
func (p *JWTProvider) CreateUser(ctx context.Context, email, password string) (*models.User, error) {
	return p.passwords.CreateUser(ctx, email, password)
}

// Authenticate verifies the password then issues a JWT access token + refresh token.
// Returns a *models.Session with Token set to the ACCESS token (for API compatibility).
func (p *JWTProvider) Authenticate(ctx context.Context, email, password string) (*models.Session, error) {
	user, err := p.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	pair, err := p.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	return &models.Session{
		UserID:    user.ID,
		Token:     pair.AccessToken,
		ExpiresAt: pair.ExpiresAt,
	}, nil
}

// ValidateSession validates the RS256 JWT access token (stateless — no DB lookup).
func (p *JWTProvider) ValidateSession(ctx context.Context, rawToken string) (*models.User, error) {
	claims := &runeforgeClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return p.pubKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return nil, fmt.Errorf("token missing subject")
	}

	return p.store.GetUserByID(ctx, sub)
}

// InvalidateSession revokes the refresh token associated with the raw token.
// Access tokens are short-lived and cannot be individually revoked — this is an
// intentional trade-off of stateless JWTs.
// For logout compatibility, we accept the access token here (no-op since it's stateless).
func (p *JWTProvider) InvalidateSession(ctx context.Context, rawToken string) error {
	// Hash the token as a best-effort refresh token lookup.
	// If rawToken is an access JWT, RevokeRefreshToken will simply find no rows.
	hash := hashRefreshToken(rawToken)
	// Ignore errors — the token may already be expired or not be a refresh token.
	_ = p.store.RevokeRefreshToken(ctx, hash)
	return nil
}

// Refresh exchanges a valid refresh token for a new access+refresh token pair.
// The old refresh token is revoked (rotation).
func (p *JWTProvider) Refresh(ctx context.Context, rawRefreshToken string) (*AuthTokenPair, error) {
	hash := hashRefreshToken(rawRefreshToken)

	rt, err := p.store.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	user, err := p.store.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Revoke the old refresh token before issuing a new pair (rotation).
	if err := p.store.RevokeRefreshToken(ctx, hash); err != nil {
		return nil, fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	return p.issueTokenPair(ctx, user)
}

// IssueAccessToken creates a signed RS256 JWT with claims: sub=userID, email, iss, iat, exp.
func (p *JWTProvider) IssueAccessToken(user *models.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(15 * time.Minute)
	claims := runeforgeClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    p.issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(p.privKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// issueTokenPair creates a new access + refresh token pair and persists the refresh token.
func (p *JWTProvider) issueTokenPair(ctx context.Context, user *models.User) (*AuthTokenPair, error) {
	accessToken, expiresAt, err := p.IssueAccessToken(user)
	if err != nil {
		return nil, err
	}

	rawRefresh, refreshHash := generateRefreshToken()
	refreshExpiry := time.Now().Add(7 * 24 * time.Hour)

	rt, err := p.store.CreateRefreshToken(ctx, user.ID, refreshHash, refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}
	rt.Token = rawRefresh

	return &AuthTokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, nil
}

// generateRefreshToken creates a cryptographically random refresh token and its SHA-256 hash.
func generateRefreshToken() (raw, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	hash = hashRefreshToken(raw)
	return
}

// hashRefreshToken returns the hex-encoded SHA-256 hash of a raw token value.
func hashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
