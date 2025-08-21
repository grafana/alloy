---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.postgres/
description: Learn about database_observability.postgres
title: database_observability.postgres
labels:
  stage: experimental
  products:
    - oss
---

# `database_observability.postgres`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
database_observability.postgres "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
}
```

## Arguments

You can use the following arguments with `database_observability.postgres`:

| Name                               | Type                 | Description                                                                                    | Default | Required |
|------------------------------------|----------------------|------------------------------------------------------------------------------------------------|---------|----------|
| `data_source_name`                 | `secret`             | [Data Source Name][] for the Postgres server to connect to.                                    |         | yes      |
| `forward_to`                       | `list(LogsReceiver)` | Where to forward log entries after processing.                                                 |         | yes      |
| `collect_interval`                 | `duration`           | How frequently to collect information from database.                                           | `"1m"`  | no       |
| `disable_collectors`               | `list(string)`       | A list of collectors to disable from the default set.                                          |         | no       |
| `disable_query_redaction`          | `bool`               | Collect unredacted SQL query text including parameters.                                        | `false` | no       |
| `enable_collectors`                | `list(string)`       | A list of collectors to enable on top of the default set.                                      |         | no       |
| `query_sample_collect_interval`    | `duration`           | How frequently to collect query samples from database.                                         | `"15s"` | no       |

The following collectors are configurable:

| Name              | Description                                                                                               | Enabled by default |
|-------------------|-----------------------------------------------------------------------------------------------------------|--------------------|
| `query_sample`    | Collect PostgreSQL activity information from pg_stat_activity, including query samples and wait events.   | no                 |
| `query_tables`    | Collect query table information.                                                                          | no                 |

## Blocks

The `database_observability.postgres` component doesn't support any blocks. You can configure this component with arguments.

## Example

```alloy
database_observability.postgres "orders_db" {
  data_source_name = "postgres://user:pass@localhost:5432/mydb"
  forward_to = [loki.write.logs_service.receiver]
  enable_collectors = ["activity", "query_tables"]
}

prometheus.scrape "orders_db" {
  targets = database_observability.postgres.orders_db.targets
  honor_labels = true // required to keep job and instance labels
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}

prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("<GRAFANA_CLOUD_HOSTED_METRICS_URL>")
    basic_auth {
      username = sys.env("<GRAFANA_CLOUD_HOSTED_METRICS_ID>")
      password = sys.env("<GRAFANA_CLOUD_RW_API_KEY>")
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("<GRAFANA_CLOUD_HOSTED_LOGS_URL>")
    basic_auth {
      username = sys.env("<GRAFANA_CLOUD_HOSTED_LOGS_ID>")
      password = sys.env("<GRAFANA_CLOUD_RW_API_KEY>")
    }
  }
}
```

Replace the following:

* _`<GRAFANA_CLOUD_HOSTED_METRICS_URL>`_: The URL for your Grafana Cloud hosted metrics.
* _`<GRAFANA_CLOUD_HOSTED_METRICS_ID>`_: The user ID for your Grafana Cloud hosted metrics.
* _`<GRAFANA_CLOUD_RW_API_KEY>`_: Your Grafana Cloud API key.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_URL>`_: The URL for your Grafana Cloud hosted logs.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_ID>`_: The user ID for your Grafana Cloud hosted logs.

[Data Source Name]: https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`database_observability.postgres` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`database_observability.postgres` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
