# Grafana — Dashboards

Grafana is available at:

**[http://localhost:3001](http://localhost:3001)**

| Field | Value |
|-------|-------|
| Username | `admin` |
| Password | `admin` |

The **MyApp — API Overview** dashboard is auto-provisioned on startup — no manual import is needed.

---

## Opening the dashboard

1. Open [http://localhost:3001](http://localhost:3001) and log in
2. Click the **Dashboards** icon in the left sidebar (grid of squares)
3. Select **MyApp — API Overview**
4. The dashboard auto-refreshes every **10 seconds**
5. Default time range: **Last 1 hour** (adjustable top-right)

Generate some traffic to see data populate:

```bash
# A few requests to seed the metrics
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl -X POST http://localhost:8080/api/v1/auth/register \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com","password":"Secret123!"}'
curl -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com","password":"Secret123!"}'
```

---

## Dashboard panels

### Row 1 — Top-level KPIs (stat panels)

#### Requests / sec

- **Query:** `sum(rate(kleido_http_requests_total[1m]))`
- **Unit:** requests/second
- **Colour:** green (any positive value is healthy)
- **Use:** quick sanity check that traffic is flowing

#### Error rate

- **Query:** `sum(rate(...{status_class=~"4xx|5xx"}[1m])) / sum(rate(...[1m]))`
- **Unit:** percentage
- **Thresholds:** green → yellow ≥ 1% → red ≥ 5%
- **Use:** first signal of a degradation or bad deploy

#### Inflight requests

- **Query:** `kleido_http_requests_inflight`
- **Unit:** count (integer gauge)
- **Use:** spike here with no matching spike in req/s → slow handler or blocked goroutines

#### Cache hit rate

- **Query:** `sum(rate(...{result="hit"}[1m])) / sum(rate(...{operation="get"}[1m]))`
- **Unit:** percentage
- **Thresholds:** red < 50% → yellow < 80% → green ≥ 80%
- **Use:** dropping hit rate means Redis is cold or cache TTLs are too short

---

### Row 2 — Time-series panels

#### Request rate by path

- **Query:** `sum by (path) (rate(kleido_http_requests_total[1m]))`
- **Display:** stacked area lines, one line per route pattern
- **Use:** see which endpoints drive the most load; detect traffic shifts after a deploy

#### Request latency p50 / p95 / p99

- **Queries (three series):**

    ```promql
    # p50
    histogram_quantile(0.50,
      sum by (le, path) (rate(kleido_http_request_duration_seconds_bucket[5m])))

    # p95
    histogram_quantile(0.95, ...)

    # p99
    histogram_quantile(0.99, ...)
    ```

- **Unit:** seconds
- **Use:** p50 is your "typical" user experience; p99 is your worst 1% — gaps between them indicate outliers worth investigating in Jaeger

---

### Row 3 — Infrastructure panels

#### DB query duration p99 by operation

- **Query:** `histogram_quantile(0.99, sum by (le, operation) (rate(kleido_db_query_duration_seconds_bucket[5m])))`
- **Unit:** seconds
- **Thresholds:** yellow > 50 ms, red > 100 ms
- **Operations tracked:** `find_by_id`, `find_by_email`, `list`, `create`, `update`, `delete`
- **Use:** identify which query type is slow; cross-reference with Jaeger's `repository/*` spans to see the exact SQL

#### Panics (5-minute window)

- **Query:** `increase(kleido_http_panics_total[5m])`
- **Unit:** count
- **Thresholds:** green = 0, red ≥ 1
- **Use:** any value above zero is a critical signal — open API logs immediately for the stack trace

---

## Exploring data beyond the dashboard

### Ad-hoc PromQL (Explore mode)

1. Click the **Explore** icon (compass) in the left sidebar
2. Select **Prometheus** as the data source
3. Type any PromQL query — autocomplete will suggest metric names
4. Toggle between **Graph** and **Table** views

### Time range shortcuts

| Shortcut | Range |
|----------|-------|
| `Last 5 minutes` | Good during active testing |
| `Last 1 hour` | Default — covers a typical dev session |
| `Last 6 hours` | Spot trends across longer runs |
| Custom | Click the time picker → type exact from/to |

### Linking Grafana → Jaeger

Grafana can display Jaeger traces inline when a **Jaeger datasource** is configured. To add it:

1. Go to **Connections → Data sources → Add data source**
2. Choose **Jaeger**
3. Set URL to `http://jaeger:16686`
4. Click **Save & test**

Once connected, you can open a trace directly from a Grafana panel by clicking a data point and selecting **Open in Jaeger**.

---

## Provisioning explained

The dashboard and datasource are provisioned automatically at startup — no manual setup required. The files are:

| File | Purpose |
|------|---------|
| `docker/grafana/provisioning/datasources/prometheus.yaml` | Registers Prometheus as the default datasource |
| `docker/grafana/provisioning/dashboards/dashboard.yaml` | Tells Grafana where to find dashboard JSON files |
| `docker/grafana/dashboards/kleido.json` | The MyApp — API Overview dashboard definition |

Changes to `kleido.json` are picked up every 30 seconds without restarting Grafana.

---

## Modifying or adding dashboards

### Option A — Edit in the UI then export

1. Open the dashboard → click the **Edit** (pencil) button in the top bar
2. Make your changes
3. Click **Dashboard settings** (gear icon) → **JSON Model** → copy the JSON
4. Paste it into `docker/grafana/dashboards/kleido.json`
5. Commit the file — the dashboard is now version-controlled

### Option B — Add a new dashboard

1. In Grafana: **Dashboards → New → New Dashboard**
2. Design your panels using PromQL queries from the [Prometheus page](prometheus.md)
3. Export as JSON and save to `docker/grafana/dashboards/<name>.json`

The provisioning config (`dashboard.yaml`) scans the entire `dashboards/` directory, so any `.json` file is picked up automatically.

---

## Resetting Grafana

If you need a clean Grafana state (e.g. to test provisioning from scratch):

```bash
docker-compose down
docker volume rm kleido_grafana_data
docker-compose up -d
```

All manual UI changes are lost; provisioned dashboards and datasources are restored automatically.
