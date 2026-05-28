package postgres_test

import (
	"context"
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

func setupSnippetForVersions(t *testing.T, store interface {
	CreateTenant(ctx context.Context, name, slug string) (*models.Tenant, error)
	CreateSnippet(ctx context.Context, tenantID, name, slug, language, createdBy string) (*models.Snippet, error)
}) (tenantID, snippetID string) {
	t.Helper()
	ctx := context.Background()
	tenant, err := store.CreateTenant(ctx, "Version Org", uniqueSlug(t, "ver-org"))
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	sn, err := store.CreateSnippet(ctx, tenant.ID, "Version Snippet", uniqueSlug(t, "ver-sn"), "bun", "user-1")
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	return tenant.ID, sn.ID
}

func TestCreateVersion_AutoIncrementsVersionNumber(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, snippetID := setupSnippetForVersions(t, store)

	v1, err := store.CreateVersion(ctx, snippetID, "code1", "{}", "{}", "user-1", 5000, 128, 100)
	if err != nil {
		t.Fatalf("CreateVersion v1: %v", err)
	}
	v2, err := store.CreateVersion(ctx, snippetID, "code2", "{}", "{}", "user-1", 5000, 128, 100)
	if err != nil {
		t.Fatalf("CreateVersion v2: %v", err)
	}

	if v1.VersionNumber != 1 {
		t.Errorf("v1 version_number = %d; want 1", v1.VersionNumber)
	}
	if v2.VersionNumber != 2 {
		t.Errorf("v2 version_number = %d; want 2", v2.VersionNumber)
	}
}

func TestCreateVersion_DefaultsStatusToDraft(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, snippetID := setupSnippetForVersions(t, store)

	v, err := store.CreateVersion(ctx, snippetID, "code", "{}", "{}", "user-1", 5000, 128, 100)
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	if v.Status != models.StatusDraft {
		t.Errorf("status = %q; want %q", v.Status, models.StatusDraft)
	}
}

func TestGetVersionByNumber(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, snippetID := setupSnippetForVersions(t, store)

	created, err := store.CreateVersion(ctx, snippetID, "my code", "{}", "{}", "user-1", 5000, 128, 100)
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	fetched, err := store.GetVersionByNumber(ctx, snippetID, created.VersionNumber)
	if err != nil {
		t.Fatalf("GetVersionByNumber: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %q, want %q", fetched.ID, created.ID)
	}
	if fetched.Code != "my code" {
		t.Errorf("code = %q; want %q", fetched.Code, "my code")
	}
}

func TestListVersions(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, snippetID := setupSnippetForVersions(t, store)

	for i := 0; i < 4; i++ {
		_, err := store.CreateVersion(ctx, snippetID, "code", "{}", "{}", "user-1", 5000, 128, 100)
		if err != nil {
			t.Fatalf("CreateVersion %d: %v", i, err)
		}
	}

	versions, err := store.ListVersions(ctx, snippetID)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 4 {
		t.Errorf("got %d versions; want 4", len(versions))
	}
	// Must be ordered by version_number ascending.
	for i, v := range versions {
		if v.VersionNumber != i+1 {
			t.Errorf("versions[%d].version_number = %d; want %d", i, v.VersionNumber, i+1)
		}
	}
}

func TestPublishVersion_SetsStatusAndEnvironmentPointer(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, _ := store.CreateTenant(ctx, "Publish Org", uniqueSlug(t, "pub-org"))
	sn, _ := store.CreateSnippet(ctx, tenant.ID, "Pub Snippet", uniqueSlug(t, "pub-sn"), "bun", "user-1")

	v, err := store.CreateVersion(ctx, sn.ID, "published code", "{}", "{}", "user-1", 5000, 128, 100)
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	published, err := store.PublishVersion(ctx, v.ID, "prod")
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}

	if published.Status != models.StatusPublished {
		t.Errorf("status = %q; want %q", published.Status, models.StatusPublished)
	}

	// Verify the environment pointer is updated.
	env, err := store.GetSnippetEnvironment(ctx, sn.ID, "prod")
	if err != nil {
		t.Fatalf("GetSnippetEnvironment: %v", err)
	}
	if env.ActiveVersionID == nil || *env.ActiveVersionID != v.ID {
		t.Errorf("active_version_id = %v; want %q", env.ActiveVersionID, v.ID)
	}
}

func TestPublishVersion_ArchivesOldPublishedVersion(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	tenant, _ := store.CreateTenant(ctx, "Archive Org", uniqueSlug(t, "arch-org"))
	sn, _ := store.CreateSnippet(ctx, tenant.ID, "Archive Snippet", uniqueSlug(t, "arch-sn"), "bun", "user-1")

	v1, _ := store.CreateVersion(ctx, sn.ID, "v1 code", "{}", "{}", "user-1", 5000, 128, 100)
	_, err := store.PublishVersion(ctx, v1.ID, "prod")
	if err != nil {
		t.Fatalf("PublishVersion v1: %v", err)
	}

	v2, _ := store.CreateVersion(ctx, sn.ID, "v2 code", "{}", "{}", "user-1", 5000, 128, 100)
	_, err = store.PublishVersion(ctx, v2.ID, "prod")
	if err != nil {
		t.Fatalf("PublishVersion v2: %v", err)
	}

	// v1 should now be archived.
	fetched, err := store.GetVersion(ctx, v1.ID)
	if err != nil {
		t.Fatalf("GetVersion v1: %v", err)
	}
	if fetched.Status != models.StatusArchived {
		t.Errorf("v1 status = %q; want %q", fetched.Status, models.StatusArchived)
	}

	// v2 should be published.
	fetched2, err := store.GetVersion(ctx, v2.ID)
	if err != nil {
		t.Fatalf("GetVersion v2: %v", err)
	}
	if fetched2.Status != models.StatusPublished {
		t.Errorf("v2 status = %q; want %q", fetched2.Status, models.StatusPublished)
	}
}
