-- Phase 5 foundations: replay and observability references on invocations.
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS input_ref TEXT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS output_ref TEXT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS stderr_ref TEXT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS cpu_ms INT;

-- Tenant-level replay privacy toggle (default off).
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS replay_enabled BOOLEAN NOT NULL DEFAULT false;
