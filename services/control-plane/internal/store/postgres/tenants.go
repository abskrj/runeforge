package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/runeforge/control-plane/internal/models"
)

// CreateTenant inserts a new tenant row and returns the created record.
func (s *Store) CreateTenant(ctx context.Context, name, slug string) (*models.Tenant, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug)
		 VALUES ($1, $2)
		 RETURNING id, name, slug, created_at, egress_policy, replay_enabled, branding`,
		name, slug,
	)

	return scanTenant(row)
}

// GetTenantByID retrieves a tenant by its primary key.
func (s *Store) GetTenantByID(ctx context.Context, id string) (*models.Tenant, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at, egress_policy, replay_enabled, branding FROM tenants WHERE id = $1`,
		id,
	)

	t, err := scanTenant(row)
	if err != nil {
		return nil, fmt.Errorf("GetTenantByID scan: %w", err)
	}
	return t, nil
}

// GetTenantBySlug retrieves a tenant by its unique URL slug.
func (s *Store) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at, egress_policy, replay_enabled, branding FROM tenants WHERE slug = $1`,
		slug,
	)

	t, err := scanTenant(row)
	if err != nil {
		return nil, fmt.Errorf("GetTenantBySlug scan: %w", err)
	}
	return t, nil
}

// UpdateEgressPolicy updates the egress policy for a tenant.
func (s *Store) UpdateEgressPolicy(ctx context.Context, tenantID string, policy models.EgressPolicy) (*models.Tenant, error) {
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("marshal egress policy: %w", err)
	}

	row := s.pool.QueryRow(ctx,
		`UPDATE tenants SET egress_policy = $2
		 WHERE id = $1
		 RETURNING id, name, slug, created_at, egress_policy, replay_enabled, branding`,
		tenantID, policyJSON,
	)

	t, err := scanTenant(row)
	if err != nil {
		return nil, fmt.Errorf("UpdateEgressPolicy scan: %w", err)
	}
	return t, nil
}

// UpdateReplayEnabled toggles replay capability for a tenant.
func (s *Store) UpdateReplayEnabled(ctx context.Context, tenantID string, enabled bool) (*models.Tenant, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE tenants
		 SET replay_enabled = $2
		 WHERE id = $1
		 RETURNING id, name, slug, created_at, egress_policy, replay_enabled, branding`,
		tenantID, enabled,
	)

	t, err := scanTenant(row)
	if err != nil {
		return nil, fmt.Errorf("UpdateReplayEnabled scan: %w", err)
	}
	return t, nil
}

// scanTenant scans a tenant row including the egress_policy JSONB column.
func scanTenant(s scannable) (*models.Tenant, error) {
	var t models.Tenant
	var egressJSON []byte
	var brandingJSON []byte
	if err := s.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedAt, &egressJSON, &t.ReplayEnabled, &brandingJSON); err != nil {
		return nil, err
	}
	if len(egressJSON) > 0 {
		if err := json.Unmarshal(egressJSON, &t.EgressPolicy); err != nil {
			return nil, fmt.Errorf("unmarshal egress_policy: %w", err)
		}
	}
	if len(brandingJSON) > 0 {
		if err := json.Unmarshal(brandingJSON, &t.Branding); err != nil {
			return nil, fmt.Errorf("unmarshal branding: %w", err)
		}
	}
	return &t, nil
}
