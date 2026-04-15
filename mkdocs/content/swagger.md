# Swagger UI — API Explorer

Swagger UI is available in development mode at:

**[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)**

It provides an interactive interface to explore every endpoint, read schemas, and execute real HTTP requests — no Postman or curl required.

!!! info "Development only"
    Swagger UI is disabled when `APP_ENV=production`. The raw OpenAPI spec is always available at `/swagger/doc.json` regardless of environment.

---

## Endpoints overview

### Public — no authentication required

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/register` | Create a new user account |
| `POST` | `/api/v1/auth/login` | Authenticate and receive an access token |
| `POST` | `/api/v1/auth/refresh` | Exchange a refresh cookie for a new access token |

### Protected — Bearer token required

| Method | Path | Description | Admin only |
|--------|------|-------------|-----------|
| `POST` | `/api/v1/auth/logout` | Revoke the current session | — |
| `GET` | `/api/v1/users/me` | Get your own profile | — |
| `GET` | `/api/v1/users` | List all users (paginated) | Yes |
| `GET` | `/api/v1/users/{id}` | Get a user by UUID | — |
| `PUT` | `/api/v1/users/{id}` | Update a user's email, role, or status | — |
| `DELETE` | `/api/v1/users/{id}` | Soft-delete a user | Yes |

### System — not in Swagger spec

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness probe (always 200 if process is running) |
| `GET` | `/readyz` | Readiness probe (503 if DB or Redis is unreachable) |
| `GET` | `/metrics` | Prometheus scrape target |

---

## Walkthrough: register → login → call a protected endpoint

### Step 1 — Register a user

1. Open [Swagger UI](http://localhost:8080/swagger/index.html)
2. Click **POST /api/v1/auth/register** → **Try it out**
3. Paste the request body:

    ```json
    {
      "email": "alice@example.com",
      "password": "SuperSecret123!"
    }
    ```

4. Click **Execute**
5. You should receive **201 Created** with a `UserResponse`:

    ```json
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "alice@example.com",
      "role": "user",
      "is_active": true,
      "created_at": "2026-04-14T10:00:00Z",
      "updated_at": "2026-04-14T10:00:00Z"
    }
    ```

---

### Step 2 — Log in and get a token

1. Click **POST /api/v1/auth/login** → **Try it out**
2. Use the same credentials:

    ```json
    {
      "email": "alice@example.com",
      "password": "SuperSecret123!"
    }
    ```

3. Click **Execute** → **200 OK**
4. Copy the `access_token` from the response:

    ```json
    {
      "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
      "expires_at": "2026-04-14T10:15:00Z"
    }
    ```

    !!! tip
        The access token is valid for **15 minutes** by default (`JWT_ACCESS_TOKEN_TTL_MINUTES`).
        A `refresh_token` is also set as an `HttpOnly` cookie and lasts **7 days**.

---

### Step 3 — Authorise Swagger UI

1. Click the **Authorize** button (lock icon) at the top right of Swagger UI
2. In the **BearerAuth** field enter:

    ```
    Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
    ```

    (prefix `Bearer ` then paste your token)

3. Click **Authorize** → **Close**

All subsequent requests will include this token in the `Authorization` header.

---

### Step 4 — Call a protected endpoint

1. Click **GET /api/v1/users/me** → **Try it out** → **Execute**
2. You should receive **200 OK** with your profile.

---

### Step 5 — Refresh your token

When the access token expires (15 min), use the refresh endpoint — the browser cookie is automatically sent by Swagger UI since `credentials: true` is set in the CORS config.

1. Click **POST /api/v1/auth/refresh** → **Try it out** → **Execute**
2. A new `access_token` is returned. Repeat Step 3 to update Swagger's auth header.

---

## Request / response schemas

Click any endpoint to expand it, then scroll to the **Schemas** section at the bottom of the page or click a schema name inline to see field descriptions and validation rules.

### Key schemas

=== "RegisterRequest"

    ```json
    {
      "email": "string (valid email, required)",
      "password": "string (min 8 chars, required)"
    }
    ```

=== "LoginRequest"

    ```json
    {
      "email": "string (required)",
      "password": "string (required)"
    }
    ```

=== "TokenResponse"

    ```json
    {
      "access_token": "string (RS256 JWT)",
      "expires_at": "string (RFC3339 timestamp)"
    }
    ```

=== "UserResponse"

    ```json
    {
      "id": "string (UUID v4)",
      "email": "string",
      "role": "string (user | admin)",
      "is_active": "boolean",
      "created_at": "string (RFC3339)",
      "updated_at": "string (RFC3339)"
    }
    ```

=== "UpdateUserRequest"

    ```json
    {
      "email": "string (optional)",
      "role": "string (optional, admin only)",
      "is_active": "boolean (optional, admin only)"
    }
    ```

=== "ListUsersResponse"

    ```json
    {
      "users": ["UserResponse"],
      "total": "integer",
      "page": "integer",
      "per_page": "integer"
    }
    ```

---

## Rate limiting

Protected endpoints are rate-limited to **100 requests per minute per user**. Exceeding the limit returns `429 Too Many Requests`.

---

## OpenAPI spec

The raw spec is served at:

```bash
curl http://localhost:8080/swagger/doc.json | jq .
```

It can be imported into any OpenAPI-compatible tool (Postman, Insomnia, etc.).

---

## Pagination

The `GET /api/v1/users` endpoint accepts:

| Query parameter | Default | Max | Description |
|-----------------|---------|-----|-------------|
| `page` | `1` | — | Page number (1-indexed) |
| `per_page` | `20` | `100` | Items per page |

Example:

```bash
curl -H "Authorization: Bearer <token>" \
     "http://localhost:8080/api/v1/users?page=2&per_page=10"
```
