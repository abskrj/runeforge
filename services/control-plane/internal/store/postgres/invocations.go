package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runeforge/control-plane/internal/models"
)

// CreateInvocation inserts a new invocation record.
// The status defaults to 'running' for sync invocations; callers that want
// 'pending' (async) should call CreateInvocationWithMode.
func (s *Store) CreateInvocation(ctx context.Context, snippetID, versionID, environment, tenantID, inputPayload string) (*models.Invocation, error) {
	return s.CreateInvocationWithMode(ctx, snippetID, versionID, environment, tenantID, inputPayload, "sync", "", models.InvocationRunning)
}

// CreateInvocationWithMode inserts a new invocation record with explicit mode,
// callback URL, and initial status.
func (s *Store) CreateInvocationWithMode(
	ctx context.Context,
	snippetID, versionID, environment, tenantID, inputPayload, invokeMode, callbackURL string,
	status models.InvocationStatus,
) (*models.Invocation, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO invocations
		   (snippet_id, version_id, environment, tenant_id, status, input_payload, invoke_mode, callback_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, snippet_id, version_id, environment, tenant_id, status,
		           input_payload, input_ref, output, output_ref, error, stderr, stderr_ref, duration_ms, peak_memory_mb, cpu_ms,
		           created_at, completed_at, callback_url, invoke_mode`,
		snippetID, versionID, environment, tenantID, string(status), inputPayload, invokeMode, nullableString(callbackURL),
	)
	inv, err := scanInvocation(row)
	if err != nil {
		return nil, fmt.Errorf("CreateInvocationWithMode scan: %w", err)
	}
	return inv, nil
}

// UpdateInvocationResult updates an invocation with the execution result.
func (s *Store) UpdateInvocationResult(ctx context.Context, id string, status models.InvocationStatus, output, errMsg, stderr string, durationMs, peakMemoryMB, cpuMs int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE invocations
		 SET status       = $2,
		     output       = $3,
		     error        = $4,
		     stderr       = $5,
		     duration_ms  = $6,
		     peak_memory_mb = $7,
		     cpu_ms       = $8,
		     completed_at = now()
		 WHERE id = $1`,
		id, string(status), output, errMsg, stderr, durationMs, peakMemoryMB, cpuMs,
	)
	if err != nil {
		return fmt.Errorf("UpdateInvocationResult: %w", err)
	}
	return nil
}

// InvocationLogFilters filters invocation log query results.
type InvocationLogFilters struct {
	Environment string
	Status      string
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
}

// SnippetMetrics contains aggregate and timeseries metrics for a snippet.
type SnippetMetrics struct {
	Window        string
	TotalCount    int64
	Completed     int64
	Failed        int64
	P50DurationMs float64
	P95DurationMs float64
	P99DurationMs float64
	AvgDurationMs float64
	Series        []SnippetMetricsPoint
}

// SnippetMetricsPoint is one timeseries bucket.
type SnippetMetricsPoint struct {
	BucketStart   time.Time `json:"bucket_start"`
	Count         int64     `json:"count"`
	P95DurationMs float64   `json:"p95_duration_ms"`
}

// ListInvocationLogs returns filtered invocations for logs endpoint.
func (s *Store) ListInvocationLogs(ctx context.Context, snippetID string, filters InvocationLogFilters) ([]*models.Invocation, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	clauses := []string{"snippet_id = $1"}
	args := []any{snippetID}
	argPos := 2

	if filters.Environment != "" {
		clauses = append(clauses, fmt.Sprintf("environment = $%d", argPos))
		args = append(args, filters.Environment)
		argPos++
	}
	if filters.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", argPos))
		args = append(args, filters.Status)
		argPos++
	}
	if filters.StartTime != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argPos))
		args = append(args, *filters.StartTime)
		argPos++
	}
	if filters.EndTime != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argPos))
		args = append(args, *filters.EndTime)
		argPos++
	}

	query := fmt.Sprintf(
		`SELECT id, snippet_id, version_id, environment, tenant_id, status,
		        input_payload, input_ref, output, output_ref, error, stderr, stderr_ref, duration_ms, peak_memory_mb, cpu_ms,
		        created_at, completed_at, callback_url, invoke_mode
		 FROM invocations WHERE %s
		 ORDER BY created_at DESC
		 LIMIT $%d`,
		strings.Join(clauses, " AND "),
		argPos,
	)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListInvocationLogs query: %w", err)
	}
	defer rows.Close()

	var invocations []*models.Invocation
	for rows.Next() {
		inv, scanErr := scanInvocation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		invocations = append(invocations, inv)
	}

	return invocations, rows.Err()
}

// GetSnippetMetrics computes aggregate and per-hour metrics in the requested window.
func (s *Store) GetSnippetMetrics(ctx context.Context, snippetID, window string, since time.Time) (*SnippetMetrics, error) {
	m := &SnippetMetrics{Window: window}

	aggRow := s.pool.QueryRow(ctx,
		`SELECT
		    COUNT(*)::bigint AS total_count,
		    COUNT(*) FILTER (WHERE status = 'completed')::bigint AS completed_count,
		    COUNT(*) FILTER (WHERE status IN ('failed', 'timeout', 'oom_killed'))::bigint AS failed_count,
		    COALESCE(AVG(duration_ms), 0)::float8 AS avg_duration_ms,
		    COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_ms), 0)::float8 AS p50_duration_ms,
		    COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::float8 AS p95_duration_ms,
		    COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_ms), 0)::float8 AS p99_duration_ms
		 FROM invocations
		 WHERE snippet_id = $1
		   AND created_at >= $2
		   AND duration_ms IS NOT NULL`,
		snippetID, since,
	)

	if err := aggRow.Scan(
		&m.TotalCount,
		&m.Completed,
		&m.Failed,
		&m.AvgDurationMs,
		&m.P50DurationMs,
		&m.P95DurationMs,
		&m.P99DurationMs,
	); err != nil {
		return nil, fmt.Errorf("GetSnippetMetrics aggregate scan: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT
		    date_trunc('hour', created_at) AS bucket_start,
		    COUNT(*)::bigint AS count,
		    COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::float8 AS p95_duration_ms
		 FROM invocations
		 WHERE snippet_id = $1
		   AND created_at >= $2
		   AND duration_ms IS NOT NULL
		 GROUP BY date_trunc('hour', created_at)
		 ORDER BY bucket_start ASC`,
		snippetID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("GetSnippetMetrics series query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var point SnippetMetricsPoint
		if scanErr := rows.Scan(&point.BucketStart, &point.Count, &point.P95DurationMs); scanErr != nil {
			return nil, fmt.Errorf("GetSnippetMetrics series scan: %w", scanErr)
		}
		m.Series = append(m.Series, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetSnippetMetrics series rows: %w", err)
	}

	return m, nil
}

// GetInvocation retrieves a single invocation by its primary key.
func (s *Store) GetInvocation(ctx context.Context, id string) (*models.Invocation, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, snippet_id, version_id, environment, tenant_id, status,
		        input_payload, input_ref, output, output_ref, error, stderr, stderr_ref, duration_ms, peak_memory_mb, cpu_ms,
		        created_at, completed_at, callback_url, invoke_mode
		 FROM invocations WHERE id = $1`,
		id,
	)
	inv, err := scanInvocation(row)
	if err != nil {
		return nil, fmt.Errorf("GetInvocation: %w", err)
	}
	return inv, nil
}

// ListInvocationsBySnippet returns recent invocations for a snippet.
func (s *Store) ListInvocationsBySnippet(ctx context.Context, snippetID string, limit int) ([]*models.Invocation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, snippet_id, version_id, environment, tenant_id, status,
		        input_payload, input_ref, output, output_ref, error, stderr, stderr_ref, duration_ms, peak_memory_mb, cpu_ms,
		        created_at, completed_at, callback_url, invoke_mode
		 FROM invocations WHERE snippet_id = $1
		 ORDER BY created_at DESC LIMIT $2`,
		snippetID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ListInvocationsBySnippet query: %w", err)
	}
	defer rows.Close()

	var invocations []*models.Invocation
	for rows.Next() {
		inv, err := scanInvocation(rows)
		if err != nil {
			return nil, err
		}
		invocations = append(invocations, inv)
	}
	return invocations, rows.Err()
}

func scanInvocation(s scannable) (*models.Invocation, error) {
	var inv models.Invocation
	var inputRef, output, outputRef, errMsg, stderr, stderrRef, callbackURL *string
	var durationMs, peakMemoryMB, cpuMs *int
	if err := s.Scan(
		&inv.ID, &inv.SnippetID, &inv.VersionID, &inv.Environment, &inv.TenantID,
		&inv.Status, &inv.InputPayload,
		&inputRef, &output, &outputRef, &errMsg, &stderr, &stderrRef,
		&durationMs, &peakMemoryMB, &cpuMs,
		&inv.CreatedAt, &inv.CompletedAt,
		&callbackURL, &inv.InvokeMode,
	); err != nil {
		return nil, err
	}
	if inputRef != nil {
		inv.InputRef = *inputRef
	}
	if output != nil {
		inv.Output = *output
	}
	if outputRef != nil {
		inv.OutputRef = *outputRef
	}
	if errMsg != nil {
		inv.Error = *errMsg
	}
	if stderr != nil {
		inv.Stderr = *stderr
	}
	if stderrRef != nil {
		inv.StderrRef = *stderrRef
	}
	if durationMs != nil {
		inv.DurationMs = *durationMs
	}
	if peakMemoryMB != nil {
		inv.PeakMemoryMB = *peakMemoryMB
	}
	if cpuMs != nil {
		inv.CPUMs = *cpuMs
	}
	if callbackURL != nil {
		inv.CallbackURL = *callbackURL
	}
	return &inv, nil
}

// nullableString converts an empty string to nil (SQL NULL).
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
