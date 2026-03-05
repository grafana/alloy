# Hackathon: Preemptive Telemetry (PT)

## Goal

When an alert transitions to pending, automatically increase telemetry detail (logs, metrics, profiles) for the affected workload, then revert after a TTL. This captures diagnostic evidence *before* an incident fully develops, eliminating the "wait for it to happen again" loop.

## Architecture

```
┌─────────────────┐     polls alert state      ┌─────────────────────┐
│  PT Controller   │ ◄──────────────────────── │  Grafana Cloud      │
│  localhost:8090  │                            │  Alerting API       │
│  (Go binary)    │ ──── UpdatePipeline ──────► │  Fleet Management   │
└─────────────────┘                            │  Pipeline API       │
                                               └─────────────────────┘
                                                         │
                                                 remotecfg push
                                                         │
                                                         ▼
                                               ┌─────────────────────┐
                                               │  Alloy DaemonSet    │
                                               │  (Kind cluster)     │
                                               │  runs pt_pipeline   │
                                               └─────────────────────┘
                                                         │
                                            collects from shop-* pods
                                                         │
                                    ┌────────────────────┼────────────────────┐
                                    ▼                    ▼                    ▼
                              shop-api (3)        shop-catalog (2)      shop-db (2)
                              ns: shop-api        ns: shop-catalog      ns: shop-db
                              port: 8080          port: 8080            port: 8080
```

## What's Built

### Shop Application (`example/kind/shop/`)
- Single Go binary, 4 modes: `api`, `catalog`, `db`, `loadgen`
- Each mode: HTTP server on :8080, Prometheus metrics on /metrics, pprof on /debug/pprof/
- Deployed to Kind cluster in separate namespaces: shop-api, shop-catalog, shop-db, shop-loadgen
- Load generator continuously exercises: GET /products, GET /products/{id}, POST /cart/add, POST /checkout

**Metrics exposed:**
- `http_requests_total{method, path, status}` counter
- `http_request_duration_seconds{method, path}` histogram
- `http_requests_in_flight` gauge
- `query_cache_size_bytes` gauge (cache byte size, LRU at ~100 entries normally)
- `query_cache_entries_total` gauge (cache entry count)
- `downstream_request_duration_seconds{target}` histogram (per-downstream-pod latency)
- Standard Go/process collectors (go_goroutines, go_memstats_*, process_*)

**Fault injection endpoints:**
- `POST /fault/oom` on shop-db: enables unbounded query cache (disables LRU eviction), seeds 23MiB, then ~48KB/request growth → OOM in ~15-20min
- `POST /fault/slowness` on shop-catalog: adds 8s delay to all business requests then returns 503, health/metrics/pprof continue working

### Alloy Config (`example/kind/config/shop/config.alloy`)
- Single config file, pushed to Fleet Management as `pt_pipeline` (id=682)
- Uses `declare` blocks for `debug_pipeline` and `normal_pipeline`
- Both take `namespace_regex` argument — PT controller changes this regex
- **Normal pipeline**: drops DEBUG log lines, scrapes metrics at 30s, labels `telemetry_level="normal"`
- **Debug pipeline**: keeps ALL logs, scrapes at 15s, collects Go pprof profiles via pyroscope.scrape, labels `telemetry_level="debug"`
- Four `// <-- PT` markers in the config where regex is swapped

**The one value PT controller changes:**
```
namespace_regex = "^$"     // normal: no debug namespaces
namespace_regex = "shop-db" // activated: shop-db gets debug telemetry
```

### PT Controller (`example/kind/pt-controller/`)
- Go binary running locally on :8090
- Web UI styled like Grafana (dark theme, dropdowns for alert/namespace/level/TTL)
- Polls Grafana alerting API every 10s: `GET /api/prometheus/grafana/api/v1/rules`
- When alert matches condition (pending/firing): reads config.alloy, replaces `^$` regex with target namespace, calls FM UpdatePipeline API
- On TTL expiry: reverts regex to `^$`
- On startup: resets pipeline to normal state

**State machine:** no_rule → watching → activated → cooldown → watching

### Alerts (in Grafana Cloud UI)
- **ShopDBMemoryHigh** (uid: bff0yh8wo9se8b): `max by (pod) (process_resident_memory_bytes{namespace="shop-db"}) / 67108864 > 0.7`, pending 5m
- **HTTPLatencyHigh** (uid: eff3z3z516wowc): `histogram_quantile(0.90, sum by (le, namespace) (rate(http_request_duration_seconds_bucket{namespace=~"shop-.*"}[2m]))) > 5`, pending 5m

### Helm / Kubernetes
- Kind cluster with 1 control plane + 3 workers (`task cluster:create`)
- Grafana Cloud Onboarding Helm chart deploys: Alloy DaemonSet, alloy-operator, kube-state-metrics, node-exporter, beyla
- `helm-values.yaml` sets `configMap.create: false, name: alloy-shop-config` so operator uses external ConfigMap
- Alloy connects to FM via remotecfg and receives the pt_pipeline

## Key Credentials (in .env.credentials)
- `GRAFANA_CLOUD_API_KEY` — FM read/write access (basic auth with REMOTE_CFG_USERNAME)
- `REMOTE_CFG_URL` — Fleet Management endpoint
- `REMOTE_CFG_USERNAME` — FM auth username (624262)
- `GRAFANA_SERVICE_ACCOUNT_TOKEN` — Grafana instance API (read alerts, list rules)
- `FM_PIPELINE_ID` — pipeline id in FM (682)

## Tools Available

### grafana-assistant CLI
```bash
grafana-assistant prompt "your question here"
```
Has access to the thampiotr Grafana Cloud stack. Can:
- Query Prometheus (PromQL) and Loki (LogQL)
- List datasources
- Check alert states via alerting_historian
- Query Pyroscope profiles
- Sometimes flaky with complex queries (500 errors on high-cardinality histograms)

### kubectl (Kind cluster)
```bash
KUBECONFIG=build/kubeconfig.yaml kubectl ...
```
Or use the tasks which set it automatically.

### Alloy API (via port-forward)
```bash
kubectl -n monitoring port-forward daemonset/grafana-cloud-alloy-daemon 12345:12345
curl http://localhost:12345/api/v0/web/components                    # base components
curl http://localhost:12345/api/v0/web/remotecfg/components          # FM-pushed components
curl http://localhost:12345/api/v0/web/components/{component_id}     # component detail
curl http://localhost:12345/metrics                                   # internal metrics
```

### FM Pipeline API
```bash
# List pipelines
curl -s -u "$REMOTE_CFG_USERNAME:$GRAFANA_CLOUD_API_KEY" \
  -H "Content-Type: application/json" \
  -X POST "$REMOTE_CFG_URL/pipeline.v1.PipelineService/ListPipelines" -d '{}'

# Update pipeline (must include name + matchers)
curl -s -u "$REMOTE_CFG_USERNAME:$GRAFANA_CLOUD_API_KEY" \
  -H "Content-Type: application/json" \
  -X POST "$REMOTE_CFG_URL/pipeline.v1.PipelineService/UpdatePipeline" \
  -d '{"pipeline":{"id":"682","name":"pt_pipeline","contents":"...","matchers":["workloadName=\"alloy-daemon\""],"enabled":true}}'
```

### Grafana Alerting API
```bash
# List all alert rules
curl -H "Authorization: Bearer $GRAFANA_SERVICE_ACCOUNT_TOKEN" \
  "https://thampiotr.grafana.net/api/v1/provisioning/alert-rules"

# Get alert states (runtime)
curl -H "Authorization: Bearer $GRAFANA_SERVICE_ACCOUNT_TOKEN" \
  "https://thampiotr.grafana.net/api/prometheus/grafana/api/v1/rules"
```

### PT Controller API (localhost:8090)
```bash
curl http://localhost:8090/api/status                    # controller state
curl http://localhost:8090/api/alerts                    # list Grafana alerts
curl -X POST http://localhost:8090/api/rules -d '...'   # save PT rule
curl -X POST http://localhost:8090/api/activate -d '{"namespace":"shop-db"}'
curl -X POST http://localhost:8090/api/deactivate
```

## Taskfile Commands (`cd example/kind && task ...`)
| Task | Description |
|------|-------------|
| `cluster:create` | Create Kind cluster |
| `cluster:delete` | Delete Kind cluster |
| `deploy:cloud-onboarding:local` | Helm install with shop Alloy config |
| `deploy:shop` | Build shop image, load into Kind, apply manifests |
| `deploy:alloy-config` | Validate config.alloy, update ConfigMap, restart Alloy pods |
| `build:shop` | Build shop Docker image and load into Kind |
| `remove:shop` | Delete all shop namespaces |
| `fault:oom` | Trigger memory leak on one shop-db pod |
| `fault:slowness` | Trigger slowness on one shop-catalog pod |
| `run:pt-controller` | Run PT controller locally on :8090 |
| `alloy:ui` | Port-forward to Alloy UI and open browser |

## Demo Scenarios

### Scenario 1: OOM (memory leak)
1. Configure PT rule: ShopDBMemoryHigh → pending → shop-db → debug → 15m TTL
2. `task fault:oom` — memory grows from ~15MiB toward 64MiB limit
3. ~5min: alert pending, PT activates debug for shop-db
4. Debug telemetry captures: debug logs showing `query_cache_entries` growing unbounded, heap profiles
5. ~15min: alert fires, ~20min: OOM kill
6. TTL expires: PT reverts to normal

### Scenario 2: Slowness (latency spike)
1. Configure PT rule: HTTPLatencyHigh → pending → shop-catalog → debug → 15m TTL
2. `task fault:slowness` — one catalog pod adds 8s delay to all business requests
3. API p90 latency spikes (requests to slow pod take 8s+)
4. Alert pending, PT activates debug for shop-catalog
5. Debug telemetry captures: goroutine profiles showing goroutines stuck in `time.Sleep` at slownessMiddleware
6. TTL expires: PT reverts to normal

## Known Issues / Gotchas
- The grafana-cloud-onboarding helm chart's operator overwrites configMap.content — use `configMap.create: false` + external ConfigMap
- FM pipelines need `matchers: ["workloadName=\"alloy-daemon\""]` or they won't be assigned to collectors
- Backtick raw strings in Alloy config survive FM JSON encoding; double-quoted strings with heavy escaping get mangled
- The Go HTTP client connection pool can route around slow pods via keep-alive, so the latency impact at the API level can be subtle
- `alloy validate` needs dummy env vars: `HOSTNAME=dummy CLUSTER_NAME=dummy GCLOUD_RW_API_KEY=dummy NAMESPACE=monitoring POD_NAME=dummy`

## File Locations
| File | Purpose |
|------|---------|
| `example/kind/shop/main.go` | Shop application (all 4 modes) |
| `example/kind/shop/Dockerfile` | Multi-stage build |
| `example/kind/shop/go.mod` | Go module |
| `example/kind/config/shop/config.alloy` | Alloy pipeline config (pushed to FM) |
| `example/kind/config/shop/config.old.alloy` | Backup of original FM-pushed config |
| `example/kind/config/shop/helm-values.yaml` | Helm values for cloud onboarding chart |
| `example/kind/config/shop/manifests.yaml` | K8s manifests for shop deployments/services |
| `example/kind/pt-controller/main.go` | PT controller binary |
| `example/kind/pt-controller/ui.html` | Grafana-styled web UI |
| `example/kind/pt-controller/go.mod` | Go module |
| `example/kind/Taskfile.yml` | All automation tasks |
| `example/kind/.env.credentials` | Credentials (gitignored) |
| `example/kind/.env.credentials.template` | Credential template |
