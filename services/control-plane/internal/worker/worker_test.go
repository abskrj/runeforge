package worker_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/runeforge/control-plane/internal/executor"
	"github.com/runeforge/control-plane/internal/models"
	redisstore "github.com/runeforge/control-plane/internal/store/redis"
	"github.com/runeforge/control-plane/internal/worker"
	"go.uber.org/zap"
)

// --- Mocks ---

// mockDequeuer implements worker.Dequeuer by returning jobs from a buffered channel.
type mockDequeuer struct {
	jobs chan *redisstore.Job
}

func newMockDequeuer(jobs ...*redisstore.Job) *mockDequeuer {
	ch := make(chan *redisstore.Job, len(jobs)+1)
	for _, j := range jobs {
		ch <- j
	}
	return &mockDequeuer{jobs: ch}
}

func (m *mockDequeuer) Dequeue(ctx context.Context) (*redisstore.Job, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	case j, ok := <-m.jobs:
		if !ok {
			<-ctx.Done()
			return nil, nil
		}
		return j, nil
	}
}

// mockWorkerStore captures UpdateInvocationResult calls.
type mockWorkerStore struct {
	mu      sync.Mutex
	results []capturedResult
}

type capturedResult struct {
	id     string
	status models.InvocationStatus
	output string
	errMsg string
}

func (m *mockWorkerStore) UpdateInvocationResult(
	ctx context.Context, id string, status models.InvocationStatus,
	output, errMsg, stderr string, durationMs, peakMemoryMB int,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, capturedResult{
		id:     id,
		status: status,
		output: output,
		errMsg: errMsg,
	})
	return nil
}

func (m *mockWorkerStore) latest() *capturedResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.results) == 0 {
		return nil
	}
	r := m.results[len(m.results)-1]
	return &r
}

func (m *mockWorkerStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.results)
}

// mockExecutor implements executor.Executor with configurable behaviour.
type mockExecutor struct {
	run       func(ctx context.Context, spec executor.RunSpec) executor.RunResult
	runStream func(ctx context.Context, spec executor.RunSpec) (<-chan executor.StreamChunk, error)
}

func (m *mockExecutor) Run(ctx context.Context, spec executor.RunSpec) executor.RunResult {
	if m.run != nil {
		return m.run(ctx, spec)
	}
	return executor.RunResult{}
}

func (m *mockExecutor) RunStream(ctx context.Context, spec executor.RunSpec) (<-chan executor.StreamChunk, error) {
	if m.runStream != nil {
		return m.runStream(ctx, spec)
	}
	ch := make(chan executor.StreamChunk)
	close(ch)
	return ch, nil
}

// --- Helpers ---

func makeJob(invocationID string) *redisstore.Job {
	return &redisstore.Job{
		InvocationID: invocationID,
		SnippetID:    "snip-1",
		VersionID:    "ver-1",
		TenantID:     "tenant-1",
		Language:     "bun",
		Code:         "export default async function handler() { return 42 }",
		Input:        `{}`,
		TimeoutMs:    5000,
		MaxMemoryMB:  128,
	}
}

// runWorkerUntilDone runs the worker, waits until expectedJobs results are
// persisted or 5 seconds elapse, then cancels the worker context.
func runWorkerUntilDone(t *testing.T, deq *mockDequeuer, store *mockWorkerStore, exec executor.Executor, expectedJobs int) {
	t.Helper()

	log := zap.NewNop()
	w := worker.New(deq, store, exec, log, 1)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if store.count() >= expectedJobs {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("worker did not stop within 2s after cancel")
	}
}

// --- Tests ---

func TestWorker_ProcessesJob(t *testing.T) {
	job := makeJob("inv-1")

	exec := &mockExecutor{
		run: func(ctx context.Context, spec executor.RunSpec) executor.RunResult {
			return executor.RunResult{Output: `{"ok":true}`, ExitCode: 0}
		},
	}
	store := &mockWorkerStore{}
	deq := newMockDequeuer(job)

	runWorkerUntilDone(t, deq, store, exec, 1)

	result := store.latest()
	if result == nil {
		t.Fatal("no result stored")
	}
	if result.id != job.InvocationID {
		t.Errorf("id = %q; want %q", result.id, job.InvocationID)
	}
	if result.status != models.InvocationCompleted {
		t.Errorf("status = %q; want %q", result.status, models.InvocationCompleted)
	}
	if result.output != `{"ok":true}` {
		t.Errorf("output = %q; want %q", result.output, `{"ok":true}`)
	}
}

func TestWorker_TimeoutMapsToTimeoutStatus(t *testing.T) {
	job := makeJob("inv-timeout")

	exec := &mockExecutor{
		run: func(ctx context.Context, spec executor.RunSpec) executor.RunResult {
			return executor.RunResult{Error: "timeout", ExitCode: -1}
		},
	}
	store := &mockWorkerStore{}
	deq := newMockDequeuer(job)

	runWorkerUntilDone(t, deq, store, exec, 1)

	result := store.latest()
	if result == nil {
		t.Fatal("no result stored")
	}
	if result.status != models.InvocationTimeout {
		t.Errorf("status = %q; want %q", result.status, models.InvocationTimeout)
	}
}

func TestWorker_OOMKilledStatus(t *testing.T) {
	job := makeJob("inv-oom")

	exec := &mockExecutor{
		run: func(ctx context.Context, spec executor.RunSpec) executor.RunResult {
			return executor.RunResult{Error: "oom", ExitCode: 137}
		},
	}
	store := &mockWorkerStore{}
	deq := newMockDequeuer(job)

	runWorkerUntilDone(t, deq, store, exec, 1)

	result := store.latest()
	if result == nil {
		t.Fatal("no result stored")
	}
	if result.status != models.InvocationOOMKilled {
		t.Errorf("status = %q; want %q", result.status, models.InvocationOOMKilled)
	}
}

func TestWorker_WebhookFiredOnCallbackURL(t *testing.T) {
	// Webhook delivery is tested in depth in webhook_test.go using httptest.
	// Here we verify the job still completes even when the callback URL is
	// unreachable (best-effort; failure is logged but not fatal).
	job := makeJob("inv-webhook")
	job.CallbackURL = "http://127.0.0.1:1/unreachable" // port 1 is not routable

	exec := &mockExecutor{
		run: func(ctx context.Context, spec executor.RunSpec) executor.RunResult {
			return executor.RunResult{Output: `{}`, ExitCode: 0}
		},
	}
	store := &mockWorkerStore{}
	deq := newMockDequeuer(job)

	runWorkerUntilDone(t, deq, store, exec, 1)

	result := store.latest()
	if result == nil {
		t.Fatal("no result stored")
	}
	if result.status != models.InvocationCompleted {
		t.Errorf("status = %q; want %q", result.status, models.InvocationCompleted)
	}
}

func TestWorker_ProcessesMultipleJobs(t *testing.T) {
	n := 5
	jobs := make([]*redisstore.Job, n)
	for i := range jobs {
		jobs[i] = makeJob(fmt.Sprintf("inv-%d", i))
	}

	exec := &mockExecutor{
		run: func(ctx context.Context, spec executor.RunSpec) executor.RunResult {
			return executor.RunResult{Output: `{}`, ExitCode: 0}
		},
	}
	store := &mockWorkerStore{}
	deq := newMockDequeuer(jobs...)

	runWorkerUntilDone(t, deq, store, exec, n)

	if got := store.count(); got != n {
		t.Errorf("processed %d jobs; want %d", got, n)
	}
}
