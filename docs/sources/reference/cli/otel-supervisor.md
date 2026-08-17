---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/otel-supervisor/
description: Learn about the otel-supervisor command
labels:
  stage: experimental
  products:
    - oss
title: otel-supervisor
weight: 360
---

# `otel-supervisor`

> **EXPERIMENTAL**: This is an [experimental][] feature.
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.

[experimental]: https://grafana.com/docs/release-life-cycle/

The `otel-supervisor` command runs {{< param "PRODUCT_NAME" >}} with the {{< param "OTEL_ENGINE" >}} under an embedded OpAMP supervisor.
The supervisor connects to Grafana Fleet Management and receives the {{< param "OTEL_ENGINE" >}} configuration through OpAMP.

{{< admonition type="note" >}}
Grafana Fleet Management doesn't officially support {{< param "PRODUCT_NAME" >}}. Grafana Labs doesn't provide support for this configuration.
{{< /admonition >}}

## Usage

To use environment-based configuration, run the following command:

```shell
alloy otel-supervisor
```

To use a supervisor configuration file, run the following command:

```shell
alloy otel-supervisor --config=<SUPERVISOR_CONFIG_FILE>
```

Replace _`<SUPERVISOR_CONFIG_FILE>`_ with the path to an OpAMP supervisor configuration file.

## Configure with environment variables

When you omit `--config`, the command reads the following environment variables:

* `GCLOUD_FM_URL`: The Grafana Fleet Management base URL. The command adds `/v1/opamp` when the URL doesn't include it.
* `GCLOUD_INSTANCE_ID`: Your Grafana Cloud instance ID.
* `GCLOUD_RW_API_KEY`: Your Grafana Cloud API token.
* `STORAGE_DIR`: A directory where the supervisor stores its data.

Set the variables before you start the supervisor. The following shell example uses placeholder values:

```shell
export GCLOUD_FM_URL="<FLEET_MANAGEMENT_URL>"
export GCLOUD_INSTANCE_ID="<GRAFANA_CLOUD_INSTANCE_ID>"
export GCLOUD_RW_API_KEY="<GRAFANA_CLOUD_API_TOKEN>"
export STORAGE_DIR="<SUPERVISOR_STORAGE_DIRECTORY>"

alloy otel-supervisor
```

For information about setting up Fleet Management, refer to the [Grafana Fleet Management documentation](https://grafana.com/docs/grafana-cloud/send-data/fleet-management/).
