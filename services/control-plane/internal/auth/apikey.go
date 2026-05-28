package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/abskrj/velane/services/control-plane/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// GenerateAPIKey returns a new plain key, its 8-char prefix, and its bcrypt hash.
// Plain key format: "vl_" + 32 random hex chars.
// The prefix is the first 8 chars of the hex portion (after "vl_").
func GenerateAPIKey() (plain, prefix, hash string, err error) {
	buf := make([]byte, 16)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("rand.Read: %w", err)
	}
	hexPart := hex.EncodeToString(buf) // 32 hex chars
	plain = "vl_" + hexPart
	prefix = hexPart[:8]

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", fmt.Errorf("bcrypt: %w", err)
	}
	hash = string(hashBytes)
	return plain, prefix, hash, nil
}

// KeyStore is the subset of the postgres.Store that auth needs.
type KeyStore interface {
	ValidateAPIKey(ctx context.Context, plain string) (*models.APIKey, error)
}

// ValidateKey is a convenience wrapper that delegates to the store.
func ValidateKey(ctx context.Context, store KeyStore, plain string) (*models.APIKey, error) {
	return store.ValidateAPIKey(ctx, plain)
}
