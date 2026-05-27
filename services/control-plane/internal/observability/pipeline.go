package observability

import (
	"context"

	"github.com/runeforge/control-plane/internal/models"
)

// PipelineObserver fans-out completed invocations to configured stores.
type PipelineObserver struct {
	Logs    LogStore
	Metrics MetricsStore
	Replay  ReplayStore
}

// NewPipelineObserver creates an observer with no-op defaults.
func NewPipelineObserver(logs LogStore, metrics MetricsStore, replay ReplayStore) *PipelineObserver {
	if logs == nil {
		logs = NoopLogStore{}
	}
	if metrics == nil {
		metrics = NoopMetricsStore{}
	}
	if replay == nil {
		replay = NoopReplayStore{}
	}
	return &PipelineObserver{
		Logs:    logs,
		Metrics: metrics,
		Replay:  replay,
	}
}

// OnInvocationCompleted stores logs, metrics, and replay payloads best-effort.
func (p *PipelineObserver) OnInvocationCompleted(ctx context.Context, invocation *models.Invocation) error {
	if invocation == nil {
		return nil
	}
	_ = p.Logs.StoreInvocationLogs(ctx, invocation)
	_ = p.Metrics.StoreInvocationMetrics(ctx, invocation)
	_ = p.Replay.StoreInvocationReplay(ctx, invocation)
	return nil
}
