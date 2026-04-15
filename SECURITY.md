# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please email **security@nowi5.com**.
Do **NOT** open a public GitHub issue.

We will acknowledge receipt within **48 hours** and aim to release a fix within **14 days**
for critical issues. We follow responsible disclosure: please allow us time to patch
before publishing details publicly.

---

## Security Architecture

### Authentication Model

| Component | Implementation | Notes |
|-----------|---------------|-------|
| Access tokens | RS256-signed JWTs | 15-minute lifetime |
| Signing key | RSA-4096 private key | Never leaves the auth service |
| Verification | RSA-4096 public key | Resource services never see the private key |
| Access token transport | `Authorization: Bearer` header | — |
| Refresh token transport | `httpOnly; Secure; SameSite=Strict` cookie | Refresh token never appears in the response body |
| Token revocation | JTI blocklist in Redis | TTL = remaining access token lifetime |
| Refresh token invalidation | Deleted from Redis | Rotation on every refresh |
| Password hashing | bcrypt | Minimum cost factor 12 |
| Password reset | Single-use token in Redis | SHA-256 hashed at rest, 1-hour TTL, consumed before password update |

### Transport

- All production traffic must use HTTPS/TLS 1.2+
- HSTS header enforced in production (`Strict-Transport-Security: max-age=31536000; includeSubDomains`)

### Input Validation

- All request bodies validated with `go-playground/validator/v10` struct tags before reaching the service layer
- UUIDs parsed and rejected early in handlers — never passed as raw strings to the DB layer
- SQL injection: impossible via pgx parameterised queries — string concatenation into SQL is a lint error (`gosec G201`)

### HTTP Security Headers

The `SecurityHeaders` middleware sets these headers on every response:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `1; mode=block` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` |
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` (production only) |

---

## Rate Limiting

Two independent rate-limiting layers protect the API:

| Layer | Scope | Limit | Window | Storage | Rejection |
|-------|-------|-------|--------|---------|-----------|
| Per-IP (global) | All authenticated endpoints | 100 req | 1 minute | Redis sliding window | 429 |
| Per-user | Authenticated endpoints (by route pattern) | 100 req | 1 minute | Redis sliding window | 429 |

### Brute-force Login Lockout

In addition to IP-level rate limiting, the login endpoint enforces email-level lockout:

- **Threshold:** 10 consecutive failures for the same email address
- **Window:** 15 minutes (sliding, via Redis INCR/EXPIRE)
- **Response when locked:** HTTP 429 with a generic error message (same message as invalid credentials — no enumeration)
- **Unlock:** automatic when the 15-minute window expires, or immediately on the next successful login
- **Redis key privacy:** email is SHA-256 hashed before use as a Redis key segment — no PII in Redis key names
- **Fail-open policy:** if Redis is unreachable during the lockout check, the request is **allowed** (with a warning log) — rate-limit failures must never cause a complete service outage

---

## Auth Audit Log

Every authentication event is written to the structured log as a `log/slog` record with `level=INFO`. Events include the following fixed fields:

| Field | Example | Notes |
|-------|---------|-------|
| `event_type` | `auth.login.success` | One of the seven vocabulary terms below |
| `ip` | `203.0.113.4:1234` | From `X-Forwarded-For` or `RemoteAddr` |
| `user_agent` | `Mozilla/5.0 ...` | Truncated in log output |
| `email_masked` | `u***@e***.com` | Local-part first char + `***`; domain first char + `***` + TLD |
| `user_id` | `550e8400-...` | UUID; empty string for pre-authentication events |

### Event Vocabulary

| `event_type` | Trigger |
|-------------|---------|
| `auth.login.success` | Credentials validated, tokens issued |
| `auth.login.failure` | Bad password or unknown email |
| `auth.login.locked` | Email is currently locked out (10 failures) |
| `auth.logout` | Explicit logout; JTI added to blocklist |
| `auth.token.refresh` | Refresh token accepted; new access token issued |
| `auth.password.reset_requested` | `POST /auth/forgot-password` called for a known email |
| `auth.password.reset_completed` | `POST /auth/reset-password` completed successfully |

> **Enumeration protection:** `ForgotPassword` always returns HTTP 200 regardless of whether the
> email exists. Audit events are emitted only for known emails, so an attacker cannot use the
> audit log to determine valid addresses.

---

## Password Reset Flow

1. **Request** — `POST /api/v1/auth/forgot-password` with `{"email":"..."}` always returns 200.
2. **Token generation** — a cryptographically random token is generated and stored in Redis:
   - Key: `auth:reset:{sha256(token)}`
   - TTL: 1 hour
   - Value: the user's UUID (plaintext, scoped to Redis)
3. **Delivery** — the reset URL is sent via the configured `EmailSender`. In non-production
   environments `StubEmailSender` writes the URL to the structured log.
4. **Consumption** — `POST /api/v1/auth/reset-password` with `{"token":"...","new_password":"..."}`:
   - The token is deleted from Redis in the same pipeline as the GET (single-use guarantee)
   - If the caller crashes after deletion but before updating the password, a new token must be requested
   - The new password must be at least 8 characters; bcrypt rehash happens in `UserService.UpdatePassword`

---

## Known Limitations

| Limitation | Status | Mitigation |
|------------|--------|-----------|
| No MFA / TOTP | Planned | Strong password policy + account lockout |
| No CORS allowed-origins list per-route | Planned | Global CORS middleware; wildcard `*` rejected by linter |
| `StubEmailSender` in non-prod | By design | Replace with `smtp.EmailSender` before production |
| No self-service account unlock | Planned | Lockout expires automatically after 15 minutes |
| Refresh token family invalidation (detect theft) | Planned | Per-session blocklist; compromise requires Redis access |

---

## Dependency Security

```bash
# Scan Go module graph for known CVEs
govulncheck ./...

# Scan container image layers
trivy image --severity HIGH,CRITICAL ghcr.io/nowi5/kleido:latest
```

Both tools run in CI on every push. All HIGH/CRITICAL findings are blocking.

---

## CI Security Checks

| Check | Tool | Blocking |
|-------|------|---------|
| Static security analysis | `gosec` (via golangci-lint) | Yes |
| Dependency vulnerabilities | `govulncheck` | Yes (HIGH+) |
| Container image scanning | `trivy` (SARIF → GitHub Security tab) | Yes (HIGH+) |
| Secret scanning | GitHub secret scanning | Yes |
| SAST | `staticcheck` (via golangci-lint) | Yes |

---

## Incident Response

### Suspected token compromise

1. Identify the JTI from the access token (`jti` claim).
2. Add the JTI to the Redis blocklist manually: `SET auth:blocklist:{jti} 1 EX <remaining_lifetime_seconds>`.
3. Delete the refresh token: `DEL auth:session:{session_id}`.
4. Rotate the RSA keypair and restart the API — all existing access tokens become invalid.

### Suspected password database leak

1. Force-reset all passwords: set `is_active = false` for all users, issue a mass "reset your password" email.
2. Flush all Redis session data: `FLUSHDB` on the sessions Redis database.
3. Rotate the RSA keypair.
4. Notify affected users per GDPR Article 33 within 72 hours.

### Account enumeration attempt detected

Watch for spikes in `auth.login.failure` events in the structured log, or for `auth.login.locked`
events across many different email addresses in a short window. Block the offending IP at the
load-balancer level and review the rate-limit thresholds.
