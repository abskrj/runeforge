package postgres_test

import (
	"context"
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
)

// testEncKey is a fixed 32-byte key for integration tests.
var testEncKey = make([]byte, 32) // all zeros — fine for tests

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	plain := "my-super-secret-value"
	enc, err := postgres.EncryptForTest(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	dec, err := postgres.DecryptForTest(key, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != plain {
		t.Errorf("round-trip: got %q; want %q", dec, plain)
	}
}

func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	plain := "same-plaintext"

	enc1, err := postgres.EncryptForTest(key, plain)
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}
	enc2, err := postgres.EncryptForTest(key, plain)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}

	if enc1 == enc2 {
		t.Error("same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = 0xFF
	}

	enc, err := postgres.EncryptForTest(key1, "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = postgres.DecryptForTest(key2, enc)
	if err == nil {
		t.Error("expected error decrypting with wrong key, got nil")
	}
}

func TestCreateSecret(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Secret Tenant", uniqueSlug(t, "sec-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	sec, err := store.CreateSecret(ctx, tenant.ID, nil, "API_KEY", "super-secret", []string{"prod"}, testEncKey)
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	if sec.ID == "" {
		t.Error("secret ID should be set")
	}
	if sec.Name != "API_KEY" {
		t.Errorf("name = %q; want %q", sec.Name, "API_KEY")
	}
	// The encrypted value must NOT be in the returned model's ValueEncrypted field
	// (the scan helper doesn't select it, so it's empty).
	if sec.ValueEncrypted != "" {
		t.Error("ValueEncrypted should be empty in returned model (never returned from DB scan)")
	}
}

func TestListSecrets(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "List Secret Tenant", uniqueSlug(t, "list-sec-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// Create two secrets.
	_, err = store.CreateSecret(ctx, tenant.ID, nil, "SECRET_A", "value-a", []string{}, testEncKey)
	if err != nil {
		t.Fatalf("create secret A: %v", err)
	}
	_, err = store.CreateSecret(ctx, tenant.ID, nil, "SECRET_B", "value-b", []string{"prod"}, testEncKey)
	if err != nil {
		t.Fatalf("create secret B: %v", err)
	}

	secrets, err := store.ListSecrets(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}

	if len(secrets) < 2 {
		t.Fatalf("expected at least 2 secrets, got %d", len(secrets))
	}

	// Values must never be returned.
	for _, s := range secrets {
		if s.ValueEncrypted != "" {
			t.Errorf("secret %q: ValueEncrypted must be empty in list response", s.Name)
		}
	}
}

func TestDeleteSecret(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Delete Secret Tenant", uniqueSlug(t, "del-sec-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	sec, err := store.CreateSecret(ctx, tenant.ID, nil, "TO_DELETE", "value", []string{}, testEncKey)
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	if err := store.DeleteSecret(ctx, sec.ID, tenant.ID); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}

	// Verify it's gone.
	secrets, err := store.ListSecrets(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListSecrets after delete: %v", err)
	}
	for _, s := range secrets {
		if s.ID == sec.ID {
			t.Error("deleted secret still appears in list")
		}
	}
}

func TestGetSecretsForInvocation_SnippetSpecific(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Inv Secret Tenant", uniqueSlug(t, "inv-sec-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	snippet, err := store.CreateSnippet(ctx, tenant.ID, "My Snippet", uniqueSlug(t, "inv-sec-snip"), "bun", "")
	if err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	// Tenant-wide secret.
	_, err = store.CreateSecret(ctx, tenant.ID, nil, "DB_PASS", "tenant-level-pass", []string{}, testEncKey)
	if err != nil {
		t.Fatalf("create tenant secret: %v", err)
	}

	// Snippet-specific secret with the same name — should override.
	_, err = store.CreateSecret(ctx, tenant.ID, &snippet.ID, "DB_PASS", "snippet-level-pass", []string{}, testEncKey)
	if err != nil {
		t.Fatalf("create snippet secret: %v", err)
	}

	secrets, err := store.GetSecretsForInvocation(ctx, tenant.ID, snippet.ID, "prod", testEncKey)
	if err != nil {
		t.Fatalf("GetSecretsForInvocation: %v", err)
	}

	val, ok := secrets["DB_PASS"]
	if !ok {
		t.Fatal("DB_PASS not in secrets map")
	}
	if val != "snippet-level-pass" {
		t.Errorf("DB_PASS = %q; want %q (snippet override)", val, "snippet-level-pass")
	}
}

func TestGetSecretsForInvocation_TenantWide(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "TW Secret Tenant", uniqueSlug(t, "tw-sec-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	snippet, err := store.CreateSnippet(ctx, tenant.ID, "TW Snippet", uniqueSlug(t, "tw-sec-snip"), "bun", "")
	if err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	// Only tenant-wide secret.
	_, err = store.CreateSecret(ctx, tenant.ID, nil, "GLOBAL_TOKEN", "global-value", []string{}, testEncKey)
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	secrets, err := store.GetSecretsForInvocation(ctx, tenant.ID, snippet.ID, "prod", testEncKey)
	if err != nil {
		t.Fatalf("GetSecretsForInvocation: %v", err)
	}

	val, ok := secrets["GLOBAL_TOKEN"]
	if !ok {
		t.Fatal("GLOBAL_TOKEN not in secrets map")
	}
	if val != "global-value" {
		t.Errorf("GLOBAL_TOKEN = %q; want %q", val, "global-value")
	}
}

func TestGetSecretsForInvocation_EnvironmentFilter(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Env Filter Tenant", uniqueSlug(t, "envf-sec-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	snippet, err := store.CreateSnippet(ctx, tenant.ID, "EnvF Snippet", uniqueSlug(t, "envf-sec-snip"), "bun", "")
	if err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	// Prod-only secret.
	_, err = store.CreateSecret(ctx, tenant.ID, nil, "PROD_ONLY", "prod-value", []string{"prod"}, testEncKey)
	if err != nil {
		t.Fatalf("create prod secret: %v", err)
	}

	// Fetch for dev environment — should NOT return the prod-only secret.
	secrets, err := store.GetSecretsForInvocation(ctx, tenant.ID, snippet.ID, "dev", testEncKey)
	if err != nil {
		t.Fatalf("GetSecretsForInvocation: %v", err)
	}

	if _, ok := secrets["PROD_ONLY"]; ok {
		t.Error("PROD_ONLY should not be returned when invoking in dev environment")
	}

	// Fetch for prod — should return it.
	secretsProd, err := store.GetSecretsForInvocation(ctx, tenant.ID, snippet.ID, "prod", testEncKey)
	if err != nil {
		t.Fatalf("GetSecretsForInvocation (prod): %v", err)
	}

	if _, ok := secretsProd["PROD_ONLY"]; !ok {
		t.Error("PROD_ONLY should be returned when invoking in prod environment")
	}
}
