package postgres

import (
	"context"
	"fmt"

	"github.com/runeforge/control-plane/internal/models"
)

// AddMember inserts a tenant_members row, ignoring conflicts (idempotent).
func (s *Store) AddMember(ctx context.Context, tenantID, userID, role string) (*models.TenantMember, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO tenant_members (tenant_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role
		 RETURNING tenant_id, user_id, role, invited_at`,
		tenantID, userID, role,
	)
	var m models.TenantMember
	if err := row.Scan(&m.TenantID, &m.UserID, &m.Role, &m.InvitedAt); err != nil {
		return nil, fmt.Errorf("AddMember scan: %w", err)
	}
	// Populate email via a follow-up query.
	user, err := s.GetUserByID(ctx, userID)
	if err == nil {
		m.Email = user.Email
	}
	return &m, nil
}

// ListMembers returns all tenant_members rows for the given tenant, joined with users for email.
func (s *Store) ListMembers(ctx context.Context, tenantID string) ([]*models.TenantMember, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT tm.tenant_id, tm.user_id, u.email, tm.role, tm.invited_at
		 FROM tenant_members tm
		 JOIN users u ON u.id = tm.user_id
		 WHERE tm.tenant_id = $1
		 ORDER BY tm.invited_at ASC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListMembers query: %w", err)
	}
	defer rows.Close()

	var members []*models.TenantMember
	for rows.Next() {
		var m models.TenantMember
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Email, &m.Role, &m.InvitedAt); err != nil {
			return nil, fmt.Errorf("ListMembers scan: %w", err)
		}
		members = append(members, &m)
	}
	if members == nil {
		members = []*models.TenantMember{}
	}
	return members, nil
}

// RemoveMember deletes a tenant_members row.
func (s *Store) RemoveMember(ctx context.Context, tenantID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM tenant_members WHERE tenant_id = $1 AND user_id = $2`,
		tenantID, userID,
	)
	return err
}

// GetMemberRole returns the role for a given tenant+user pair, or an error if not a member.
func (s *Store) GetMemberRole(ctx context.Context, tenantID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM tenant_members WHERE tenant_id = $1 AND user_id = $2`,
		tenantID, userID,
	).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("GetMemberRole: user is not a member of this tenant")
	}
	return role, nil
}

// GetUserPrimaryTenantSlug returns the slug of the first tenant the user belongs to,
// ordered by when they were added. Returns ("", nil) if the user has no memberships.
func (s *Store) GetUserPrimaryTenantSlug(ctx context.Context, userID string) (string, error) {
	var slug string
	err := s.pool.QueryRow(ctx,
		`SELECT t.slug FROM tenants t
		 JOIN tenant_members tm ON tm.tenant_id = t.id
		 WHERE tm.user_id = $1
		 ORDER BY tm.invited_at ASC
		 LIMIT 1`,
		userID,
	).Scan(&slug)
	if err != nil {
		return "", nil // no membership is fine — not an error
	}
	return slug, nil
}
