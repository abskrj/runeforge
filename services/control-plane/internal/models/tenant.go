package models

import "time"

// Tenant represents an isolated organisation namespace within runeforge.
type Tenant struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Slug         string       `json:"slug"`
	CreatedAt    time.Time    `json:"created_at"`
	EgressPolicy EgressPolicy `json:"egress_policy"`
}
