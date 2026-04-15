DROP TABLE IF EXISTS tenants CASCADE;

CREATE TABLE tenants (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    settings    JSONB       NOT NULL DEFAULT '{}',
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_is_active ON tenants(is_active);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenants_updated_at
BEFORE UPDATE ON tenants
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE users ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL;

CREATE INDEX idx_users_tenant_id ON users(tenant_id);