---
canonical: https://grafana.com/docs/alloy/latest/monitor/import-mixin-dashboards-and-alerts/
description: Import rendered Alloy mixin dashboards and alert rules into Grafana
menuTitle: Import mixin dashboards and alerts
title: Import rendered mixin dashboards and alert rules into Grafana
weight: 650
---

# Import rendered mixin dashboards and alert rules into Grafana

Use this task to import dashboards and alert rules generated from the Alloy mixin into your Grafana instance.
You can import dashboards from rendered JSON files in the source repository or from the release zip archive.

## Before you begin

Before you begin, ensure you have the following:

- Access to a Grafana instance where you can import dashboards and alert rules.
- Access to rendered mixin files from one of these sources:
  - The Alloy source repository at `operations/alloy-mixin/rendered/`.
  - The release archive file `alloy-mixin-dashboards-<RELEASE_TAG>.zip`.
- A configured Prometheus data source in Grafana for imported dashboards.

## Get rendered mixin files

Choose one source for dashboards:

- **Source repository:** Use the rendered files committed to `operations/alloy-mixin/rendered/`.
- **Release archive:** Download `alloy-mixin-dashboards-<RELEASE_TAG>.zip` from a release artifact and extract it.

If you need alert rules, use the source repository path `operations/alloy-mixin/rendered/alerts/`.
The release zip archive contains dashboard JSON files only.

To regenerate rendered files from source, run:

```sh
make generate-rendered-mixin
```

The command writes:

- Dashboard files to `operations/alloy-mixin/rendered/dashboards/*.json`.
- Alert rule files to `operations/alloy-mixin/rendered/alerts/*.yaml`.

## Import dashboards from JSON files

Use the Grafana dashboard import UI to import each rendered dashboard JSON file:

1. Open Grafana and go to **Dashboards**.
2. Click **New** and then click **Import**.
3. Upload a JSON file from `operations/alloy-mixin/rendered/dashboards/` or from the extracted zip archive.
4. Select the target data source mappings.
5. Click **Import**.
6. Repeat for each dashboard file you want to import.

For full UI details and options, refer to the official Grafana documentation on [importing dashboards](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/import-dashboards/).

## Import alert rules

Each file in `operations/alloy-mixin/rendered/alerts/*.yaml` is a Prometheus-style rule group.
Import these files with Grafana Alerting by following the official workflow for [importing alert rules](https://grafana.com/docs/grafana/latest/alerting/alerting-rules/alerting-migration/#import-rules-with-grafana-alerting).

## Next steps

- Refer to [Monitor metrics and logs with Grafana Alloy](https://grafana.com/docs/alloy/latest/monitor/) for end-to-end monitoring tasks.
- Refer to the [Alloy mixin source](https://github.com/grafana/alloy/tree/main/operations/alloy-mixin) for rendered files and customization options.
