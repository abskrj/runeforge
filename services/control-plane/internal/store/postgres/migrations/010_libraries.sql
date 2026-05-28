-- Platform-level libraries: no versioning, code is live immediately.
CREATE TABLE IF NOT EXISTS platform_libraries (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    slug        TEXT NOT NULL,
    language    TEXT NOT NULL CHECK (language IN ('bun', 'python')),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    code        TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (slug, language)
);

-- Tenant-owned libraries with full draft/publish versioning.
CREATE TABLE IF NOT EXISTS libraries (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    language    TEXT NOT NULL CHECK (language IN ('bun', 'python')),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, slug, language)
);

CREATE TABLE IF NOT EXISTS library_versions (
    id             TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    library_id     TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    code           TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    published_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (library_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_library_versions_library_id ON library_versions(library_id);
CREATE INDEX IF NOT EXISTS idx_libraries_tenant_id ON libraries(tenant_id);
