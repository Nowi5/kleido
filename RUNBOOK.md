# Runbook

Operational playbooks for the kleido API service.

> **Conventions used below**
> - `$POD` — the running container / process name (e.g. `kleido-api-1` in Docker Compose)
> - `$REDIS` — the Redis container name (e.g. `kleido-redis-1`)
> - `$DB` — the Postgres container name (e.g. `kleido-postgres-1`)
> - All Redis commands assume `redis-cli` inside `$REDIS`; all psql commands assume `psql` inside `$DB`

---

## 1. API returning 500s

**Symptoms:** `kleido_http_requests_total{status_class="5xx"}` is non-zero; error logs visible.

**Steps:**

1. Tail the structured log for `level=ERROR` entries:
   ```bash
   docker compose logs --tail=200 -f api | grep '"level":"ERROR"'
   ```
2. Check for database errors — look for `"db_error"` or `"pgx"` in the log fields.
3. Check for Redis errors — look for `"redis"` in the log fields.
4. Check the readiness probe:
   ```bash
   curl http://localhost:8080/readyz
   # {"status":"ready","db":true,"redis":true}  ← healthy
   # {"status":"unavailable","db":false,...}     ← DB down
   ```
5. If a panic occurred, look for `"panic recovered"` in the log — the panic middleware catches
   and logs the stack trace.
6. Restart the API container if logs show no useful detail and errors persist:
   ```bash
   docker compose restart api
   ```

**Escalate if:** errors persist after restart or if DB/Redis is healthy but 500s continue —
this indicates an application bug requiring a code fix.

---

## 2. Login endpoint returning 429 (brute-force lockout)

**Symptoms:** Legitimate users report being locked out. `auth.login.locked` events in the log.

**Steps:**

1. Identify the locked email from the audit log:
   ```bash
   docker compose logs api | grep 'auth.login.locked' | tail -20
   ```
   The log entry contains `email_masked` (e.g. `u***@e***.com`) and `ip`.

2. Compute the SHA-256 hash of the email to find the Redis key:
   ```bash
   echo -n "user@example.com" | sha256sum
   # outputs: abc123...  (64-char hex)
   ```

3. Check the current failure count:
   ```bash
   docker compose exec redis redis-cli GET "auth:lockout:abc123..."
   # → "10"  (at or above threshold)
   ```

4. To manually unlock (clear the counter):
   ```bash
   docker compose exec redis redis-cli DEL "auth:lockout:abc123..."
   ```

5. Check remaining TTL before deciding to clear:
   ```bash
   docker compose exec redis redis-cli TTL "auth:lockout:abc123..."
   # → 347  (seconds remaining; lockout expires automatically)
   ```

**Notes:**
- Lockout expires automatically after 15 minutes — if the TTL is low, waiting may be the
  right call rather than manual intervention.
- If the same IP is triggering many different email lockouts, block the IP at the load balancer.
- Threshold is 10 failures; configured in `internal/repository/redis/session.go`.

---

## 3. Database connection lost

**Symptoms:** `readyz` returns `{"db":false}`; log shows pgx connection errors.

**Steps:**

1. Check if Postgres is running:
   ```bash
   docker compose ps postgres
   ```

2. Check Postgres logs:
   ```bash
   docker compose logs --tail=50 postgres
   ```

3. Verify connectivity from the API container:
   ```bash
   docker compose exec api nc -zv postgres 5432
   ```

4. If Postgres is down, restart it:
   ```bash
   docker compose restart postgres
   ```

5. The API uses `pgxpool` with automatic reconnection — once Postgres is back, the pool
   recovers without an API restart. Confirm via `readyz`:
   ```bash
   curl http://localhost:8080/readyz
   ```

6. If Postgres is up but the API still shows `db:false`, check the `DATABASE_URL` env var
   and confirm the password/host have not changed.

7. If the connection pool is exhausted (no `FATAL` from Postgres, but queries time out),
   check `DATABASE_MAX_CONNS` — default is 25. Increase if traffic justifies it.

**Data integrity:** No action required for transient connection loss — pgx retries are
transparent to callers. For a prolonged outage, review WAL logs for any missed writes.

---

## 4. Redis connection lost

**Symptoms:** `readyz` returns `{"redis":false}`; log shows `go-redis` connection errors;
rate limiting and session operations degrade.

**Steps:**

1. Check if Redis is running:
   ```bash
   docker compose ps redis
   ```

2. Check Redis logs:
   ```bash
   docker compose logs --tail=50 redis
   ```

3. Verify connectivity:
   ```bash
   docker compose exec api nc -zv redis 6379
   ```

4. Restart Redis if down:
   ```bash
   docker compose restart redis
   ```

5. **Impact of Redis loss:**
   - JWT blocklist: tokens that were revoked while Redis was down become temporarily valid
     until their natural expiry (max 15 minutes). This is an accepted trade-off — access
     tokens are short-lived.
   - Rate limiting: the middleware fails open — requests are **not** blocked while Redis
     is unreachable.
   - Brute-force lockout: also fails open — the login endpoint allows all attempts.
   - Password reset tokens: any reset tokens issued during the outage are lost. Users
     must re-request a reset link.

6. After Redis recovers, the API reconnects automatically. Verify via `readyz`.

**Post-recovery:** If Redis data was lost (e.g. container replaced without a persistent
volume), all active refresh tokens are invalidated. Users will need to log in again.
This is expected behaviour.

---

## 5. High latency / slow responses

**Symptoms:** `kleido_http_request_duration_seconds` p99 rises above 500 ms; users report timeouts.

**Steps:**

1. Check the Grafana "API Overview" dashboard — identify which route patterns are slow.

2. Check DB query latency panel:
   - `kleido_db_query_duration_seconds` p99 > 50 ms → DB bottleneck
   - `kleido_db_query_duration_seconds` p99 < 5 ms → not DB; check app logic or Redis

3. Check Redis latency:
   ```bash
   docker compose exec redis redis-cli --latency -i 1
   ```

4. Check Postgres for long-running queries:
   ```bash
   docker compose exec postgres psql -U postgres -d kleido_db -c \
     "SELECT pid, now()-query_start AS duration, state, query
      FROM pg_stat_activity
      WHERE state != 'idle' AND query_start < now() - interval '5 seconds'
      ORDER BY duration DESC LIMIT 10;"
   ```

5. Check pool exhaustion — if `kleido_http_requests_inflight` is consistently at max and
   latency is high, increase `DATABASE_MAX_CONNS` or `REDIS_POOL_SIZE`.

6. Check Jaeger for the slow trace — find the request in Jaeger UI, identify which span is
   taking the time (HTTP → handler → service → repository).

7. If all looks healthy but latency is high, check for GC pressure:
   ```bash
   curl -s http://localhost:8080/debug/pprof/heap > heap.out
   go tool pprof heap.out
   ```
   (Note: pprof endpoint must be explicitly enabled — it is not on by default in this API.)

---

## 6. Metrics not appearing in Prometheus

**Symptoms:** Grafana panels show "No Data"; Prometheus targets page shows the API as DOWN.

**Steps:**

1. Confirm the `/metrics` endpoint is reachable:
   ```bash
   curl http://localhost:8080/metrics | head -20
   ```

2. Check Prometheus targets at `http://localhost:9090/targets` — find `kleido-api` and
   check the `Last Error` field.

3. Common causes:
   - **Wrong scrape address:** Prometheus config uses `api:8080` (Docker network name).
     If running locally without Docker, update `config/prometheus.yml` to `localhost:8080`.
   - **Prometheus container not on the same network:** Verify `docker compose ps` shows
     both `prometheus` and `api` in the `kleido` network.
   - **API not yet started:** Prometheus marks the target as DOWN if the first scrape fails.
     Start the API first, then restart Prometheus.

4. Reload Prometheus config without restart:
   ```bash
   curl -X POST http://localhost:9090/-/reload
   ```

---

## 7. Grafana shows "No Data"

**Symptoms:** Grafana dashboard panels are blank or show "No data" after Docker Compose start.

**Steps:**

1. Confirm Prometheus is healthy (see Playbook 6 above).

2. Check the Grafana datasource — open Grafana → Connections → Data sources → Prometheus →
   click **Test**. It should show "Data source is working".

3. If the datasource URL is wrong (`http://prometheus:9090` is the correct value for Docker
   Compose), update it and save.

4. Check that the dashboard is provisioned:
   - Open Grafana → Dashboards → look for **MyApp — API Overview**
   - If missing, restart Grafana: `docker compose restart grafana`
   - The dashboard JSON is auto-provisioned from `docker/grafana/dashboards/`

5. If panels show data for some metrics but not others, the missing metric has not been
   emitted yet — make a few API requests to generate traffic, then refresh the dashboard.

6. Time range: ensure the Grafana time picker is set to **Last 5 minutes** or **Last 15 minutes**
   — newly started services have no historical data.
