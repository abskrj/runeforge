package models

import "time"

// VersionStatus tracks the lifecycle of a snippet version.
type VersionStatus string

const (
	StatusDraft     VersionStatus = "draft"
	StatusPublished VersionStatus = "published"
	StatusArchived  VersionStatus = "archived"
)

// SnippetVersion is an immutable snapshot of snippet code at a point in time.
type SnippetVersion struct {
	ID            string        `json:"id"`
	SnippetID     string        `json:"snippet_id"`
	VersionNumber int           `json:"version_number"`
	Code          string        `json:"code"`          // stored as plaintext in Phase 1
	InputSchema   string        `json:"input_schema"`  // JSON Schema string
	OutputSchema  string        `json:"output_schema"` // JSON Schema string
	TimeoutMs     int           `json:"timeout_ms"`
	MaxMemoryMB   int           `json:"max_memory_mb"`
	MaxCPUPercent int           `json:"max_cpu_percent"`
	Status        VersionStatus `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	CreatedBy     string        `json:"created_by"`
}

// SnippetEnvironment pins a specific version of a snippet to a deployment
// environment (dev, staging, or prod).
type SnippetEnvironment struct {
	SnippetID       string  `json:"snippet_id"`
	Env             string  `json:"env"` // "dev" | "staging" | "prod"
	ActiveVersionID *string `json:"active_version_id,omitempty"`
	MinInstances    int     `json:"min_instances"`
	CanaryVersionID *string `json:"canary_version_id,omitempty"`
	CanaryPct       int     `json:"canary_pct"`
}
