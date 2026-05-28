package models

import "time"

// Secret represents an encrypted value scoped to a tenant and optionally a snippet.
// When IsSecret is false (a "variable"), Value is populated in list responses.
// When IsSecret is true (a "credential"), Value is always nil in responses.
type Secret struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	SnippetID    *string   `json:"snippet_id,omitempty"`
	Name         string    `json:"name"`
	IsSecret     bool      `json:"is_secret"`
	Value        *string   `json:"value,omitempty"` // non-nil for variables in list responses
	Environments []string  `json:"environments"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
