# Alloy mixin

This directory contains an Alloy jsonnet mixin that can be rendered into:

- Grafana dashboard JSON files.
- Prometheus alert rule YAML files.

For convenience, the rendered outputs are committed under the `rendered/` folder.

## Regenerating rendered outputs

From the repository root, run:

```bash
make generate-rendered-mixin
```

## Importing dashboards into Grafana

Follow [these instructions](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/import-dashboards/) to import the dashboards into your Grafana instance.

## Using the rendered alert rules

The files under `rendered/alerts/*.yaml` are Prometheus rule group YAML files.
These can be loaded into your Grafana instance following [these instructions](https://grafana.com/docs/grafana/latest/alerting/alerting-rules/alerting-migration/#import-rules-with-grafana-alerting).

## Customizing via `config.libsonnet`

`config.libsonnet` controls mixin rendering options and is merged into the mixin’s `$._config`. Refer to the comments in the file for more details on what options are available.

After changing `config.libsonnet`, re-render the mixin with:

```bash
make generate-rendered-mixin
```
