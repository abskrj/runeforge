package postgres_test

import (
	"context"
	"testing"
)

func TestSetCanary(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Canary Tenant", uniqueSlug(t, "canary-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	snippet, err := store.CreateSnippet(ctx, tenant.ID, "Canary Snippet", uniqueSlug(t, "canary-snip"), "bun", "")
	if err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	// Create two versions.
	v1, err := store.CreateVersion(ctx, snippet.ID, "// v1", "{}", "{}", "", 5000, 128, 100)
	if err != nil {
		t.Fatalf("create version 1: %v", err)
	}

	v2, err := store.CreateVersion(ctx, snippet.ID, "// v2", "{}", "{}", "", 5000, 128, 100)
	if err != nil {
		t.Fatalf("create version 2: %v", err)
	}

	// Publish v1 to prod (creates the env row).
	_, err = store.PublishVersion(ctx, v1.ID, "prod")
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	// Set v2 as canary with 30%.
	se, err := store.SetCanary(ctx, snippet.ID, "prod", v2.ID, 30)
	if err != nil {
		t.Fatalf("SetCanary: %v", err)
	}

	if se.CanaryVersionID == nil {
		t.Fatal("CanaryVersionID should be set")
	}
	if *se.CanaryVersionID != v2.ID {
		t.Errorf("CanaryVersionID = %q; want %q", *se.CanaryVersionID, v2.ID)
	}
	if se.CanaryPct != 30 {
		t.Errorf("CanaryPct = %d; want 30", se.CanaryPct)
	}

	// Verify via GetSnippetEnvironment.
	env, err := store.GetSnippetEnvironment(ctx, snippet.ID, "prod")
	if err != nil {
		t.Fatalf("GetSnippetEnvironment: %v", err)
	}
	if env.CanaryVersionID == nil || *env.CanaryVersionID != v2.ID {
		t.Error("GetSnippetEnvironment did not return canary version ID")
	}
	if env.CanaryPct != 30 {
		t.Errorf("GetSnippetEnvironment CanaryPct = %d; want 30", env.CanaryPct)
	}
}

func TestClearCanary(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, err := store.CreateTenant(ctx, "Clear Canary Tenant", uniqueSlug(t, "clear-canary-tenant"))
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	snippet, err := store.CreateSnippet(ctx, tenant.ID, "Clear Canary Snippet", uniqueSlug(t, "clear-canary-snip"), "bun", "")
	if err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	v1, err := store.CreateVersion(ctx, snippet.ID, "// v1", "{}", "{}", "", 5000, 128, 100)
	if err != nil {
		t.Fatalf("create version 1: %v", err)
	}

	v2, err := store.CreateVersion(ctx, snippet.ID, "// v2", "{}", "{}", "", 5000, 128, 100)
	if err != nil {
		t.Fatalf("create version 2: %v", err)
	}

	_, err = store.PublishVersion(ctx, v1.ID, "prod")
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	_, err = store.SetCanary(ctx, snippet.ID, "prod", v2.ID, 50)
	if err != nil {
		t.Fatalf("SetCanary: %v", err)
	}

	// Now clear it.
	if err := store.ClearCanary(ctx, snippet.ID, "prod"); err != nil {
		t.Fatalf("ClearCanary: %v", err)
	}

	// Verify fields are reset.
	env, err := store.GetSnippetEnvironment(ctx, snippet.ID, "prod")
	if err != nil {
		t.Fatalf("GetSnippetEnvironment: %v", err)
	}
	if env.CanaryVersionID != nil {
		t.Errorf("CanaryVersionID should be nil after ClearCanary, got %v", *env.CanaryVersionID)
	}
	if env.CanaryPct != 0 {
		t.Errorf("CanaryPct should be 0 after ClearCanary, got %d", env.CanaryPct)
	}
}
