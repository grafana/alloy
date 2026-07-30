---
canonical: https://grafana.com/docs/alloy/latest/collect/mysql-database-metrics/
description: Learn how to collect MySQL database metrics and logs with Grafana Alloy.
title: Collect MySQL database metrics and logs
weight: 410
---

# Collect MySQL database metrics and logs

You can configure {{< param "PRODUCT_NAME" >}} to collect database observability data from MySQL servers including metrics, logs, and performance schema statistics.
Forward this data to Grafana Cloud, Prometheus, Loki, or any compatible observability backend.

You can accomplish the following with {{< param "PRODUCT_NAME" >}}:

* Collect query performance data, explain plans, and lock information from MySQL.
* Monitor schema and table statistics.
* Forward database observability logs to Loki.
* Scrape MySQL metrics with Prometheus.

## Components used in this topic

* [`database_observability.mysql`][database_observability.mysql]
* [`prometheus.exporter.mysql`][prometheus.exporter.mysql]
* [`loki.relabel`][loki.relabel]
* [`discovery.relabel`][discovery.relabel]
* [`prometheus.scrape`][prometheus.scrape]
* [`prometheus.remote_write`][prometheus.remote_write]
* [`loki.write`][loki.write]

## Before you begin

* Ensure you have a running MySQL server with access credentials.
* Enable the MySQL `performance_schema` on your MySQL server.
  Refer to the [MySQL performance_schema documentation](https://dev.mysql.com/doc/refman/8.0/en/performance-schema.html) for setup instructions.
* Have write access to a Prometheus-compatible endpoint or Grafana Cloud for storing metrics.
* Have write access to a Loki-compatible endpoint or Grafana Cloud for storing logs.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Collect MySQL database observability data

The `database_observability.mysql` component connects to a MySQL database and collects performance schema data.
This data includes query details, execution plans, and lock information forwarded as logs to Loki.
The component also exports targets that can be scraped with Prometheus to collect MySQL metrics.

To collect MySQL observability data and forward it to Grafana Cloud, complete the following steps:

1. Add a `database_observability.mysql` component to your configuration file.

   ```alloy
   database_observability.mysql "<LABEL>" {
     data_source_name = "<MYSQL_DSN>"
     forward_to       = [loki.relabel.<LABEL>.receiver]

     enable_collectors = ["query_samples", "explain_plans"]

     cloud_provider {
       aws {
         arn = "<AWS_RDS_ARN>"
       }
     }
   }
   ```

   Replace the following:

   * _`<LABEL>`_: The Alloy component label, such as `prod-mysql`.
   * _`<MYSQL_DSN>`_: The MySQL Data Source Name, such as `user:pass@tcp(mysql:3306)/`.
     Refer to the [go-sql-driver/mysql documentation](https://github.com/go-sql-driver/mysql#dsn-data-source-name) for the full DSN format.
   * _`<AWS_RDS_ARN>`_: The ARN of your AWS RDS database instance, such as `arn:aws:rds:us-east-1:123456789:db/prod-mysql`.
     If using Azure or GCP, use the corresponding cloud provider block instead.

   For more information, refer to the [`database_observability.mysql`][database_observability.mysql] documentation.

2. Add a `prometheus.exporter.mysql` component to export MySQL metrics.

   ```alloy
   prometheus.exporter.mysql "<LABEL>" {
     data_source_name  = "<MYSQL_DSN>"
     enable_collectors = ["perf_schema.eventsstatements"]
   }
   ```

   Replace the following:

   * _`<LABEL>`_: Must match the label used for the `database_observability.mysql` component.
   * _`<MYSQL_DSN>`_: The same MySQL Data Source Name as the `database_observability.mysql` component.

   For more information, refer to the [`prometheus.exporter.mysql`][prometheus.exporter.mysql] documentation.

3. Add a `loki.relabel` component to standardize labels on database logs.

   ```alloy
   loki.relabel "<LABEL>" {
     forward_to = [loki.write.logs_service.receiver]

     rule {
       target_label = "job"
       replacement  = "integrations/db-o11y"
     }

     rule {
       target_label = "instance"
       replacement  = "<LABEL>"
     }
   }
   ```

   Replace the following:

   * _`<LABEL>`_: Must match the label used for the `database_observability.mysql` component.

   For more information, refer to the [`loki.relabel`][loki.relabel] documentation.

4. Add a `discovery.relabel` component to standardize labels on Prometheus targets.

   ```alloy
   discovery.relabel "<LABEL>" {
     targets = database_observability.mysql.<LABEL>.targets

     rule {
       target_label = "job"
       replacement  = "integrations/db-o11y"
     }

     rule {
       target_label = "instance"
       replacement  = "<LABEL>"
     }
   }
   ```

   Replace the following:

   * _`<LABEL>`_: Must match the label used for the `database_observability.mysql` component.

   For more information, refer to the [`discovery.relabel`][discovery.relabel] documentation.

5. Add a `prometheus.scrape` component to scrape the MySQL metrics exported by both the `database_observability.mysql` and `prometheus.exporter.mysql` components.

   ```alloy
   prometheus.scrape "<LABEL>" {
     targets    = discovery.relabel.<LABEL>.output
     job_name   = "integrations/db-o11y"
     forward_to = [prometheus.remote_write.metrics_service.receiver]
   }
   ```

   Replace the following:

   * _`<LABEL>`_: Must match the label used for the `database_observability.mysql` component.

   For more information, refer to the [`prometheus.scrape`][prometheus.scrape] documentation.

6. Add a `prometheus.remote_write` component to send metrics to Grafana Cloud or Prometheus.

   ```alloy
   prometheus.remote_write "metrics_service" {
     endpoint {
       url = "<GRAFANA_CLOUD_HOSTED_METRICS_URL>"

       basic_auth {
         username = "<GRAFANA_CLOUD_HOSTED_METRICS_ID>"
         password = "<GRAFANA_CLOUD_RW_API_KEY>"
       }
     }
   }
   ```

   Replace the following:

   * _`<GRAFANA_CLOUD_HOSTED_METRICS_URL>`_: The URL for your Grafana Cloud hosted metrics endpoint.
   * _`<GRAFANA_CLOUD_HOSTED_METRICS_ID>`_: The user ID for your Grafana Cloud hosted metrics.
   * _`<GRAFANA_CLOUD_RW_API_KEY>`_: Your Grafana Cloud API key.

   For more information, refer to the [`prometheus.remote_write`][prometheus.remote_write] documentation.

7. Add a `loki.write` component to send logs to Grafana Cloud or Loki.

   ```alloy
   loki.write "logs_service" {
     endpoint {
       url = "<GRAFANA_CLOUD_HOSTED_LOGS_URL>"

       basic_auth {
         username = "<GRAFANA_CLOUD_HOSTED_LOGS_ID>"
         password = "<GRAFANA_CLOUD_RW_API_KEY>"
       }
     }
   }
   ```

   Replace the following:

   * _`<GRAFANA_CLOUD_HOSTED_LOGS_URL>`_: The URL for your Grafana Cloud hosted logs endpoint.
   * _`<GRAFANA_CLOUD_HOSTED_LOGS_ID>`_: The user ID for your Grafana Cloud hosted logs.
   * _`<GRAFANA_CLOUD_RW_API_KEY>`_: Your Grafana Cloud API key.

   For more information, refer to the [`loki.write`][loki.write] documentation.

## Complete configuration

The following is a complete configuration that collects MySQL database observability data and forwards it to Grafana Cloud.

```alloy
database_observability.mysql "example" {
  data_source_name = "<MYSQL_DSN>"
  forward_to       = [loki.relabel.example.receiver]

  enable_collectors = ["query_samples", "explain_plans"]

  cloud_provider {
    aws {
      arn = "<AWS_RDS_ARN>"
    }
  }
}

prometheus.exporter.mysql "example" {
  data_source_name  = "<MYSQL_DSN>"
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
    url = "<GRAFANA_CLOUD_HOSTED_METRICS_URL>"

    basic_auth {
      username = "<GRAFANA_CLOUD_HOSTED_METRICS_ID>"
      password = "<GRAFANA_CLOUD_RW_API_KEY>"
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = "<GRAFANA_CLOUD_HOSTED_LOGS_URL>"

    basic_auth {
      username = "<GRAFANA_CLOUD_HOSTED_LOGS_ID>"
      password = "<GRAFANA_CLOUD_RW_API_KEY>"
    }
  }
}
```

Replace the following:

* _`<MYSQL_DSN>`_: The MySQL Data Source Name for your MySQL server.
* _`<AWS_RDS_ARN>`_: The ARN of your AWS RDS database instance.
* _`<GRAFANA_CLOUD_HOSTED_METRICS_URL>`_: The URL for your Grafana Cloud hosted metrics endpoint.
* _`<GRAFANA_CLOUD_HOSTED_METRICS_ID>`_: The user ID for your Grafana Cloud hosted metrics.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_URL>`_: The URL for your Grafana Cloud hosted logs endpoint.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_ID>`_: The user ID for your Grafana Cloud hosted logs.
* _`<GRAFANA_CLOUD_RW_API_KEY>`_: Your Grafana Cloud API key.

[database_observability.mysql]: ../../reference/components/database_observability.mysql/
[prometheus.exporter.mysql]: ../../reference/components/prometheus/prometheus.exporter.mysql/
[loki.relabel]: ../../reference/components/loki/loki.relabel/
[discovery.relabel]: ../../reference/components/discovery/discovery.relabel/
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: ../../reference/components/prometheus/prometheus.remote_write/
[loki.write]: ../../reference/components/loki/loki.write/
[Components]: ../../get-started/components/
[Prometheus]: https://prometheus.io/
