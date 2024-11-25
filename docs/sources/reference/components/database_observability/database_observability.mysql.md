---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.mysql/
description: Learn about database_observability.mysql
title: database_observability.mysql
---

Database Observability component. This component is under active development and can be run with Alloy flag `--stability.level=experimental`.

## Example

```alloy
database_observability.mysql "orders_db" {
  data_source_name = "user:pass@mysql:3306/"
  forward_to = [loki.write.logs_service.receiver]
}

prometheus.scrape "orders_db" {
  targets = database_observability.mysql.orders_db.targets
  honor_labels = true // required to keep job and instance labels
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}

prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_METRICS_URL")
    basic_auth {
      username = sys.env("GCLOUD_HOSTED_METRICS_ID")
      password = sys.env("GCLOUD_RW_API_KEY")
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_LOGS_URL")
    basic_auth {
      username = sys.env("GCLOUD_HOSTED_LOGS_ID")
      password = sys.env("GCLOUD_RW_API_KEY")
    }
  }
}
```

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
