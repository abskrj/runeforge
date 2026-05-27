package models

import "time"

// Secret represents an encrypted secret value scoped to a tenant and
// optionally to a specific snippet.
type Secret struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	SnippetID      *string   `json:"snippet_id,omitempty"`
	Name           string    `json:"name"`
	// ValueEncrypted is never serialised to JSON responses.
	ValueEncrypted string    `json:"-"`
	Environments   []string  `json:"environments"`
	CreatedAt      time.Time `json:"created_at"`
}
