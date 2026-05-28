package models

import (
	"encoding/json"
	"time"
)

// AuditEntry represents a single immutable audit log record.
type AuditEntry struct {
	ID         string          `json:"id"`
	TenantID   string          `json:"tenant_id"`
	ActorID    string          `json:"actor_id"`
	ActorType  string          `json:"actor_type"`  // "user" | "api_key"
	Action     string          `json:"action"`
	ResourceID string          `json:"resource_id,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
