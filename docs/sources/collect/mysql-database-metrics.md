---
canonical: https://grafana.com/docs/alloy/latest/collect/mysql-database-metrics/
description: Learn how to collect MySQL database metrics and logs with Grafana Alloy.
title: Collect MySQL database metrics and logs
labels:
  stage: experimental
---

# Collect MySQL database metrics and logs

[SCAFFOLD PLACEHOLDER: This topic is a scaffold. The following example is complete and functional, but narrative explanation and step-by-step guidance are pending. See the `develop-task-topic` skill for the next step in completing this scaffold.]

```alloy
// MySQL database observability with Loki and Prometheus
database_observability.mysql "example" {
  data_source_name = "user:pass@tcp(mysql:3306)/"
  forward_to       = [loki.relabel.example.receiver]
  targets          = prometheus.exporter.mysql.example.targets

  enable_collectors = ["query_samples", "explain_plans"]

  cloud_provider {
    aws {
      arn = "your-rds-db-arn"
    }
  }
}

prometheus.exporter.mysql "example" {
  data_source_name  = "user:pass@tcp(mysql:3306)/"
  enable_collectors = ["perf_schema.eventsstatements"]
}

loki.relabel "example" {
  forward_to = [loki.write.logs_service.receiver]
  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
  rule {
    target_label = "instance"
    replacement  = "example"
  }
}

discovery.relabel "example" {
  targets = database_observability.mysql.example.targets

  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
  rule {
    target_label = "instance"
    replacement  = "example"
  }
}

prometheus.scrape "example" {
  targets    = discovery.relabel.example.output
  job_name   = "integrations/db-o11y"
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
