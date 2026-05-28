package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

// ---- Tenant libraries -------------------------------------------------------

func (s *Store) CreateLibrary(ctx context.Context, tenantID, slug, language, name, description string) (*models.Library, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO libraries (tenant_id, slug, language, name, description)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, slug, language, name, description, created_at, updated_at`,
		tenantID, slug, language, name, description,
	)
	var l models.Library
	if err := row.Scan(&l.ID, &l.TenantID, &l.Slug, &l.Language, &l.Name, &l.Description, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return nil, fmt.Errorf("CreateLibrary: %w", err)
	}
	return &l, nil
}

func (s *Store) ListLibraries(ctx context.Context, tenantID string) ([]*models.Library, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, slug, language, name, description, created_at, updated_at
		 FROM libraries WHERE tenant_id = $1
		 ORDER BY name ASC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListLibraries: %w", err)
	}
	defer rows.Close()

	var libs []*models.Library
	for rows.Next() {
		var l models.Library
		if err := rows.Scan(&l.ID, &l.TenantID, &l.Slug, &l.Language, &l.Name, &l.Description, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		libs = append(libs, &l)
	}
	return libs, rows.Err()
}

func (s *Store) GetLibraryByID(ctx context.Context, id, tenantID string) (*models.Library, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, slug, language, name, description, created_at, updated_at
		 FROM libraries WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	var l models.Library
	if err := row.Scan(&l.ID, &l.TenantID, &l.Slug, &l.Language, &l.Name, &l.Description, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return nil, fmt.Errorf("GetLibraryByID: %w", err)
	}
	return &l, nil
}

func (s *Store) DeleteLibrary(ctx context.Context, id, tenantID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM libraries WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("DeleteLibrary: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("library not found")
	}
	return nil
}

// ---- Library versions -------------------------------------------------------

func (s *Store) CreateLibraryVersion(ctx context.Context, libraryID, code string) (*models.LibraryVersion, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO library_versions (library_id, version_number, code)
		 SELECT $1,
		        COALESCE((SELECT MAX(version_number) FROM library_versions WHERE library_id = $1), 0) + 1,
		        $2
		 RETURNING id, library_id, version_number, code, status, published_at, created_at`,
		libraryID, code,
	)
	var v models.LibraryVersion
	if err := row.Scan(&v.ID, &v.LibraryID, &v.VersionNumber, &v.Code, &v.Status, &v.PublishedAt, &v.CreatedAt); err != nil {
		return nil, fmt.Errorf("CreateLibraryVersion: %w", err)
	}
	return &v, nil
}

func (s *Store) ListLibraryVersions(ctx context.Context, libraryID, tenantID string) ([]*models.LibraryVersion, error) {
	// Verify library belongs to tenant.
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM libraries WHERE id = $1 AND tenant_id = $2)`,
		libraryID, tenantID,
	).Scan(&exists); err != nil || !exists {
		return nil, fmt.Errorf("library not found")
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, library_id, version_number, code, status, published_at, created_at
		 FROM library_versions WHERE library_id = $1
		 ORDER BY version_number DESC`,
		libraryID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListLibraryVersions: %w", err)
	}
	defer rows.Close()

	var versions []*models.LibraryVersion
	for rows.Next() {
		var v models.LibraryVersion
		if err := rows.Scan(&v.ID, &v.LibraryID, &v.VersionNumber, &v.Code, &v.Status, &v.PublishedAt, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, &v)
	}
	return versions, rows.Err()
}

func (s *Store) PublishLibraryVersion(ctx context.Context, libraryID, tenantID string, versionNumber int) (*models.LibraryVersion, error) {
	now := time.Now()
	row := s.pool.QueryRow(ctx,
		`UPDATE library_versions lv
		 SET status = 'published', published_at = $4
		 FROM libraries l
		 WHERE lv.library_id = l.id
		   AND lv.library_id = $1
		   AND l.tenant_id = $2
		   AND lv.version_number = $3
		 RETURNING lv.id, lv.library_id, lv.version_number, lv.code, lv.status, lv.published_at, lv.created_at`,
		libraryID, tenantID, versionNumber, now,
	)
	var v models.LibraryVersion
	if err := row.Scan(&v.ID, &v.LibraryID, &v.VersionNumber, &v.Code, &v.Status, &v.PublishedAt, &v.CreatedAt); err != nil {
		return nil, fmt.Errorf("PublishLibraryVersion: %w", err)
	}
	return &v, nil
}

// GetTenantLibrariesForInvocation returns the latest published version of each
// tenant library for the given language, formatted as importPath→code.
//
// Import path: @{tenantSlug}/{slug} (bun) | {tenantSlug_}.{slug_} (python)
//
// Platform libraries are NOT included here — the scheduler merges them from
// the embedded binary via the platformlibs package.
func (s *Store) GetTenantLibrariesForInvocation(ctx context.Context, tenantID, tenantSlug, language string) (map[string]string, error) {
	result := make(map[string]string)

	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT ON (l.slug) l.slug, lv.code
		 FROM libraries l
		 JOIN library_versions lv ON lv.library_id = l.id
		 WHERE l.tenant_id = $1
		   AND l.language = $2
		   AND lv.status = 'published'
		 ORDER BY l.slug, lv.version_number DESC`,
		tenantID, language,
	)
	if err != nil {
		return nil, fmt.Errorf("GetTenantLibrariesForInvocation: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var slug, code string
		if err := rows.Scan(&slug, &code); err != nil {
			return nil, err
		}
		result[importPath(language, tenantSlug, slug)] = code
	}
	return result, rows.Err()
}

// importPath builds the language-specific import path for a library.
func importPath(language, namespace, slug string) string {
	switch language {
	case "python":
		ns := strings.ReplaceAll(namespace, "-", "_")
		mod := strings.ReplaceAll(slug, "-", "_")
		return ns + "." + mod
	default: // bun / typescript
		return "@" + namespace + "/" + slug
	}
}
