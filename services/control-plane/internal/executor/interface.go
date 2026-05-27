package executor

import "context"

// RunSpec describes a single snippet execution request sent to an executor.
type RunSpec struct {
	Language    string // "bun" | "python"
	Code        string // raw source code of the snippet
	Input       string // raw JSON input payload
	TimeoutMs   int    // execution deadline in milliseconds
	MaxMemoryMB int    // soft memory ceiling in MiB
}

// RunResult captures the outcome of a snippet execution.
type RunResult struct {
	Output       string // raw JSON written to stdout by the snippet
	Stderr       string // anything written to stderr
	DurationMs   int    // wall-clock execution time in milliseconds
	PeakMemoryMB int    // peak RSS in MiB (best-effort)
	ExitCode     int    // OS exit code of the subprocess
	Error        string // "timeout" | "oom" | "" for other errors, or error message
}

// StreamChunk is one piece of output emitted by a streaming snippet.
type StreamChunk struct {
	Data  string // partial JSON or text emitted by the snippet
	Error string // non-empty on error
	Done  bool   // true on the final chunk
}

// Executor is the interface that all execution backends must satisfy.
type Executor interface {
	// Run executes a snippet synchronously and returns the full result.
	Run(ctx context.Context, spec RunSpec) RunResult

	// RunStream calls the executor's streaming endpoint and returns a channel
	// of StreamChunks. The channel is closed when the stream ends or the
	// context is cancelled. The caller must drain the channel.
	RunStream(ctx context.Context, spec RunSpec) (<-chan StreamChunk, error)
}
