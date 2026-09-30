# Scrape allocation demo

From `example/kind`, with Docker, kind, kubectl, Helm, Task and Go installed:

```sh
task cluster:up
task deploy:vanilla      # Alloy clustering; every peer discovers targets
task deploy:ta-hashing   # Local TA; Alloy uses HTTP discovery
task deploy:ta-packing   # TODO; exits without deploying
task cluster:down        # Deletes the cluster and stops ingestion
```

Set `GRAFANA_CLOUD_PROMETHEUS_URL`, `GRAFANA_CLOUD_PROMETHEUS_USERNAME` and
`GRAFANA_CLOUD_METRICS_WRITE_TOKEN` in gitignored `example/kind/.env.credentials`.
TA builds from `~/workspace/opentelemetry-operator`; override with `OPERATOR_DIR`.
Only one kind cluster may run. The demo uses one node, three Alloy pods and one TA.
Edit `common/targets.yaml` for target counts/series; shared Helm values and metrics
are in `common/`, with discovery configs and values under each mode's folder.
Deploying reapplies these files; unchanged target pods stay in place.

| Preliminary observations (earlier runs; placement depends on target addresses) | Series per Alloy |
| --- | --- |
| Alloy clustering | 120 / 90 / 2430 |
| TA consistent hashing | 880 / 100 / 1660 |

Total: 2,640 generated series. Query `sum by (scraper) (scrape_samples_scraped{job="target-allocation",run_id="ta-hashing"})` in Cloud; allow old samples to age out after switching. HA is deferred; see [ALGO.md](../../../../ALGO.md).
