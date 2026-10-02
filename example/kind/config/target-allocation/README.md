# Scrape allocation demo

From `example/kind`, with Docker, kind, kubectl, Helm, Task, jq and Go installed (GCX for dashboard publishing/annotations):

```sh
task cluster:up
task deploy:vanilla      # Alloy clustering; every peer discovers targets
task deploy:ta-hashing   # Local TA; Alloy uses HTTP discovery
task deploy:ta-packing   # Local TA sticky allocation; asynchronous size probes
task ui:ta               # Print the allocation UI URL and forward localhost:8080 (PORT=8081 to override)
task demo:prepare        # Build/load once before recording
task demo:mode MODE=ta-hashing # Reuse images; ta-hashing ↔ ta-packing keeps Alloy pods
task demo:alloys REPLICAS=4    # Use 3 to scale back down
task demo:targets:extra       # Add eight mixed targets
task demo:targets:reset       # Remove the extra batch
task demo:reset               # Restore three Alloys and base targets
task demo:dashboard           # Publish the checked-in Grafana dashboard
task demo:profiles            # Capture local Alloy heap + 10s CPU profiles
task cluster:down             # Deletes the cluster and stops ingestion
```

Set `GRAFANA_CLOUD_PROMETHEUS_URL`, `GRAFANA_CLOUD_PROMETHEUS_USERNAME` and
`GRAFANA_CLOUD_METRICS_WRITE_TOKEN` in gitignored `example/kind/.env.credentials`.
TA builds from `~/workspace/opentelemetry-operator`; override with `OPERATOR_DIR`.
The load-shedding milestone is saved in Alloy commit `14b6b6141` and operator
commit `56242506`; `ta-packing` now uses the experimental `sticky` strategy.
For a newer containerd cluster, override the repo-pinned kind 0.24 with a compatible
binary: `KIND=/opt/homebrew/bin/kind task deploy:ta-packing` (tested with kind 0.33).
Only one kind cluster may run. The demo uses one node, three Alloy pods and one TA.
The pool includes 83 synthetic targets (149,997 series), all three Alloy instances,
TA, both CoreDNS pods, and real kube-state-metrics (state and self-metrics ports).
Discovery selects ready pods with ports named `metrics`, `http-metrics` or
`telemetry` in `target-allocation` and `kube-system`: 91 endpoints in this cluster.
All endpoints go through the same allocation job; there are no separate self-scrapes.
KSM reads workload/node/namespace/service metadata, excluding Secrets and ConfigMaps.
Its static manifest and 192 MiB memory limit are in `common/kube-state-metrics.yaml`.
Protected/loopback-only control-plane endpoints are not included.
Try UI searches such as `payments`, `invoice-worker` or `ap-southeast-2`; search runs
on TA and returns 25 results per page, with matching text highlighted.
Synthetic sizes: 33 × 317, 17 × 347, 23 × 733, 3 × 1,571, 2 × 8,009,
2 × 12,007, and 3 × 24,011 series. The seven whales make size-blind collisions
more visible; the largest remains below mean load even with five Alloy instances.
Keep the base pool at 83 synthetic pods so five Alloys plus the extra batch fit
the single node’s 110-pod limit.
Edit `common/targets.yaml` for target counts/series; shared Helm values and metrics
are in `common/`; both TA modes share `common/allocator.yaml` and `common/ta.alloy`.
Edit `ta-packing/allocator.yaml` for sizing/probe settings (default size 100,
5s allocation cycles, six-cycle windows, four probes at a time, refreshed every 30s).
Deploying reapplies these files; unchanged target pods stay in place. Fast mode switches
reload the run label from the ConfigMap; the pod demo-mode label may retain its previous
value until a full deployment. Scaling does not restart TA. Actions annotate the dashboard.
Allow 2–3 minutes for discovery, probes, config reload and Cloud ingestion; CPU panels need 2m.
See [DEMO.md](../../../../DEMO.md) for the four-minute recording runbook and links.

| Preliminary observations (placement depends on target addresses) | Targets | Series per Alloy |
| --- | --- | --- |
| Earlier Alloy clustering | 27 | 120 / 90 / 2430 |
| Earlier TA consistent hashing | 27 | 880 / 100 / 1660 |
| Earlier TA load-shedding | 27 | 920 / 900 / 820 |
| Load-shedding milestone | 60 | 1220 / 1600 / 910 |
| Earlier Sticky | 60 | 1250 / 1240 / 1240 |
| Earlier Sticky, synthetic uneven size mix | 77 | 2211 / 1964 / 1919 |
| Earlier Sticky with real cluster exporters | 84 | ~3710 / 2902 / 3322 |
| Earlier Sticky with larger synthetic targets | 84 | ~21,885 / 21,964 / 21,866 |
| Earlier Sticky with two whales | 84 | ~21,165 / 23,453 / 22,252 |
| Vanilla with seven whales | 90 | ~46,522 / 62,844 / 46,183 |
| TA hashing with seven whales | 91 | ~35,335 / 71,160 / 49,247 |
| Sticky with seven whales | 91 | ~52,002 / 51,854 / 51,905 |

Current workload includes real exporter metrics, so series counts vary. Query `sum by (scraper) (scrape_samples_scraped{job="target-allocation",run_id="ta-packing"})` in Cloud; allow rolling restarts, size windows and scrape refreshes to settle after switching. HA is deferred; see [ALGO.md](../../../../ALGO.md).
