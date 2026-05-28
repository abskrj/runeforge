package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// CreateInvite inserts a new invite_tokens row.
func (s *Store) CreateInvite(ctx context.Context, tenantID, email, role, tokenHash string, expiresAt time.Time) (*models.InviteToken, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO invite_tokens (tenant_id, email, role, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, email, role, token_hash, expires_at, accepted_at, created_at`,
		tenantID, email, role, tokenHash, expiresAt,
	)
	return scanInviteToken(row)
}

// GetInviteByTokenHash retrieves an invite by its hashed token value.
func (s *Store) GetInviteByTokenHash(ctx context.Context, tokenHash string) (*models.InviteToken, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, email, role, token_hash, expires_at, accepted_at, created_at
		 FROM invite_tokens WHERE token_hash = $1`,
		tokenHash,
	)
	inv, err := scanInviteToken(row)
	if err != nil {
		return nil, fmt.Errorf("GetInviteByTokenHash: %w", err)
	}
	return inv, nil
}

// AcceptInvite sets accepted_at = now() for the given invite ID.
func (s *Store) AcceptInvite(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE invite_tokens SET accepted_at = now() WHERE id = $1`,
		id,
	)
	return err
}

// ListPendingInvites returns all invite_tokens for the tenant where accepted_at IS NULL and expires_at > now().
func (s *Store) ListPendingInvites(ctx context.Context, tenantID string) ([]*models.InviteToken, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, email, role, token_hash, expires_at, accepted_at, created_at
		 FROM invite_tokens
		 WHERE tenant_id = $1 AND accepted_at IS NULL AND expires_at > now()
		 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListPendingInvites query: %w", err)
	}
	defer rows.Close()

	var invites []*models.InviteToken
	for rows.Next() {
		inv, err := scanInviteToken(rows)
		if err != nil {
			return nil, fmt.Errorf("ListPendingInvites scan: %w", err)
		}
		invites = append(invites, inv)
	}
	if invites == nil {
		invites = []*models.InviteToken{}
	}
	return invites, nil
}

func scanInviteToken(s scannable) (*models.InviteToken, error) {
	var inv models.InviteToken
	if err := s.Scan(
		&inv.ID, &inv.TenantID, &inv.Email, &inv.Role,
		&inv.TokenHash, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &inv, nil
}
