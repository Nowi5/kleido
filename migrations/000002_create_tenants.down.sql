DROP TRIGGER IF EXISTS tenants_updated_at ON tenants;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS tenants CASCADE;