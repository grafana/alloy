---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/import-mixin-dashboards/
description: Import rendered Grafana Alloy mixin dashboards 
menuTitle: Import mixin dashboards
title: Import rendered mixin dashboards
weight: 250
---

# Import rendered mixin dashboards

The {{< param "FULL_PRODUCT_NAME" >}} mixin defines dashboards you can use to monitor collector health, resource use, and internal pipeline metrics. You can import these dashboards from the {{< param "FULL_PRODUCT_NAME" >}} mixin directly into your Grafana instance.
The dashboards are available as rendered JSON files in the source repository or in the release archive.

## Before you begin

Before you begin, ensure you have the following:

- Access to a Grafana instance where you can import dashboards.
- Access to rendered mixin files from one of these sources:
  - The {{< param "PRODUCT_NAME" >}} source repository at `operations/alloy-mixin/rendered/dashboards/`.
  - The release archive `alloy-mixin-dashboards-<RELEASE_TAG>.zip`.
- A configured Prometheus data source in Grafana for imported dashboards.

## Get rendered mixin dashboard files

Choose one source for dashboards:

- **Source repository:** Use the rendered dashboard files located in `operations/alloy-mixin/rendered/dashboards/` in your copy or clone of the {{< param "PRODUCT_NAME" >}} repository.
- **Release archive:** Download `alloy-mixin-dashboards-<RELEASE_TAG>.zip` from an {{< param "PRODUCT_NAME" >}} release artifact and extract the archive.

## Import dashboards from JSON files

Use the Grafana dashboard import UI to import each rendered dashboard JSON file:

1. Open Grafana and go to **Dashboards**.
2. Click **New** and then click **Import**.
3. Upload a JSON file from `operations/alloy-mixin/rendered/dashboards/` or from the extracted zip archive.
4. Select the target data source mappings.
5. Click **Import**.
6. Repeat for each dashboard file you want to import.

For full UI details and options, refer to [importing dashboards](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/import-dashboards/) in the Grafana documentation.

## Next steps

- Refer to [Troubleshoot {{< param "FULL_PRODUCT_NAME" >}}](../) for additional troubleshooting tasks.
- Refer to the [{{< param "PRODUCT_NAME" >}} mixin source](https://github.com/grafana/alloy/tree/main/operations/alloy-mixin) for rendered files and customization options.
