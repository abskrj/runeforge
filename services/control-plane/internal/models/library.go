package models

import "time"

// Library is a tenant-owned reusable code module with full version management.
type Library struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Slug        string    `json:"slug"`
	Language    string    `json:"language"` // "bun" | "python"
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LibraryVersion is one immutable version of a tenant Library.
type LibraryVersion struct {
	ID            string     `json:"id"`
	LibraryID     string     `json:"library_id"`
	VersionNumber int        `json:"version_number"`
	Code          string     `json:"code"`
	Status        string     `json:"status"` // "draft" | "published" | "archived"
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
