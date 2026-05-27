-- Staging environment support: add 'staging' to allowed env values
ALTER TABLE snippet_environments DROP CONSTRAINT IF EXISTS snippet_environments_env_check;
ALTER TABLE snippet_environments ADD CONSTRAINT snippet_environments_env_check
  CHECK (env IN ('dev', 'staging', 'prod'));

ALTER TABLE invocations DROP CONSTRAINT IF EXISTS invocations_environment_check;

-- Canary traffic splitting on snippet_environments
ALTER TABLE snippet_environments ADD COLUMN IF NOT EXISTS canary_version_id TEXT REFERENCES snippet_versions(id);
ALTER TABLE snippet_environments ADD COLUMN IF NOT EXISTS canary_pct INT NOT NULL DEFAULT 0 CHECK (canary_pct >= 0 AND canary_pct <= 100);

-- Secrets table
CREATE TABLE IF NOT EXISTS secrets (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    snippet_id TEXT REFERENCES snippets(id) ON DELETE CASCADE, -- NULL = tenant-wide
    name TEXT NOT NULL,
    value_encrypted TEXT NOT NULL,
    environments TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, snippet_id, name)
);
CREATE INDEX IF NOT EXISTS idx_secrets_tenant_id ON secrets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_secrets_snippet_id ON secrets(snippet_id);

-- Egress policy on tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS egress_policy JSONB NOT NULL DEFAULT '{"blocked_cidrs":["169.254.0.0/16","10.0.0.0/8","172.16.0.0/12","192.168.0.0/16"],"blocked_domains":[]}';
