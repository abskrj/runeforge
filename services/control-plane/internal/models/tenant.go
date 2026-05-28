package models

import "time"

// Tenant represents an isolated organisation namespace within velane.
type Tenant struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Slug          string       `json:"slug"`
	CreatedAt     time.Time    `json:"created_at"`
	EgressPolicy  EgressPolicy `json:"egress_policy"`
	ReplayEnabled bool         `json:"replay_enabled"`
	Branding      Branding     `json:"branding"`
}

// Branding controls visual identity for embed surfaces.
type Branding struct {
	LogoURL      string `json:"logo_url,omitempty"`
	AccentColor  string `json:"accent_color,omitempty"`
	FontFamily   string `json:"font_family,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
	HideBranding bool   `json:"hide_branding,omitempty"`
}
