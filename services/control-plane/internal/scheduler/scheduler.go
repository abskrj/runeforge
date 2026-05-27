package scheduler

import (
	"context"
	"fmt"

	"github.com/runeforge/control-plane/internal/executor"
	"github.com/runeforge/control-plane/internal/models"
	redisstore "github.com/runeforge/control-plane/internal/store/redis"
)

// Store is the subset of *postgres.Store that the Scheduler depends on.
// Keeping this narrow makes the scheduler straightforward to test with a mock.
type Store interface {
	GetSnippetBySlug(ctx context.Context, tenantID, slug string) (*models.Snippet, error)
	GetSnippetEnvironment(ctx context.Context, snippetID, env string) (*models.SnippetEnvironment, error)
	GetVersion(ctx context.Context, id string) (*models.SnippetVersion, error)
	GetVersionByNumber(ctx context.Context, snippetID string, num int) (*models.SnippetVersion, error)
	CreateInvocation(ctx context.Context, snippetID, versionID, env, tenantID, input string) (*models.Invocation, error)
	CreateInvocationWithMode(ctx context.Context, snippetID, versionID, environment, tenantID, inputPayload, invokeMode, callbackURL string, status models.InvocationStatus) (*models.Invocation, error)
	UpdateInvocationResult(ctx context.Context, id string, status models.InvocationStatus, output, errMsg, stderr string, durationMs, peakMemoryMB int) error
	GetInvocation(ctx context.Context, id string) (*models.Invocation, error)
}

// Queue is the subset of *redisstore.Client used by the Scheduler for async jobs.
type Queue interface {
	Enqueue(ctx context.Context, job redisstore.Job) error
}

// InvokeRequest carries the parameters for a snippet invocation.
type InvokeRequest struct {
	TenantID      string
	SnippetSlug   string
	Env           string // "dev" | "prod"
	Input         string // raw JSON
	PinnedVersion int    // 0 = use active version from environment
}

// Scheduler resolves, executes, and records snippet invocations.
type Scheduler struct {
	store    Store
	executor executor.Executor
	queue    Queue // nil in sync-only mode
}

// New creates a Scheduler wired to the given store and executor (sync only, no queue).
func New(store Store, exec executor.Executor) *Scheduler {
	return &Scheduler{store: store, executor: exec}
}

// NewWithQueue creates a Scheduler with an async job queue.
func NewWithQueue(store Store, exec executor.Executor, q Queue) *Scheduler {
	return &Scheduler{store: store, executor: exec, queue: q}
}

// resolveVersion resolves the snippet and version for a request.
// If req.PinnedVersion > 0, fetches that specific version number.
// Otherwise uses the active version from the snippet environment.
func (s *Scheduler) resolveVersion(ctx context.Context, req InvokeRequest) (*models.Snippet, *models.SnippetVersion, error) {
	snippet, err := s.store.GetSnippetBySlug(ctx, req.TenantID, req.SnippetSlug)
	if err != nil {
		return nil, nil, fmt.Errorf("snippet not found: %w", err)
	}

	if req.PinnedVersion > 0 {
		version, err := s.store.GetVersionByNumber(ctx, snippet.ID, req.PinnedVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("get pinned version %d: %w", req.PinnedVersion, err)
		}
		return snippet, version, nil
	}

	// Use the active version from the environment.
	env, err := s.store.GetSnippetEnvironment(ctx, snippet.ID, req.Env)
	if err != nil {
		return nil, nil, fmt.Errorf("get environment: %w", err)
	}
	if env.ActiveVersionID == nil {
		return nil, nil, fmt.Errorf("no published version in environment %q for snippet %q", req.Env, req.SnippetSlug)
	}

	version, err := s.store.GetVersion(ctx, *env.ActiveVersionID)
	if err != nil {
		return nil, nil, fmt.Errorf("get version: %w", err)
	}

	return snippet, version, nil
}

// normaliseInput returns "{}" for empty input.
func normaliseInput(input string) string {
	if input == "" {
		return "{}"
	}
	return input
}

// mapResultStatus maps an executor RunResult to an InvocationStatus.
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

// Invoke executes a snippet synchronously:
//  1. Resolve the snippet and version (active or pinned).
//  2. Create an invocation record (status=running).
//  3. Call the executor.
//  4. Persist the result and return the completed invocation.
func (s *Scheduler) Invoke(ctx context.Context, req InvokeRequest) (*models.Invocation, error) {
	snippet, version, err := s.resolveVersion(ctx, req)
	if err != nil {
		return nil, err
	}

	input := normaliseInput(req.Input)

	invocation, err := s.store.CreateInvocation(ctx, snippet.ID, version.ID, req.Env, req.TenantID, input)
	if err != nil {
		return nil, fmt.Errorf("create invocation: %w", err)
	}

	result := s.executor.Run(ctx, executor.RunSpec{
		Language:    string(snippet.Language),
		Code:        version.Code,
		Input:       input,
		TimeoutMs:   version.TimeoutMs,
		MaxMemoryMB: version.MaxMemoryMB,
	})

	status := mapResultStatus(result)

	updateErr := s.store.UpdateInvocationResult(ctx,
		invocation.ID,
		status,
		result.Output,
		result.Error,
		result.Stderr,
		result.DurationMs,
		result.PeakMemoryMB,
	)
	if updateErr != nil {
		_ = updateErr
	}

	final, err := s.store.GetInvocation(ctx, invocation.ID)
	if err != nil {
		invocation.Status = status
		invocation.Output = result.Output
		invocation.Error = result.Error
		invocation.Stderr = result.Stderr
		invocation.DurationMs = result.DurationMs
		invocation.PeakMemoryMB = result.PeakMemoryMB
		return invocation, nil
	}

	return final, nil
}

// InvokeAsync enqueues the snippet for background execution and returns the
// pending invocation record immediately.
func (s *Scheduler) InvokeAsync(ctx context.Context, req InvokeRequest, callbackURL string) (*models.Invocation, error) {
	if s.queue == nil {
		return nil, fmt.Errorf("async invocation requires a configured queue")
	}

	snippet, version, err := s.resolveVersion(ctx, req)
	if err != nil {
		return nil, err
	}

	input := normaliseInput(req.Input)

	invocation, err := s.store.CreateInvocationWithMode(
		ctx,
		snippet.ID, version.ID, req.Env, req.TenantID, input,
		"async", callbackURL,
		models.InvocationPending,
	)
	if err != nil {
		return nil, fmt.Errorf("create invocation: %w", err)
	}

	job := redisstore.Job{
		InvocationID: invocation.ID,
		SnippetID:    snippet.ID,
		VersionID:    version.ID,
		TenantID:     req.TenantID,
		Language:     string(snippet.Language),
		Code:         version.Code,
		Input:        input,
		TimeoutMs:    version.TimeoutMs,
		MaxMemoryMB:  version.MaxMemoryMB,
		CallbackURL:  callbackURL,
		Env:          req.Env,
	}

	if err := s.queue.Enqueue(ctx, job); err != nil {
		return nil, fmt.Errorf("enqueue job: %w", err)
	}

	return invocation, nil
}

// InvokeStream executes the snippet and streams chunks to the returned channel.
// The caller is responsible for reading from the channel until it is closed.
func (s *Scheduler) InvokeStream(ctx context.Context, req InvokeRequest) (<-chan executor.StreamChunk, *models.Invocation, error) {
	snippet, version, err := s.resolveVersion(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	input := normaliseInput(req.Input)

	invocation, err := s.store.CreateInvocationWithMode(
		ctx,
		snippet.ID, version.ID, req.Env, req.TenantID, input,
		"stream", "",
		models.InvocationRunning,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create invocation: %w", err)
	}

	ch, err := s.executor.RunStream(ctx, executor.RunSpec{
		Language:    string(snippet.Language),
		Code:        version.Code,
		Input:       input,
		TimeoutMs:   version.TimeoutMs,
		MaxMemoryMB: version.MaxMemoryMB,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("run stream: %w", err)
	}

	return ch, invocation, nil
}
