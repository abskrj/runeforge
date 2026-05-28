package auth_test

import (
	"strings"
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/auth"
)

func TestGenerateAPIKey_Format(t *testing.T) {
	plain, prefix, hash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(plain, "vl_") {
		t.Errorf("plain key must start with 'vl_', got %q", plain)
	}

	hexPart := strings.TrimPrefix(plain, "vl_")
	if len(hexPart) != 32 {
		t.Errorf("hex portion must be 32 chars, got %d", len(hexPart))
	}

	if prefix != hexPart[:8] {
		t.Errorf("prefix must equal first 8 hex chars: want %q, got %q", hexPart[:8], prefix)
	}

	if len(hash) == 0 {
		t.Error("hash must not be empty")
	}

	if hash == plain {
		t.Error("hash must not equal the plain key")
	}
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		plain, _, _, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[plain] {
			t.Fatalf("duplicate key generated at iteration %d: %q", i, plain)
		}
		seen[plain] = true
	}
}

func TestGenerateAPIKey_PrefixLength(t *testing.T) {
	for i := 0; i < 10; i++ {
		_, prefix, _, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(prefix) != 8 {
			t.Errorf("prefix must be exactly 8 chars, got %d (%q)", len(prefix), prefix)
		}
	}
}

func TestGenerateAPIKey_HashIsNotPlainKey(t *testing.T) {
	plain, _, hash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == plain {
		t.Error("hash must differ from plain key (bcrypt must have been applied)")
	}
	// A bcrypt hash always starts with "$2a$" or "$2b$".
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("hash does not look like a bcrypt hash: %q", hash[:min(10, len(hash))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
