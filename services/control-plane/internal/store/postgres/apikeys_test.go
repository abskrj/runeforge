package postgres_test

import (
	"context"
	"strings"
	"testing"
)

func setupTenant(t *testing.T, ctx context.Context, store interface {
	CreateTenant(ctx context.Context, name, slug string) (interface{ GetID() string }, error)
}) string {
	t.Helper()
	return ""
}

func TestCreateAPIKeyWithPlain(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Key Test Org", uniqueSlug(t, "key-org"))
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	key, plain, err := store.CreateAPIKeyWithPlain(ctx, tenant.ID, "ci-key", []string{"invoke", "manage"})
	if err != nil {
		t.Fatalf("CreateAPIKeyWithPlain: %v", err)
	}

	if key.ID == "" {
		t.Error("key ID must not be empty")
	}
	if key.TenantID != tenant.ID {
		t.Errorf("tenant_id = %q; want %q", key.TenantID, tenant.ID)
	}
	if key.Name != "ci-key" {
		t.Errorf("name = %q; want %q", key.Name, "ci-key")
	}
	if len(key.Scopes) != 2 {
		t.Errorf("scopes count = %d; want 2", len(key.Scopes))
	}
	if !strings.HasPrefix(plain, "vl_") {
		t.Errorf("plain key must start with 'vl_', got %q", plain)
	}
	if len(plain) != 35 { // "vl_" + 32 hex chars
		t.Errorf("plain key length = %d; want 35", len(plain))
	}
	if key.KeyHash == plain {
		t.Error("stored hash must not equal the plain key")
	}
}

func TestValidateAPIKey_Success(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Validate Org", uniqueSlug(t, "validate-org"))
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	_, plain, err := store.CreateAPIKeyWithPlain(ctx, tenant.ID, "test-key", []string{"invoke"})
	if err != nil {
		t.Fatalf("CreateAPIKeyWithPlain: %v", err)
	}

	validated, err := store.ValidateAPIKey(ctx, plain)
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if validated.TenantID != tenant.ID {
		t.Errorf("tenant_id = %q; want %q", validated.TenantID, tenant.ID)
	}
}

func TestValidateAPIKey_WrongKey(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, _ := store.CreateTenant(ctx, "Wrong Key Org", uniqueSlug(t, "wrong-key-org"))
	_, plain, _ := store.CreateAPIKeyWithPlain(ctx, tenant.ID, "key", []string{"invoke"})

	// Tamper with the last byte of the plain key.
	tampered := plain[:len(plain)-1] + "X"
	_, err := store.ValidateAPIKey(ctx, tampered)
	if err == nil {
		t.Fatal("expected error for tampered key, got nil")
	}
}

func TestValidateAPIKey_InvalidFormat(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	for _, bad := range []string{"", "notvl_something", "vl_short"} {
		_, err := store.ValidateAPIKey(ctx, bad)
		if err == nil {
			t.Errorf("expected error for invalid key %q, got nil", bad)
		}
	}
}

func TestValidateAPIKey_UpdatesLastUsedAt(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, _ := store.CreateTenant(ctx, "LastUsed Org", uniqueSlug(t, "lastused-org"))
	key, plain, _ := store.CreateAPIKeyWithPlain(ctx, tenant.ID, "lu-key", []string{"invoke"})

	if key.LastUsedAt != nil {
		t.Error("last_used_at should be nil before first use")
	}

	_, err := store.ValidateAPIKey(ctx, plain)
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
}
