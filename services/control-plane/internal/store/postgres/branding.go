package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// GetBranding reads the branding JSONB column from the tenants table.
func (s *Store) GetBranding(ctx context.Context, tenantID string) (*models.Branding, error) {
	var brandingJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT branding FROM tenants WHERE id = $1`,
		tenantID,
	).Scan(&brandingJSON)
	if err != nil {
		return nil, fmt.Errorf("GetBranding: %w", err)
	}

	var b models.Branding
	if len(brandingJSON) > 0 {
		if err := json.Unmarshal(brandingJSON, &b); err != nil {
			return nil, fmt.Errorf("GetBranding unmarshal: %w", err)
		}
	}
	return &b, nil
}

// UpdateBranding persists the branding config for the given tenant.
func (s *Store) UpdateBranding(ctx context.Context, tenantID string, b models.Branding) error {
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("UpdateBranding marshal: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE tenants SET branding = $2 WHERE id = $1`,
		tenantID, data,
	)
	return err
}
