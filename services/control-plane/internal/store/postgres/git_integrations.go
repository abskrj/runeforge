package postgres

import (
	"context"
	"fmt"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// CreateGitIntegration inserts a new git integration for a snippet.
func (s *Store) CreateGitIntegration(ctx context.Context, tenantID, snippetID, provider, repoURL, secret string) (*models.GitIntegration, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO git_integrations (tenant_id, snippet_id, provider, repo_url, secret)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, snippet_id, provider, repo_url, secret, created_at`,
		tenantID, snippetID, provider, repoURL, secret,
	)
	return scanGitIntegration(row)
}

// GetGitIntegrationBySnippet retrieves a git integration scoped to a tenant+snippet.
func (s *Store) GetGitIntegrationBySnippet(ctx context.Context, tenantID, snippetID string) (*models.GitIntegration, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, snippet_id, provider, repo_url, secret, created_at
		 FROM git_integrations WHERE tenant_id = $1 AND snippet_id = $2`,
		tenantID, snippetID,
	)
	gi, err := scanGitIntegration(row)
	if err != nil {
		return nil, fmt.Errorf("git integration not found: %w", err)
	}
	return gi, nil
}

// DeleteGitIntegration removes a git integration for a tenant+snippet pair.
func (s *Store) DeleteGitIntegration(ctx context.Context, tenantID, snippetID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM git_integrations WHERE tenant_id = $1 AND snippet_id = $2`,
		tenantID, snippetID,
	)
	if err != nil {
		return fmt.Errorf("DeleteGitIntegration: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("git integration not found")
	}
	return nil
}

// GetGitIntegrationBySnippetID retrieves a git integration by snippet ID only,
// used by the webhook handler which receives requests without tenant auth.
func (s *Store) GetGitIntegrationBySnippetID(ctx context.Context, snippetID string) (*models.GitIntegration, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, snippet_id, provider, repo_url, secret, created_at
		 FROM git_integrations WHERE snippet_id = $1`,
		snippetID,
	)
	gi, err := scanGitIntegration(row)
	if err != nil {
		return nil, fmt.Errorf("git integration not found: %w", err)
	}
	return gi, nil
}

func scanGitIntegration(s scannable) (*models.GitIntegration, error) {
	var gi models.GitIntegration
	if err := s.Scan(&gi.ID, &gi.TenantID, &gi.SnippetID, &gi.Provider, &gi.RepoURL, &gi.Secret, &gi.CreatedAt); err != nil {
		return nil, err
	}
	return &gi, nil
}
