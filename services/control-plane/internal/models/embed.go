package models

import "time"

// EmbedToken represents a read-only token used by the embed dashboard.
type EmbedToken struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	AllowedSnippetIDs []string   `json:"allowed_snippet_ids"`
	ExpiresAt         time.Time  `json:"expires_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	CreatedBy         string     `json:"created_by"`
	LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// EmbedContext carries authorization scope resolved from an embed token.
type EmbedContext struct {
	TokenID           string
	TenantID          string
	AllowedSnippetIDs []string
	ExpiresAt         time.Time
}
