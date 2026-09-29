# Skewed scrape allocation baseline

Run three Grafana Alloy peers in the existing kind example. Each peer discovers
and relabels every demo target, and `prometheus.scrape` uses standard Alloy
clustering to choose ownership. Metrics go to your Grafana Cloud stack.

You need Docker, kind, kubectl, Helm, Task, Python 3.9 or later, and access to a
Grafana Cloud Prometheus remote-write endpoint.

## Start the baseline

Add these values to `example/kind/.env.credentials`, which is gitignored:

```dotenv
GRAFANA_CLOUD_PROMETHEUS_URL=<REMOTE_WRITE_URL>
GRAFANA_CLOUD_PROMETHEUS_USERNAME=<METRICS_TENANT_ID>
GRAFANA_CLOUD_METRICS_WRITE_TOKEN=<METRICS_WRITE_TOKEN>
```

Use a token with `metrics:write` scoped to your stack. These values are transferred
to a Kubernetes Secret through stdin. Rendered manifests contain Secret references,
not credentials. Keep the credentials file readable only by your user (`chmod 600`).

Run from `example/kind`:

```sh
task target-allocation:render
task target-allocation:up
task target-allocation:status
```

The tasks reuse `cluster:create`, the local Alloy Helm chart, and the integration
tests' `make prom-gen-image` fixture. They create a `target-allocation` namespace
in a single-node `alloy-example` cluster and leave it running. Three Alloy pods
share one node: this demonstrates allocation, not machine-level resilience. The
start task refuses to run while another kind cluster is active; stop that cluster
first. All kubectl and Helm operations select the demo kubeconfig and context.
Existing clusters retain their topology; recreate the demo cluster to change it.

## Change the workload

Edit [settings.json](settings.json), then rerun `task target-allocation:up`.
The default is 2,640 workload series, or 88 samples/second at a 30-second interval,
plus scrape and Alloy monitoring metrics.

An even split is feasible at 880 workload series per peer: one large target and
eight small targets. Compare that reference point with the observed hash allocation. The current demo
selects a placement with all three large targets on one peer, giving two peers
below 500 series and one above 2,000. This is a deliberate skew scenario, not a
claim about typical hashing results. Recreating target pods changes their IPs
and can change ownership; the settings alone do not guarantee this placement.
The recorded run required recreating `targets-large-0` once after deployment.

| Setting | Default | Meaning |
| --- | --- | --- |
| `alloy_replicas` | `3` | Number of clustered scrapers. |
| `large_targets` | `3` | Number of large target pods. |
| `large_series` | `800` | Series per large target. |
| `small_targets` | `24` | Number of small target pods. |
| `small_series` | `10` | Series per small target. |
| `scrape_interval_seconds` | `30` | Interval for workload and self-scrapes. |
| `run_id` | `baseline-onehot` | Label identifying the measurement run. |
| `alloy_image` | `grafana/alloy:v1.20.1` | Pinned Alloy release. |

You can use a separate profile without editing the defaults:

```sh
cp config/target-allocation/settings.json build/my-settings.json
# Edit build/my-settings.json.
task target-allocation:up SETTINGS=build/my-settings.json
```

Each synthetic target emits exactly its configured number of stable gauge series.
The generator's default mode remains unchanged for integration tests.
StatefulSet pod names provide stable target identities; pod IP changes can still
change the hashing input after restarts. Record the measured ownership distribution.

Changing settings rolls affected pods, and reapplying also restarts Alloy to pick
up credential changes. Allow at least five minutes of warm-up after all three
peers have joined. Peers can briefly scrape overlapping target sets during startup.
Alloy suppresses stale markers on ownership handoff, so old scraper-labelled
series can remain visible until the backend query lookback expires.
Changing `run_id` or target ownership creates new Cloud series; historical series
can remain counted in the Cloud active-series window after a change.

## Measure the baseline

Use the Prometheus datasource in Grafana Explore, or the configured gcx context:

```sh
gcx --context thampiotr metrics query 'sum by (scraper) (scrape_samples_scraped{experiment="target-allocation",run_id="baseline-onehot",job="target-allocation"})'
```

Save these queries with the measurement window and rendered settings from
`example/kind/build/target-allocation/settings.json`:

| Measurement | PromQL |
| --- | --- |
| Samples per scrape per peer | `sum by (scraper) (scrape_samples_scraped{experiment="target-allocation",run_id="baseline-onehot",job="target-allocation"}) or on (scraper) (0 * max by (scraper) (up{experiment="target-allocation",run_id="baseline-onehot",job="alloy-self"}))` |
| Healthy targets per peer | `sum by (scraper) (up{experiment="target-allocation",run_id="baseline-onehot",job="target-allocation"})` |
| Unhealthy targets | `sum(up{experiment="target-allocation",run_id="baseline-onehot",job="target-allocation"} == bool 0)` |
| Scraper count per target (expect 1) | `count by (instance) (up{experiment="target-allocation",run_id="baseline-onehot",job="target-allocation"})` |
| Process memory | `process_resident_memory_bytes{experiment="target-allocation",run_id="baseline-onehot",job="alloy-self"}` |
| CPU cores | `rate(process_cpu_seconds_total{experiment="target-allocation",run_id="baseline-onehot",job="alloy-self"}[5m])` |
| Remote-write failures | `sum by (scraper) (rate(prometheus_remote_storage_samples_failed_total{experiment="target-allocation",run_id="baseline-onehot",job="alloy-self"}[5m]))` |

For imbalance, divide `max(LOAD)` by `avg(LOAD)`, substituting the full first query
for `LOAD` in both places. Its zero-fill includes idle scrapers in the mean.
Only compare windows where every self-scrape is healthy and the expected number
of target series is present. Samples per scrape are a size proxy, not a direct
estimate of memory cost. Total process resource usage does not isolate discovery cost.

Inspect each peer's discovered and owned targets through the Alloy UI:

```sh
kubectl --kubeconfig build/kubeconfig.yaml --context kind-alloy-example \
  -n target-allocation port-forward pod/alloy-target-allocation-0 12345:12345
```

Open `http://localhost:12345` and inspect `discovery.relabel.demo` and
`prometheus.scrape.demo`. Repeat with peers `-1` and `-2`; all should see 27
discovered targets, with disjoint owned subsets in steady state.

The `scraper` label is added by remote write, after ownership selection. It lets
you attribute load but also changes metric identity when a target moves. Account
for this when interpreting later handoff and staleness experiments.

## Stop the baseline

Stop demo ingestion and remove its resources while retaining kind:

```sh
task target-allocation:down
```

Use `task cluster:delete` to remove the whole example cluster, including any other
examples deployed there. The demo WAL uses the chart's ephemeral storage; this is
a local experiment, not a durable production deployment.

## Use vanilla Target Allocator

Once the baseline is running, switch in the same cluster:

```sh
task target-allocation:switch MODE=ta
task target-allocation:verify
```

The switch installs one upstream Target Allocator (TA) Deployment, an internal
Service, its configuration, and namespace-scoped pod-watch RBAC. No operator or
CRDs are installed. The image is pinned to the multi-platform digest of upstream
0.160.0. Three Alloy pods identify themselves to TA using their pod names.

TA performs Kubernetes discovery and filters ready targets with a metrics port.
Each Alloy fetches only its assigned subset using `discovery.http` every 10 seconds.
Alloy clustering is disabled, so targets are not sharded twice. The Alloy service
account has no pod-watch permissions and its API token is not mounted.

Vanilla TA returns original discovery labels even after using relabel rules for
filtering and hashing. Alloy therefore maps `instance` and `size_class` on its
assigned subset. TA's `/scrape_configs` API is not imported: this example has one
explicitly configured scrape job, with the same interval in TA and Alloy.

Switching updates only allocator and scraper resources. It does not rebuild
images, create another cluster, or change target pods. The switch rejects a profile
whose target counts or series sizes differ from the running workload. Use `up`
explicitly to resize. HTTP polling and scraping intervals are independent.

Try TA's other vanilla strategy on exactly the same targets:

```sh
task target-allocation:switch MODE=ta STRATEGY=least-weighted
```

Rollback restores Alloy Kubernetes discovery, clustering, token mounting and RBAC,
then removes TA resources after Alloy is ready:

```sh
task target-allocation:switch MODE=alloy
```

The switch task assigns run IDs `ta-consistent`, `ta-least-weighted`, or
`baseline-onehot`. Substitute that run ID into the earlier queries. A mode switch
rolls scraper pods and can produce temporary gaps or duplicate owners; wait for
convergence and at least five minutes of query lookback before comparing. A changed
allocation does not imply series-aware balancing: both vanilla TA strategies are
size-blind. Do not recreate targets to force TA to reproduce the selected baseline
skew.

For a custom run ID, invoke the Python entry point directly:

```sh
python3 config/target-allocation/demo.py switch --mode ta --run-id ta-repeat
```

The default `settings.json` still selects the Alloy baseline. `switch` overrides it
for that invocation and writes the effective settings to
`build/target-allocation/settings.json`; it does not modify the input profile.
A later `up` uses the input profile and may switch back. Set `discovery_mode: ta`
and an appropriate `run_id` in your profile for a fresh TA-mode deployment.

| Setting | Purpose |
| --- | --- |
| `discovery_mode` | `alloy` or `ta` for `up` and `render`. |
| `ta_image` | Upstream image pinned by digest, never built locally. |
| `ta_strategy` | `consistent-hashing` or `least-weighted`. |
| `http_refresh_interval_seconds` | Target refresh interval; default 10. |
| `alloy_resources`, `ta_resources` | Kubernetes resource requests/limits. |

### Verify and monitor TA mode

`task target-allocation:verify` compares TA HTTP assignments, Alloy discovery
exports and healthy scrape targets. It checks unique ownership and label identity,
and rejects a mounted Alloy API token. It reads effective rendered settings.
A failure during rollout can be transient; rerun after pods and discovery settle.

The TA Service exposes `/jobs`, `/jobs/target-allocation/targets?collector_id=<pod>`,
`/metrics`, `/livez`, and `/readyz`. An unknown collector ID returns HTTP 200 with
`[]`, so HTTP status alone does not prove correct collector registration.

Peer 0 scrapes TA's telemetry once under `job="target-allocator"`; workload queries
continue to select `job="target-allocation"`. Monitoring peer 0 is deliberately
simple and not highly available. The self-scrape allowlist includes HTTP discovery
failures and refresh metrics. Useful additional queries:

```promql
sum by (scraper) (prometheus_sd_http_failures_total{experiment="target-allocation",run_id="ta-consistent"})
opentelemetry_allocator_collectors_allocatable{experiment="target-allocation",run_id="ta-consistent"}
opentelemetry_allocator_targets{experiment="target-allocation",run_id="ta-consistent"}
sum(process_resident_memory_bytes{experiment="target-allocation",run_id="ta-consistent",job=~"alloy-self|target-allocator"})
sum(rate(process_cpu_seconds_total{experiment="target-allocation",run_id="ta-consistent",job=~"alloy-self|target-allocator"}[5m]))
```

Include TA's CPU and memory in comparisons. Removing Alloy pod watches demonstrates
centralized discovery; total process memory does not isolate discovery savings.
At this small workload, allocator overhead may exceed resource savings.

A failed HTTP refresh retains the previous discovery targets. A successful empty
list clears that collector's assignment; these are different failure modes. TA is
one process with no HA in this demo. A TA restart can temporarily change assignments
while collector discovery recovers. The switch to an external allocator also uses
normal scrape staleness behavior; do not assume Alloy clustering's handoff handling
applies to HTTP-discovered target removal.
