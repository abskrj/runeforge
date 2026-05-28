package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/store/postgres"
)

// testStore connects to a real Postgres instance using TEST_DATABASE_URL.
// The test is skipped if the env var is not set.
func testStore(t *testing.T) *postgres.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test postgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// uniqueSlug returns a slug that is unique per test run to avoid conflicts.
func uniqueSlug(t *testing.T, base string) string {
	t.Helper()
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}
