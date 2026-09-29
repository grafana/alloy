# Hackathon 18: size-aware scrape target allocation

Working plan for the demo by Friday 2 October 2026. Increase fidelity as we make
decisions together. Only milestone 1 is expanded for now; the algorithm remains open.

All Alloy configuration, kind setup, and demo assets live in this repository on
`thampiotr/hackathon-target-allocator`. Target Allocator development lives in
`~/workspace/opentelemetry-operator`, with
[thampiotr/opentelemetry-operator](https://github.com/thampiotr/opentelemetry-operator)
as the fork and the OpenTelemetry repository as `upstream`.

## 1. Build a skewed scrape example in a local kind cluster

Demonstrate uneven scrape load using standard Alloy clustering, with every Alloy
instance doing discovery and relabeling. Send metrics to Piotr's private Grafana
Cloud stack so the baseline is visible and reusable for later comparisons.

### Reuse the existing Kubernetes setup

- Extend [example/kind](example/kind/README.md) for the persistent demo. Put its
  configuration and workload manifests under `example/kind/config/target-allocation/`
  and add demo tasks to the existing Taskfile as we implement them.
- Reuse its kind lifecycle, explicit kubeconfig, and local Alloy Helm chart.
  Keep demo workloads running until an explicit teardown so Cloud observations
  and the presentation can continue after setup finishes.
- Reuse [Kubernetes integration-test](integration-tests/k8s/README.md) fixtures and
  conventions where useful. Its runner already builds and loads images, and its
  Alloy dependency installs the same local chart with a config file and values.
- The integration runner's `--reuse-cluster` keeps the cluster, but the test harness
  still removes test workloads. It is suitable for automated validation; the
  persistent demo should use the existing example tasks.
- Extend the existing
  [prom-gen fixture](integration-tests/docker/configs/prom-gen/main.go) with an
  opt-in `--series` setting. Preserve its defaults for existing tests and reuse
  `make prom-gen-image`. The synthetic mode emits a fixed number of stable gauge
  series per target.

### Proposed starting setup

- Use one single-node kind cluster and a dedicated demo namespace. Keep other
  kind clusters stopped to conserve laptop memory. Three Alloy pods share the node.
- Use focused fixture tests and configuration checks locally; avoid full-repo
  lint and test runs on this laptop.
- Deploy three Alloy replicas using the Alloy Helm chart and a StatefulSet for
  stable scraper names. Pin the chart and image versions when implementing.
- Enable both Helm's `alloy.clustering.enabled` and the `clustering` block in
  `prometheus.scrape`. Use identical discovery, relabeling, and scrape configuration
  on all replicas, with the chart providing peer discovery.
- Use the pipeline `discovery.kubernetes` → `discovery.relabel` →
  `prometheus.scrape` → `prometheus.remote_write`. All replicas discover the full
  demo target set; Alloy's consistent hashing decides which targets each scrapes.
- Use a released Alloy image for the baseline so existing local source changes
  do not affect the experiment.

This follows the repository's [clustering model](docs/sources/get-started/clustering.md)
and [Helm configuration](operations/helm/charts/alloy/values.yaml).

### Generate controllable skew

- Use the existing prom-gen fixture with configurable series counts. Keep series
  identities stable throughout the measurement window.
- Use several large targets among many small targets. A provisional workload is
  3 targets at 800 series each and 24 at 10 series each, all scraped every 30 seconds.
  The workload stays below the agreed roughly 5,000-series budget. Change the workload and
  scrape interval in [settings.json](example/kind/config/target-allocation/settings.json).
- That starting profile is 2,640 workload series and 88 samples/second, excluding
  monitoring overhead. Measure actual exposed sample counts before fixing the profile.
- Keep the workload, target identities, scrape interval, and resource settings
  fixed across comparisons. Record the actual ownership distribution; do not assume
  consistent hashing will produce the same skew after recreating pods or the cluster.
- Establish observable imbalance before freezing the workload. Include multiple
  large targets so a better allocation could help: no allocator can split one
  indivisible target across scrapers.

### Send metrics to Grafana Cloud and attribute the work

- Configure the private stack's Prometheus remote-write URL, metrics tenant ID,
  and write token through a Kubernetes Secret. Keep credentials out of tracked files.
- Label the experiment and run so its metrics can be isolated in Cloud.
- Add a scraper identity label after ownership selection, for example through
  remote-write external labels. Replica-specific labels must not change the target
  labels presented to the clustering hash.
- Preserve a stable target identity as well as the scraper identity. A target move
  will create a different Cloud series when the scraper label changes; account for
  this in later comparisons.
- Collect Alloy's own metrics from every replica using a separate, unclustered
  self-scrape. Record process CPU and memory, scrape health, and remote-write health.

### Baseline evidence

- Show target count and the sum of `scrape_samples_scraped` per Alloy replica.
  Treat samples per scrape as the initial size proxy, not an exact memory estimate.
- Calculate load imbalance as maximum per-replica load divided by mean load,
  including replicas with zero assigned targets.
- Show CPU and memory alongside load; allow a warm-up period before recording a
  fixed measurement window. Confirm successful scrapes and remote-write delivery.
- Verify all three replicas see the full discovered target set, while each target
  has one scraper in steady state. Check ownership through Alloy's debug views.
- Record discovery duplication separately from total process resource usage.
  Total Alloy CPU or memory alone cannot establish how much discovery costs;
  component metrics or profiles may be needed when quantifying savings later.

### Deliverables and completion criteria

- [x] Reproducible kind setup, target manifests, Helm values, and Alloy configuration.
- [x] Documented commands to start, stop, and repeat the baseline.
- [x] Metrics arriving in the private Grafana Cloud stack.
- [x] Saved baseline queries or a minimal dashboard showing skew and scraper health.
- [x] Recorded versions, workload, ownership, and measurement window for comparison.

The private Cloud stack is `https://thampiotr.grafana.net`, with gcx context
`thampiotr`. The ignored `example/kind/.env.credentials` holds its remote-write
settings and stack-scoped metrics-write token (expires 6 October 2026).

The [baseline runbook](example/kind/config/target-allocation/README.md) contains
setup, resizing, measurement, and teardown commands. Local baseline measurements
confirm the original 4,800-series run and the revised 2,640-series run. The revised
profile selects one heavy peer and two light peers (2,430 / 120 / 90 series).
The starting profile shows clear skew. Local resource limits remain optional tuning.

## 2. Add Target Allocator

Add Target Allocator to the example.

## 3. Customise Target Allocator to better assign targets

Work together to invent the allocation algorithm.

## 4. Make a demo

Make the demo. Stretch goal: try it in a dev cluster to demonstrate real savings.
