package remote

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/executor"
)

// egressPolicy mirrors executor.EgressPolicy for JSON serialisation.
type egressPolicy struct {
	BlockedCIDRs   []string `json:"blocked_cidrs"`
	BlockedDomains []string `json:"blocked_domains"`
}

// runRequest is the JSON body sent to executor containers.
type runRequest struct {
	Code          string            `json:"code"`
	Input         string            `json:"input"`
	TimeoutMs     int               `json:"timeout_ms"`
	MaxMemoryMB   int               `json:"max_memory_mb"`
	SecretEnvVars map[string]string `json:"secret_env_vars,omitempty"`
	Libraries     map[string]string `json:"libraries,omitempty"`
	EgressPolicy  *egressPolicy     `json:"egress_policy,omitempty"`
}

// runResponse is the JSON body returned by executor containers.
type runResponse struct {
	Output       string `json:"output"`
	Stderr       string `json:"stderr"`
	DurationMs   int    `json:"duration_ms"`
	PeakMemoryMB int    `json:"peak_memory_mb"`
	ExitCode     int    `json:"exit_code"`
	Error        string `json:"error"`
}

// RemoteExecutor routes execution requests to the appropriate language
// executor container over HTTP.
type RemoteExecutor struct {
	bunEndpoint    string // e.g. "http://bun-executor:8080"
	pythonEndpoint string // e.g. "http://python-executor:8080"
	httpClient     *http.Client
}

// New creates a RemoteExecutor with the supplied endpoint URLs.
func New(bunEndpoint, pythonEndpoint string) *RemoteExecutor {
	return &RemoteExecutor{
		bunEndpoint:    bunEndpoint,
		pythonEndpoint: pythonEndpoint,
		httpClient: &http.Client{
			// The outer context carries the real deadline; this is a backstop.
			Timeout: 5 * time.Minute,
		},
	}
}

// buildRunRequest converts an executor.RunSpec into the wire-format request body.
func buildRunRequest(spec executor.RunSpec) runRequest {
	req := runRequest{
		Code:          spec.Code,
		Input:         spec.Input,
		TimeoutMs:     spec.TimeoutMs,
		MaxMemoryMB:   spec.MaxMemoryMB,
		SecretEnvVars: spec.SecretEnvVars,
		Libraries:     spec.Libraries,
	}
	if spec.EgressPolicy != nil {
		req.EgressPolicy = &egressPolicy{
			BlockedCIDRs:   spec.EgressPolicy.BlockedCIDRs,
			BlockedDomains: spec.EgressPolicy.BlockedDomains,
		}
	}
	return req
}

// Run sends the spec to the appropriate executor container and returns the
// result. HTTP and JSON errors are surfaced in RunResult.Error.
func (e *RemoteExecutor) Run(ctx context.Context, spec executor.RunSpec) executor.RunResult {
	endpoint, err := e.endpointFor(spec.Language)
	if err != nil {
		return executor.RunResult{Error: err.Error(), ExitCode: -1}
	}

	reqBody, err := json.Marshal(buildRunRequest(spec))
	if err != nil {
		return executor.RunResult{Error: fmt.Sprintf("marshal request: %v", err), ExitCode: -1}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/run", bytes.NewReader(reqBody))
	if err != nil {
		return executor.RunResult{Error: fmt.Sprintf("build request: %v", err), ExitCode: -1}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return executor.RunResult{Error: fmt.Sprintf("executor http: %v", err), ExitCode: -1}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return executor.RunResult{Error: fmt.Sprintf("read body: %v", err), ExitCode: -1}
	}

	if resp.StatusCode >= 500 {
		return executor.RunResult{
			Error:    fmt.Sprintf("executor returned %d: %s", resp.StatusCode, string(body)),
			ExitCode: -1,
		}
	}

	var runResp runResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		return executor.RunResult{Error: fmt.Sprintf("unmarshal response: %v", err), ExitCode: -1}
	}

	return executor.RunResult{
		Output:       runResp.Output,
		Stderr:       runResp.Stderr,
		DurationMs:   runResp.DurationMs,
		PeakMemoryMB: runResp.PeakMemoryMB,
		ExitCode:     runResp.ExitCode,
		Error:        runResp.Error,
	}
}

// RunStream calls POST /run/stream on the executor container.
// The executor returns text/event-stream. Each SSE "data:" line is a JSON
// StreamChunk. The method returns a channel immediately; a goroutine reads the
// SSE stream and sends chunks. The channel is closed when the stream ends or
// the context is cancelled.
func (e *RemoteExecutor) RunStream(ctx context.Context, spec executor.RunSpec) (<-chan executor.StreamChunk, error) {
	endpoint, err := e.endpointFor(spec.Language)
	if err != nil {
		return nil, err
	}

	reqBody, err := json.Marshal(buildRunRequest(spec))
	if err != nil {
		return nil, fmt.Errorf("runstream marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/run/stream", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("runstream build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("runstream http: %w", err)
	}

	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("runstream executor returned %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan executor.StreamChunk, 16)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Blank lines are SSE field separators — skip.
			if line == "" {
				continue
			}

			// SSE lines that carry data start with "data: ".
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			payload := strings.TrimPrefix(line, "data: ")

			var chunk executor.StreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				// Emit an error chunk and stop.
				select {
				case ch <- executor.StreamChunk{Error: fmt.Sprintf("unmarshal chunk: %v", err), Done: true}:
				case <-ctx.Done():
				}
				return
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}

			if chunk.Done {
				return
			}
		}

		// Scanner finished (EOF or error).
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- executor.StreamChunk{Error: fmt.Sprintf("stream read error: %v", err), Done: true}:
			default:
			}
		}
	}()

	return ch, nil
}

func (e *RemoteExecutor) endpointFor(language string) (string, error) {
	switch language {
	case "bun":
		return e.bunEndpoint, nil
	case "python":
		return e.pythonEndpoint, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}
