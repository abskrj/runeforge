package audit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/runeforge/control-plane/internal/audit"
	"github.com/runeforge/control-plane/internal/models"
	"go.uber.org/zap"
)

type mockAuditStore struct {
	mu      sync.Mutex
	entries []models.AuditEntry
	err     error
}

func (m *mockAuditStore) AppendAuditLog(_ context.Context, entry models.AuditEntry) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	m.entries = append(m.entries, entry)
	m.mu.Unlock()
	return nil
}

func (m *mockAuditStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func TestAuditLogger_LogCallsStore(t *testing.T) {
	store := &mockAuditStore{}
	log, _ := zap.NewDevelopment()
	a := audit.New(store, log)

	entry := models.AuditEntry{
		TenantID:   "tenant-1",
		ActorID:    "user-1",
		ActorType:  "user",
		Action:     "publish",
		ResourceID: "snippet-1",
		Metadata:   json.RawMessage(`{"version_num":3,"env":"prod"}`),
	}

	a.Log(context.Background(), entry)

	// Log is fire-and-forget; give the goroutine time to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.count() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if store.count() != 1 {
		t.Errorf("expected 1 audit entry, got %d", store.count())
	}
}

func TestAuditLogger_StoreErrorDoesNotPanic(t *testing.T) {
	store := &mockAuditStore{err: fmt.Errorf("db unavailable")}
	log, _ := zap.NewDevelopment()
	a := audit.New(store, log)

	// This must not panic or block.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log panicked: %v", r)
		}
	}()

	a.Log(context.Background(), models.AuditEntry{
		TenantID:  "tenant-1",
		ActorType: "user",
		Action:    "publish",
	})

	// Give the goroutine time to attempt the insert (and fail silently).
	time.Sleep(50 * time.Millisecond)

	if store.count() != 0 {
		t.Errorf("expected 0 entries on store error, got %d", store.count())
	}
}
