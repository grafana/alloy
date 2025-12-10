---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.databricks/
aliases:
  - ../prometheus.exporter.databricks/ # /docs/alloy/latest/reference/components/prometheus.exporter.databricks/
description: Learn about prometheus.exporter.databricks
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.databricks
---

# `prometheus.exporter.databricks`

The `prometheus.exporter.databricks` component embeds the [`databricks_exporter`](https://github.com/grafana/databricks-prometheus-exporter) for collecting billing, jobs, pipelines, and SQL warehouse metrics from Databricks System Tables via HTTP for Prometheus consumption.

## Usage

```alloy
prometheus.exporter.databricks "LABEL" {
    server_hostname     = "<DATABRICKS_SERVER_HOSTNAME>"
    warehouse_http_path = "<DATABRICKS_WAREHOUSE_HTTP_PATH>"
    client_id           = "<DATABRICKS_CLIENT_ID>"
    client_secret       = "<DATABRICKS_CLIENT_SECRET>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.databricks`:

| Name                    | Type       | Description                                                                     | Default | Required |
|-------------------------|------------|---------------------------------------------------------------------------------|---------|----------|
| `server_hostname`       | `string`   | The Databricks workspace hostname (e.g., `dbc-xxx.cloud.databricks.com`).       |         | yes      |
| `warehouse_http_path`   | `string`   | The HTTP path of the SQL Warehouse (e.g., `/sql/1.0/warehouses/abc123`).        |         | yes      |
| `client_id`             | `string`   | The OAuth2 Application ID (Client ID) of your Service Principal.                |         | yes      |
| `client_secret`         | `secret`   | The OAuth2 Client Secret of your Service Principal.                             |         | yes      |
| `query_timeout`         | `duration` | Timeout for individual SQL queries.                                             | `"5m"`  | no       |
| `billing_lookback`      | `duration` | How far back to look for billing data.                                          | `"24h"` | no       |
| `jobs_lookback`         | `duration` | How far back to look for job runs.                                              | `"2h"`  | no       |
| `pipelines_lookback`    | `duration` | How far back to look for pipeline runs.                                         | `"2h"`  | no       |
| `queries_lookback`      | `duration` | How far back to look for SQL warehouse queries.                                 | `"1h"`  | no       |
| `sla_threshold_seconds` | `int`      | Duration threshold (seconds) for job SLA miss detection.                        | `3600`  | no       |
| `collect_task_retries`  | `bool`     | Collect task retry metrics (high cardinality due to `task_key` label).          | `false` | no       |

## Blocks

The `prometheus.exporter.databricks` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.databricks` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.databricks` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.databricks` doesn't expose any component-specific debug metrics.

## Prerequisites

Before using this component, you need:

1. **Databricks Workspace** with Unity Catalog and System Tables enabled
2. **Service Principal** with OAuth2 M2M authentication configured
3. **SQL Warehouse** for querying System Tables (serverless recommended for cost efficiency)

See the [Databricks documentation](https://docs.databricks.com/en/dev-tools/auth/oauth-m2m.html) for detailed OAuth2 M2M setup instructions.

## Example

The following example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.databricks`:

```alloy
prometheus.exporter.databricks "example" {
  server_hostname     = "dbc-abc123-def456.cloud.databricks.com"
  warehouse_http_path = "/sql/1.0/warehouses/xyz789"
  client_id           = "my-service-principal-id"
  client_secret       = "my-service-principal-secret"
}

// Configure a prometheus.scrape component to collect databricks metrics.
prometheus.scrape "demo" {
  targets         = prometheus.exporter.databricks.example.targets
  forward_to      = [prometheus.remote_write.demo.receiver]
  scrape_interval = "5m"
  scrape_timeout  = "4m"
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

## Tuning recommendations

- **`scrape_interval`**: Default is 5 minutes. The exporter queries Databricks System Tables which can be slow. Increase to reduce SQL Warehouse costs.
- **`scrape_timeout`**: Default is 4 minutes. The exporter typically takes 90-120 seconds per scrape depending on data volume.

## High cardinality warning

The `collect_task_retries` flag adds task-level retry metrics which can significantly increase cardinality for workspaces with many jobs. Only enable if needed.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.databricks` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

