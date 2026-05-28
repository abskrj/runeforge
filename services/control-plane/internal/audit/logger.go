package audit

import (
	"context"

	"github.com/runeforge/control-plane/internal/models"
	"go.uber.org/zap"
)

// Store is the subset of *postgres.Store that the audit logger needs.
type Store interface {
	AppendAuditLog(ctx context.Context, entry models.AuditEntry) error
}

// Logger wraps a Store and provides fire-and-forget audit log writing.
type Logger struct {
	store Store
	log   *zap.Logger
}

// New constructs an audit.Logger.
func New(store Store, log *zap.Logger) *Logger {
	return &Logger{store: store, log: log}
}

// Log appends an audit entry. Never blocks the caller — runs the DB insert in a
// goroutine and logs a warning on error, but never returns it to the caller.
func (a *Logger) Log(ctx context.Context, entry models.AuditEntry) {
	go func() {
		if err := a.store.AppendAuditLog(context.Background(), entry); err != nil {
			a.log.Warn("audit log append failed",
				zap.String("tenant_id", entry.TenantID),
				zap.String("action", entry.Action),
				zap.Error(err),
			)
		}
	}()
}
