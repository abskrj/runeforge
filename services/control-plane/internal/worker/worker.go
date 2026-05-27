// Package worker implements the background job processor that pulls async
// invocations off the Redis queue and executes them.
package worker

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/runeforge/control-plane/internal/executor"
	"github.com/runeforge/control-plane/internal/models"
	redisstore "github.com/runeforge/control-plane/internal/store/redis"
)

// Dequeuer is the interface the worker uses to pull jobs from the queue.
type Dequeuer interface {
	Dequeue(ctx context.Context) (*redisstore.Job, error)
}

// WorkerStore is the DB operations the worker needs.
type WorkerStore interface {
	UpdateInvocationResult(ctx context.Context, id string, status models.InvocationStatus, output, errMsg, stderr string, durationMs, peakMemoryMB int) error
}

// Worker pulls jobs from the Redis queue and executes them concurrently.
type Worker struct {
	queue   Dequeuer
	store   WorkerStore
	exec    executor.Executor
	webhook *WebhookClient
	log     *zap.Logger
	workers int // concurrency level
}

// New creates a Worker. workers controls how many goroutines process jobs in
// parallel.
func New(queue Dequeuer, store WorkerStore, exec executor.Executor, log *zap.Logger, workers int) *Worker {
	if workers <= 0 {
		workers = 1
	}
	return &Worker{
		queue:   queue,
		store:   store,
		exec:    exec,
		webhook: newWebhookClient(log),
		log:     log,
		workers: workers,
	}
}

// Run starts w.workers goroutines, each blocking on Dequeue, and processes
// jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.loop(ctx, workerID)
		}(i)
	}
	wg.Wait()
}

// loop is the per-goroutine job processing loop.
func (w *Worker) loop(ctx context.Context, workerID int) {
	for {
		if ctx.Err() != nil {
			return
		}

		job, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.Error("worker: dequeue error", zap.Int("worker", workerID), zap.Error(err))
			continue
		}

		// Dequeue returns (nil, nil) on context cancellation.
		if job == nil {
			return
		}

		w.process(ctx, workerID, job)
	}
}

// process executes a single job and persists the result.
func (w *Worker) process(ctx context.Context, workerID int, job *redisstore.Job) {
	w.log.Info("worker: executing job",
		zap.Int("worker", workerID),
		zap.String("invocation_id", job.InvocationID),
		zap.String("language", job.Language),
	)

	result := w.exec.Run(ctx, executor.RunSpec{
		Language:    job.Language,
		Code:        job.Code,
		Input:       job.Input,
		TimeoutMs:   job.TimeoutMs,
		MaxMemoryMB: job.MaxMemoryMB,
	})

	status := mapResultStatus(result)

	if err := w.store.UpdateInvocationResult(ctx,
		job.InvocationID,
		status,
		result.Output,
		result.Error,
		result.Stderr,
		result.DurationMs,
		result.PeakMemoryMB,
	); err != nil {
		w.log.Error("worker: persist result",
			zap.String("invocation_id", job.InvocationID),
			zap.Error(err),
		)
	}

	if job.CallbackURL != "" {
		w.webhook.Deliver(ctx, job.CallbackURL, WebhookPayload{
			InvocationID: job.InvocationID,
			Status:       string(status),
			Output:       result.Output,
			Error:        result.Error,
			DurationMs:   result.DurationMs,
		})
	}

	w.log.Info("worker: job done",
		zap.Int("worker", workerID),
		zap.String("invocation_id", job.InvocationID),
		zap.String("status", string(status)),
	)
}

// mapResultStatus converts an executor RunResult into an InvocationStatus.
func mapResultStatus(result executor.RunResult) models.InvocationStatus {
	if result.Error == "timeout" {
		return models.InvocationTimeout
	}
	if result.Error == "oom" {
		return models.InvocationOOMKilled
	}
	if result.ExitCode != 0 || result.Error != "" {
		return models.InvocationFailed
	}
	return models.InvocationCompleted
}
