---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.mysql/
description: Learn about database_observability.mysql
title: database_observability.mysql
labels:
  stage: experimental
  products:
    - oss
---

# `database_observability.mysql`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
database_observability.mysql "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
}
```

## Arguments

You can use the following arguments with `database_observability.mysql`:

| Name                               | Type                 | Description                                                                             | Default | Required |
|------------------------------------|----------------------|-----------------------------------------------------------------------------------------|---------|----------|
| `data_source_name`                 | `secret`             | [Data Source Name][] for the MySQL server to connect to.                                |         | yes      |
| `forward_to`                       | `list(LogsReceiver)` | Where to forward log entries after processing.                                          |         | yes      |
| `collect_interval`                 | `duration`           | How frequently to collect information from database.                                    | `"1m"`  | no       |
| `disable_collectors`               | `list(string)`       | A list of collectors to disable from the default set.                                   |         | no       |
| `disable_query_redaction`          | `bool`               | Collect unredacted sql query text including parameters.                                 | `false` | no       |
| `enable_collectors`                | `list(string)`       | A list of collectors to enable on top of the default set.                               |         | no       |
| `setup_consumers_collect_interval` | `duration`           | How frequently to collect performance_schema.setup_consumers information from database. | `"1h"`  | no       |

The following collectors are configurable:

| Name              | Description                                           | Enabled by default |
|-------------------|-------------------------------------------------------|--------------------|
| `query_tables`    | Collect query table information.                      | yes                |
| `schema_table`    | Collect schemas and tables from `information_schema`. | yes                |
| `query_sample`    | Collect query samples.                                | no                 |
| `setup_consumers` | Collect enabled `performance_schema.setup_consumers`. | yes                |

## Blocks

The `database_observability.mysql` component doesn't support any blocks. You can configure this component with arguments.

## Example

```alloy
# This block configures the database_observability.mysql component to collect MySQL metrics and logs and forward logs to Loki.
database_observability.mysql "orders_db" {
  # The MySQL Data Source Name (DSN) to connect to.
  data_source_name = "user:pass@tcp(mysql:3306)/"
  # Forward collected logs to the Loki logs_service receiver.
  forward_to = [loki.write.logs_service.receiver]
}

# This block configures Prometheus to scrape metrics from the MySQL observability component.
prometheus.scrape "orders_db" {
  # Use the targets exported by the database_observability.mysql component.
  targets = database_observability.mysql.orders_db.targets
  # Required to keep job and instance labels.
  honor_labels = true
  # Forward scraped metrics to the remote_write receiver.
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}

# This block configures Prometheus remote_write to send metrics to a Grafana Cloud endpoint.
prometheus.remote_write "metrics_service" {
  endpoint {
    # Set the remote_write endpoint URL for hosted metrics.
    url = sys.env("<GRAFANA_CLOUD_HOSTED_METRICS_URL>")
    # Configure basic authentication for the metrics endpoint.
    basic_auth {
      username = sys.env("<GRAFANA_CLOUD_HOSTED_METRICS_ID>")
      password = sys.env("<GRAFANA_CLOUD_RW_API_KEY>")
    }
  }
}

# This block configures Loki to write logs to a Grafana Cloud endpoint.
loki.write "logs_service" {
  endpoint {
    # Set the Loki endpoint URL for hosted logs.
    url = sys.env("<GRAFANA_CLOUD_HOSTED_LOGS_URL>")
    # Configure basic authentication for the logs endpoint.
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

[Data Source Name]: https://github.com/go-sql-driver/mysql#dsn-data-source-name

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`database_observability.mysql` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`database_observability.mysql` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
