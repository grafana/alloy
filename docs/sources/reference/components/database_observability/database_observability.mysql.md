---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.mysql/
description: Learn about database_observability.mysql
title: database_observability.mysql
labels:
  stage: experimental
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

 Name                 | Type                 | Description                                               | Default | Required 
----------------------|----------------------|-----------------------------------------------------------|---------|----------
 `data_source_name`   | `secret`             | [Data Source Name][] for the MySQL server to connect to.  |         | yes      
 `forward_to`         | `list(LogsReceiver)` | Where to forward log entries after processing.            |         | yes      
 `collect_interval`   | `duration`           | How frequently to collect information from database.      | `"1m"`  | no       
 `disable_collectors` | `list(string)`       | A list of collectors to disable from the default set.     |         | no       
 `enable_collectors`  | `list(string)`       | A list of collectors to enable on top of the default set. |         | no       

The following collectors are enabled by default:

 Name           | Description                                           
----------------|-------------------------------------------------------
 `query_sample` | Collect query samples.                                
 `schema_table` | Collect schemas and tables from `information_schema`. 

## Blocks

The `database_observability.mysql` component doesn't support any blocks. You can configure this component with arguments.

## Example

```alloy
database_observability.mysql "orders_db" {
  data_source_name = "user:pass@tcp(mysql:3306)/"
  forward_to = [loki.write.logs_service.receiver]
}

prometheus.scrape "orders_db" {
  targets = database_observability.mysql.orders_db.targets
  honor_labels = true // required to keep job and instance labels
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}

prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("<GCLOUD_HOSTED_METRICS_URL>")
    basic_auth {
      username = sys.env("<GCLOUD_HOSTED_METRICS_ID>")
      password = sys.env("<GCLOUD_RW_API_KEY>")
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("<GCLOUD_HOSTED_LOGS_URL>")
    basic_auth {
      username = sys.env("<GCLOUD_HOSTED_LOGS_ID>")
      password = sys.env("<GCLOUD_RW_API_KEY>")
    }
  }
}
```

Replace the following:

* _`<GCLOUD_HOSTED_METRICS_URL>`_: The URL for your Google Cloud hosted metrics.
* _`<GCLOUD_HOSTED_METRICS_ID>`_: The user ID for your Google Cloud hosted metrics.
* _`<GCLOUD_RW_API_KEY>`_: Your Google Cloud API key.
* _`<GCLOUD_HOSTED_LOGS_URL>`_: The URL for your Google Cloud hosted logs.
* _`<GCLOUD_HOSTED_LOGS_ID>`_: The user ID for your Google Cloud hosted logs.

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
