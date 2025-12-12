# Alloy jsonnet mixin (dashboards + alerts)

This directory contains an Alloy jsonnet mixin that can be rendered into:

- Grafana dashboard JSON files (one file per dashboard).
- Prometheus alert rule JSON files (one file per rule *group*, i.e. “alert family”).

The rendered outputs are committed under `operations/alloy-mixin/rendered/` and verified in CI to stay up to date.

## Rendering

From the repository root:

```bash
make alloy-mixin-render
```

Outputs are written to:

- `operations/alloy-mixin/rendered/dashboards/*.json`
- `operations/alloy-mixin/rendered/alerts/*.json`

## Importing dashboards into Grafana

### Manual import (Grafana UI)

1. Open Grafana.
2. Go to **Dashboards** → **New** → **Import**.
3. Upload or paste the contents of a file from `rendered/dashboards/*.json`.
4. Select the target folder (or create one) and complete the import.

Grafana docs: `https://grafana.com/docs/grafana/latest/dashboards/manage-dashboards/#import-a-dashboard`

### Provisioning (recommended for environments)

You can provision these dashboards by placing the JSON files on disk and configuring Grafana dashboard provisioning to load them from a directory.

Grafana provisioning docs: `https://grafana.com/docs/grafana/latest/administration/provisioning/#dashboards`

## Using the rendered alert rules

The files under `rendered/alerts/*.json` are Prometheus rule group JSON (each file contains a single entry in a top-level `groups` array). How you load them depends on your alerting backend:

- Prometheus “rule_files”
- Prometheus Operator / `PrometheusRule` CRD (convert JSON → YAML if needed)
- Grafana Mimir ruler / compatible rule loaders

Prometheus rule format docs: `https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/`

## Customizing via `config.libsonnet`

`config.libsonnet` controls mixin rendering options and is merged into the mixin’s `$._config`. Some commonly used toggles:

- `enableK8sCluster`: whether to include `cluster`/`namespace` label dimensions in queries and templates.
- `enableAlloyCluster`: whether to include clustering dashboards/alerts.
- `enableLokiLogs`: whether to include the logs overview dashboard.
- `filterSelector` / `logsFilterSelector`: add selectors to narrow metrics/logs to a specific Alloy installation.
- `dashboardTag`: change the dashboard tag used for cross-dashboard links.

After changing `config.libsonnet`, re-render and commit updates:

```bash
make alloy-mixin-render
```

## TODO

- Publish a release archive artifact containing the rendered dashboards and alerts.
- Publish these dashboards to grafana.com so they can be imported by ID.

