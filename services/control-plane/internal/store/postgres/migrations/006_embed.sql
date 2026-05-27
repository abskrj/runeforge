ALTER TABLE tenants ADD COLUMN IF NOT EXISTS branding JSONB NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS embed_tokens (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    allowed_snippet_ids JSONB NOT NULL DEFAULT '[]',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_by TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_embed_tokens_tenant_id ON embed_tokens(tenant_id);
CREATE INDEX IF NOT EXISTS idx_embed_tokens_expires_at ON embed_tokens(expires_at);
