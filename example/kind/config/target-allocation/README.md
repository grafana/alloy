# Scrape allocation demo

From `example/kind`, with Docker, kind, kubectl, Helm, Task and Go installed:

```sh
task cluster:up
task deploy:vanilla      # Alloy clustering; every peer discovers targets
task deploy:ta-hashing   # Local TA; Alloy uses HTTP discovery
task deploy:ta-packing   # Local TA load-shedding; asynchronous size probes
task ui:ta               # Print the allocation UI URL and forward localhost:8080 (PORT=8081 to override)
task cluster:down        # Deletes the cluster and stops ingestion
```

Set `GRAFANA_CLOUD_PROMETHEUS_URL`, `GRAFANA_CLOUD_PROMETHEUS_USERNAME` and
`GRAFANA_CLOUD_METRICS_WRITE_TOKEN` in gitignored `example/kind/.env.credentials`.
TA builds from `~/workspace/opentelemetry-operator`; override with `OPERATOR_DIR`.
Milestone verified with operator commit [`74567406`](https://github.com/thampiotr/opentelemetry-operator/commit/74567406).
For a newer containerd cluster, override the repo-pinned kind 0.24 with a compatible
binary: `KIND=/opt/homebrew/bin/kind task deploy:ta-packing` (tested with kind 0.33).
Only one kind cluster may run. The demo uses one node, three Alloy pods and one TA.
The 60 targets represent inventory, checkout, catalog search and billing services.
Try UI searches such as `payments`, `invoice-worker` or `ap-southeast-2`; search runs
on TA and returns 25 results per page, with matching text highlighted.
Edit `common/targets.yaml` for target counts/series; shared Helm values and metrics
are in `common/`; both TA modes share `common/allocator.yaml` and `common/ta.alloy`.
Edit `ta-packing/allocator.yaml` for sizing/probe settings (default size 100,
5s allocation cycles, six-cycle windows, four probes at a time, refreshed every 30s).
Deploying reapplies these files; unchanged target pods stay in place.

| Preliminary observations (earlier runs; placement depends on target addresses) | Series per Alloy |
| --- | --- |
| Alloy clustering | 120 / 90 / 2430 |
| TA consistent hashing | 880 / 100 / 1660 |
| TA load-shedding (`ta-packing`, same targets) | 920 / 900 / 820 |

Total: 2,640 generated series. Query `sum by (scraper) (scrape_samples_scraped{job="target-allocation",run_id="ta-packing"})` in Cloud; allow rolling restarts, size windows and scrape refreshes to settle after switching. HA is deferred; see [ALGO.md](../../../../ALGO.md).
