# Multi-Tenancy

Kleido implements **tenant isolation by design** — every database transaction is scoped to a specific tenant, ensuring complete data separation between organizations.

## Overview

The multi-tenant architecture provides:

- **Tenant isolation**: Users belong to one tenant; cross-tenant data access is prevented
- **Flexible resolution**: Tenant can be specified via subdomain or query parameter
- **Automatic seeding**: A default tenant is created on first startup

## Architecture

```
Request → TenantMiddleware → Context (tenant_id) → Handlers/Services/Repositories
```

### Components

| Component | Path | Purpose |
|-----------|------|---------|
| Model | `internal/model/tenant.go` | Tenant struct (ID, Name, Slug, Settings) |
| Repository | `internal/repository/postgres/tenant.go` | Database operations |
| Service | `internal/service/tenant.go` | Business logic |
| Middleware | `internal/middleware/tenant.go` | Tenant resolution |
| Handler | `internal/handler/tenant.go` | HTTP endpoints |

## Tenant Resolution

The system resolves the current tenant using this priority order:

1. **Subdomain** (highest priority)
   - `tenant1.example.com` → tenant with slug `tenant1`
   
2. **Query Parameter**
   - `?tenant_id=<uuid>` - explicit tenant ID

3. **Tenant Dropdown** (UI fallback)
   - If no tenant detected, login page shows tenant selection dropdown

## API Endpoints

### Public Endpoints

```bash
# List all active tenants
GET /api/v1/tenants

# Get tenant by ID
GET /api/v1/tenants/{id}
```

### Tenant-Aware Endpoints

```bash
# Requires tenant_id in context (subdomain or query param)
GET /api/v1/tenant-demo/status?tenant_id=<uuid>

# Subdomain-based access
curl -H "Host: default.example.com" http://localhost:8080/api/v1/tenant-demo/status
```

## Database Schema

### tenants table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| name | TEXT | Display name |
| slug | TEXT | Unique identifier (used in subdomain) |
| settings | JSONB | Tenant-specific configuration |
| is_active | BOOLEAN | Whether tenant is active |
| created_at | TIMESTAMPTZ | Creation timestamp |
| updated_at | TIMESTAMPTZ | Last update timestamp |

### users table (updated)

| Column | Type | Description |
|--------|------|-------------|
| tenant_id | UUID | FK to tenants (nullable for migration) |

## Testing

```bash
# List tenants
curl http://localhost:8080/api/v1/tenants

# Get specific tenant
curl http://localhost:8080/api/v1/tenants/{id}

# Tenant-aware endpoint - missing tenant_id (should fail)
curl http://localhost:8080/api/v1/tenant-demo/status

# Tenant-aware endpoint - with tenant_id (should succeed)
curl "http://localhost:8080/api/v1/tenant-demo/status?tenant_id=<uuid>"

# Subdomain resolution
curl -H "Host: default.example.com" "http://localhost:8080/api/v1/tenant-demo/status"
```

## Security

- All tenant-aware endpoints require valid tenant context
- Missing tenant_id returns HTTP 400 with error message
- Tenant context is propagated through request context (`reqctx.WithTenantID`)
- Repository queries can be scoped to tenant using `tenant_id` from context