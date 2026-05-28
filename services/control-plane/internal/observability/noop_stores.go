package observability

import (
	"context"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// NoopLogStore is a no-op implementation of LogStore.
type NoopLogStore struct{}

func (NoopLogStore) StoreInvocationLogs(context.Context, *models.Invocation) error {
	return nil
}

// NoopMetricsStore is a no-op implementation of MetricsStore.
type NoopMetricsStore struct{}

func (NoopMetricsStore) StoreInvocationMetrics(context.Context, *models.Invocation) error {
	return nil
}

// NoopReplayStore is a no-op implementation of ReplayStore.
type NoopReplayStore struct{}

func (NoopReplayStore) StoreInvocationReplay(context.Context, *models.Invocation) error {
	return nil
}
