package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

func hashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateOpaqueToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random token bytes: %w", err)
	}
	return "et_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

// CreateEmbedToken creates a new opaque token and stores only its hash.
func (s *Store) CreateEmbedToken(
	ctx context.Context,
	tenantID string,
	allowedSnippetIDs []string,
	ttl time.Duration,
	createdBy string,
) (*models.EmbedToken, string, error) {
	plain, err := generateOpaqueToken()
	if err != nil {
		return nil, "", err
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	allowedJSON, err := json.Marshal(allowedSnippetIDs)
	if err != nil {
		return nil, "", fmt.Errorf("marshal allowed snippet ids: %w", err)
	}

	row := s.pool.QueryRow(ctx,
		`INSERT INTO embed_tokens (tenant_id, token_hash, allowed_snippet_ids, expires_at, created_by)
		 VALUES ($1, $2, $3, now() + $4::interval, $5)
		 RETURNING id, tenant_id, allowed_snippet_ids, expires_at, revoked_at, created_by, last_used_at, created_at`,
		tenantID, hashOpaqueToken(plain), allowedJSON, formatInterval(ttl), createdBy,
	)
	token, err := scanEmbedToken(row)
	if err != nil {
		return nil, "", fmt.Errorf("CreateEmbedToken scan: %w", err)
	}
	return token, plain, nil
}

func formatInterval(ttl time.Duration) string {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = 3600
	}
	return fmt.Sprintf("%d seconds", seconds)
}

// ListEmbedTokens returns all non-revoked embed tokens for a tenant.
func (s *Store) ListEmbedTokens(ctx context.Context, tenantID string) ([]*models.EmbedToken, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, allowed_snippet_ids, expires_at, revoked_at, created_by, last_used_at, created_at
		 FROM embed_tokens
		 WHERE tenant_id = $1 AND revoked_at IS NULL
		 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListEmbedTokens query: %w", err)
	}
	defer rows.Close()

	var tokens []*models.EmbedToken
	for rows.Next() {
		t, err := scanEmbedToken(rows)
		if err != nil {
			return nil, fmt.Errorf("ListEmbedTokens scan: %w", err)
		}
		tokens = append(tokens, t)
	}
	if tokens == nil {
		tokens = []*models.EmbedToken{}
	}
	return tokens, nil
}

// ValidateEmbedToken validates token hash and expiration/revocation checks.
func (s *Store) ValidateEmbedToken(ctx context.Context, plain string) (*models.EmbedToken, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, allowed_snippet_ids, expires_at, revoked_at, created_by, last_used_at, created_at
		 FROM embed_tokens
		 WHERE token_hash = $1
		   AND revoked_at IS NULL
		   AND expires_at > now()`,
		hashOpaqueToken(plain),
	)
	token, err := scanEmbedToken(row)
	if err != nil {
		return nil, fmt.Errorf("ValidateEmbedToken: %w", err)
	}
	_, _ = s.pool.Exec(ctx, `UPDATE embed_tokens SET last_used_at = now() WHERE id = $1`, token.ID)
	return token, nil
}

// RevokeEmbedToken revokes a token by id, constrained to tenant ownership.
func (s *Store) RevokeEmbedToken(ctx context.Context, tenantID, tokenID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE embed_tokens
		 SET revoked_at = now()
		 WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL`,
		tokenID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("RevokeEmbedToken: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("embed token not found")
	}
	return nil
}

func scanEmbedToken(s scannable) (*models.EmbedToken, error) {
	var token models.EmbedToken
	var allowedJSON []byte
	if err := s.Scan(
		&token.ID,
		&token.TenantID,
		&allowedJSON,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedBy,
		&token.LastUsedAt,
		&token.CreatedAt,
	); err != nil {
		return nil, err
	}
	if len(allowedJSON) > 0 {
		if err := json.Unmarshal(allowedJSON, &token.AllowedSnippetIDs); err != nil {
			return nil, fmt.Errorf("unmarshal allowed_snippet_ids: %w", err)
		}
	}
	if token.AllowedSnippetIDs == nil {
		token.AllowedSnippetIDs = []string{}
	}
	return &token, nil
}
