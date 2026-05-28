-- Replace session tokens with JWT refresh tokens
DROP TABLE IF EXISTS user_sessions;

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit log: append-only, no deletes, no updates
CREATE TABLE IF NOT EXISTS audit_log (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_id    TEXT,              -- user_id or api_key_id
    actor_type  TEXT NOT NULL,     -- 'user' | 'api_key'
    action      TEXT NOT NULL,     -- 'publish' | 'secret_create' | 'secret_delete' | 'egress_update' | 'member_invite' | 'member_remove' | 'api_key_create' | 'api_key_revoke' | 'branding_update' | 'canary_set' | 'canary_clear'
    resource_id TEXT,              -- snippet_id, secret_id, etc.
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_tenant_id ON audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(tenant_id, created_at DESC);
