# Prometheus — Metrics

Prometheus is available at:

**[http://localhost:9090](http://localhost:9090)**

It scrapes the API's `/metrics` endpoint every **15 seconds** and stores time-series data that Grafana queries for dashboards.

---

## Metric inventory

All metrics are prefixed with `kleido_`.

### HTTP metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kleido_http_requests_total` | Counter | `method`, `path`, `status_class` | Total HTTP requests completed |
| `kleido_http_request_duration_seconds` | Histogram | `method`, `path` | Request latency (seconds) |
| `kleido_http_requests_inflight` | Gauge | — | Requests currently being processed |
| `kleido_http_panics_total` | Counter | — | Panics caught and recovered by middleware |

**Label values:**

- `method` — `GET`, `POST`, `PUT`, `DELETE`
- `path` — normalised chi route pattern (e.g. `/api/v1/users/{id}`, not the raw URL)
- `status_class` — `2xx`, `4xx`, `5xx`

**Histogram buckets for `kleido_http_request_duration_seconds`:**

`.005s · .01s · .025s · .05s · .1s · .25s · .5s · 1s · 2.5s · 5s`

---

### Database metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kleido_db_query_duration_seconds` | Histogram | `operation` | Time to execute each DB query |
| `kleido_db_errors_total` | Counter | `operation` | DB errors (NotFound results excluded) |

**`operation` label values:** `find_by_id` · `find_by_email` · `list` · `create` · `update` · `delete`

**Histogram buckets for `kleido_db_query_duration_seconds`:**

`.001s · .005s · .01s · .025s · .05s · .1s · .25s · .5s · 1s`

---

### Cache metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kleido_cache_operations_total` | Counter | `operation`, `result` | Redis cache operation counts |

**Label values:**

- `operation` — `get`, `set`, `delete`
- `result` — `hit`, `miss`, `ok`, `error`

---

## Using the expression browser

Open [http://localhost:9090](http://localhost:9090) and click **Graph** (default view).

### Useful queries to start with

#### Current request rate (req/s)

```promql
sum(rate(kleido_http_requests_total[1m]))
```

#### Request rate broken down by route

```promql
sum by (path) (rate(kleido_http_requests_total[1m]))
```

#### Error rate (4xx + 5xx as fraction of total)

```promql
sum(rate(kleido_http_requests_total{status_class=~"4xx|5xx"}[1m]))
/
sum(rate(kleido_http_requests_total[1m]))
```

Multiply by 100 for a percentage value.

#### Latency percentiles (p50 / p95 / p99)

```promql
histogram_quantile(0.99,
  sum by (le, path) (
    rate(kleido_http_request_duration_seconds_bucket[5m])
  )
)
```

Change `0.99` to `0.95` or `0.50` for other percentiles.

#### Database query p99 latency by operation

```promql
histogram_quantile(0.99,
  sum by (le, operation) (
    rate(kleido_db_query_duration_seconds_bucket[5m])
  )
)
```

#### Cache hit rate

```promql
sum(rate(kleido_cache_operations_total{result="hit"}[1m]))
/
sum(rate(kleido_cache_operations_total{operation="get"}[1m]))
```

Returns a ratio 0–1. Multiply by 100 for a percentage.

#### Inflight requests

```promql
kleido_http_requests_inflight
```

#### Panics in the last 5 minutes

```promql
increase(kleido_http_panics_total[5m])
```

---

## Scrape config

Prometheus is configured via `config/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: kleido-api
    static_configs:
      - targets: ['api:8080']
    metrics_path: /metrics
```

The `api` hostname resolves to the API container inside the Docker network. Data is retained for **7 days**.

---

## Checking scrape health

Go to **[http://localhost:9090/targets](http://localhost:9090/targets)** to confirm the `kleido-api` target shows:

- **State:** UP
- **Last scrape:** a few seconds ago
- **Scrape duration:** < 1ms (instant for in-process metrics)

If the target shows **DOWN**, confirm the API container is healthy:

```bash
curl http://localhost:8080/metrics | head -5
```

---

## Raw metrics endpoint

The `/metrics` endpoint on the API exposes the Prometheus text format directly:

```bash
curl http://localhost:8080/metrics | grep kleido_
```

Example output:

```
# HELP kleido_http_requests_total Total HTTP requests completed.
# TYPE kleido_http_requests_total counter
kleido_http_requests_total{method="GET",path="/api/v1/users/me",status_class="2xx"} 14
kleido_http_requests_total{method="POST",path="/api/v1/auth/login",status_class="2xx"} 3
kleido_http_requests_total{method="POST",path="/api/v1/auth/login",status_class="4xx"} 1
# HELP kleido_http_requests_inflight Requests currently being processed.
# TYPE kleido_http_requests_inflight gauge
kleido_http_requests_inflight 0
```

---

## Alerting (reference)

Prometheus alerting rules are not pre-configured in this project, but here are production-ready starting points:

```yaml
groups:
  - name: kleido
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(kleido_http_requests_total{status_class=~"5xx"}[5m]))
          / sum(rate(kleido_http_requests_total[5m])) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Error rate above 5%"

      - alert: HighP99Latency
        expr: |
          histogram_quantile(0.99,
            sum by (le) (rate(kleido_http_request_duration_seconds_bucket[5m]))
          ) > 1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "p99 latency above 1 second"

      - alert: PanicDetected
        expr: increase(kleido_http_panics_total[5m]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "Panic recovered in API"
```

Place these in a file (e.g. `config/alerts.yml`) and reference it from `prometheus.yml` under `rule_files:`.
