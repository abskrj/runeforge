package models

// EgressPolicy defines the network egress restrictions for a tenant.
type EgressPolicy struct {
	BlockedCIDRs   []string `json:"blocked_cidrs"`
	BlockedDomains []string `json:"blocked_domains"`
}
