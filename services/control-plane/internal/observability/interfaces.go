package observability

import (
	"context"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// LogStore persists invocation logs in an external store.
type LogStore interface {
	StoreInvocationLogs(ctx context.Context, invocation *models.Invocation) error
}

// MetricsStore persists invocation metrics in an external store.
type MetricsStore interface {
	StoreInvocationMetrics(ctx context.Context, invocation *models.Invocation) error
}

// ReplayStore persists invocation replay payloads in an external store.
type ReplayStore interface {
	StoreInvocationReplay(ctx context.Context, invocation *models.Invocation) error
}

// InvocationObserver receives completed invocation records.
type InvocationObserver interface {
	OnInvocationCompleted(ctx context.Context, invocation *models.Invocation) error
}

// NoopObserver is the default observer that does nothing.
type NoopObserver struct{}

func (NoopObserver) OnInvocationCompleted(context.Context, *models.Invocation) error {
	return nil
}
