-- Extend the secrets table to support non-secret variables (plaintext visible on list)
-- and add updated_at for tracking changes.
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS is_secret BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
