---
canonical: https://grafana.com/docs/alloy/latest/tutorials/send-metrics-to-prometheus/
description: Learn how to send metrics to Prometheus
title: Use Grafana Alloy to send metrics to Prometheus
menuTitle: Send metrics to Prometheus
weight: 150
---
# Use {{% param "FULL_PRODUCT_NAME" %}} to send metrics to Prometheus

In the [previous tutorial][], you learned how to configure {{< param "PRODUCT_NAME" >}} to collect and process logs from your local machine and send them to Loki.

This tutorial shows you how to configure {{< param "PRODUCT_NAME" >}} to collect and process metrics from your local machine, send them to Prometheus, and use Grafana to explore the results.

## Before you begin

To complete this tutorial:

* You must have a basic understanding of Alloy and telemetry collection in general.
* You should be familiar with Prometheus, PromQL, Loki, LogQL, and basic Grafana navigation.
* You must complete the [previous tutorial][] to prepare the following prerequisites:
  * Install {{< param "PRODUCT_NAME" >}} and start the service in your environment.
  * Set up a local Grafana instance.
  * Create a `config.alloy` file.

## Configure {{% param "PRODUCT_NAME" %}}

In this tutorial, you configure {{< param "PRODUCT_NAME" >}} to collect metrics and send them to Prometheus.

You add components to your `config.alloy` file to tell {{< param "PRODUCT_NAME" >}} which metrics you want to scrape, how you want to process that data, and where you want the data sent.

The following steps build on the `config.alloy` file you created in the previous tutorial.

### First component: Scraping

Paste the following component configuration at the top of your `config.alloy` file:

```alloy
prometheus.exporter.unix "local_system" { }

prometheus.scrape "scrape_metrics" {
  targets         = prometheus.exporter.unix.local_system.targets
  forward_to      = [prometheus.relabel.filter_metrics.receiver]
  scrape_interval = "10s"
}
```

This configuration creates a [`prometheus.scrape`][prometheus.scrape] component named `scrape_metrics` which does the following:

* It connects to the `local_system` component as its source or target.
* It forwards the metrics it scrapes to the receiver of another component called `filter_metrics`.
* It tells {{< param "PRODUCT_NAME" >}} to scrape metrics every 10 seconds.

### Second component: Filter metrics

Filtering non-essential metrics before sending them to a data source can help you reduce costs and allow you to focus on the data that matters most.

The following example demonstrates how you can filter out or drop metrics before sending them to Prometheus.

Paste the following component configuration below the previous component in your `config.alloy` file:

```alloy
prometheus.relabel "filter_metrics" {
  rule {
    action        = "drop"
    source_labels = ["env"]
    regex         = "dev"
  }

  forward_to = [prometheus.remote_write.metrics_service.receiver]
}
```

The [`prometheus.relabel`][prometheus.relabel] component is commonly used to filter Prometheus metrics or standardize the label set passed to one or more downstream receivers.
You can use this component to rewrite the label set of each metric sent to the receiver.
Within this component, you can define rule blocks to specify how you would like to process metrics before they're stored or forwarded.

This configuration creates a [`prometheus.relabel`][prometheus.relabel] component named `filter_metrics` which does the following:

* It receives scraped metrics from the `scrape_metrics` component.
* It tells {{< param "PRODUCT_NAME" >}} to drop metrics that have an `"env"` label equal to `"dev"`.
* It forwards the processed metrics to the receiver of another component called `metrics_service`.

### Third component: Write metrics to Prometheus

Paste the following component configuration below the previous component in your `config.alloy` file:

```alloy
prometheus.remote_write "metrics_service" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"

        // basic_auth {
        //   username = "admin"
        //   password = "admin"
        // }
    }
}
```

This final component creates a [`prometheus.remote_write`][prometheus.remote_write] component named `metrics_service` that points to `http://localhost:9090/api/v1/write`.

This completes the simple configuration pipeline.

{{< admonition type="tip" >}}
The `basic_auth` is commented out because the local `docker compose` stack doesn't require it.
It's included in this example to show how you can configure authorization for other environments.

For further authorization options, refer to the [`prometheus.remote_write`][prometheus.remote_write] component documentation.

[prometheus.remote_write]: ../../reference/components/prometheus/prometheus.remote_write/
{{< /admonition >}}

This connects directly to the Prometheus instance running in the Docker container.

## Reload the configuration

Copy your local `config.alloy` file into the default {{< param "PRODUCT_NAME" >}} configuration file location.

{{< code >}}

```macos
sudo cp config.alloy $(brew --prefix)/etc/alloy/config.alloy
```

```linux
sudo cp config.alloy /etc/alloy/config.alloy
```
{{< /code >}}

Call the `/-/reload` endpoint to tell {{< param "PRODUCT_NAME" >}} to reload the configuration file without a system service restart.

```bash
    curl -X POST http://localhost:12345/-/reload
```

{{< admonition type="tip" >}}
This step uses the {{< param "PRODUCT_NAME" >}} UI, on `localhost` port `12345`.
If you choose to run Alloy in a Docker container, make sure you use the `--server.http.listen-addr=0.0.0.0:12345` argument.

If you don’t use this argument, the [debugging UI][debug] won’t be available outside of the Docker container.

[debug]: ../../troubleshoot/debug/#alloy-ui
{{< /admonition >}}

1. Optional: You can do a system service restart {{< param "PRODUCT_NAME" >}} and load the configuration file:

   {{< code >}}

   ```macos
   brew services restart alloy
   ```

   ```linux
   sudo systemctl reload alloy
   ```

   {{< /code >}}


## Inspect your configuration in the {{% param "PRODUCT_NAME" %}} UI

Open [http://localhost:12345](http://localhost:12345) and click the **Graph** tab at the top.
The graph should look similar to the following:

{{< figure src="/media/docs/alloy/tutorial/Metrics-inspect-your-config.png" alt="Your configuration in the Alloy UI" >}}

The {{< param "PRODUCT_NAME" >}} UI shows you a visual representation of the pipeline you built with your {{< param "PRODUCT_NAME" >}} component configuration.

You can see that the components are healthy, and you are ready to explore the metrics in Grafana.

## Log into Grafana and explore metrics in Prometheus

Open [http://localhost:3000/explore](http://localhost:3000/explore) to access the **Explore** feature in Grafana.

Select Prometheus as the data source and click the **Metrics Browser** button to select the metric, labels, and values for your labels.

Here you can see that metrics are flowing through to Prometheus as expected, and the end-to-end configuration was successful.

{{< figure src="/media/docs/alloy/tutorial/Metrics_visualization.png" alt="Your data flowing through Prometheus." >}}

## Summary

You have configured {{< param "PRODUCT_NAME" >}} to collect and process metrics from your local host and send them to your local Grafana stack.

[previous tutorial]: ../send-logs-to-loki/
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/
[prometheus.relabel]: ../../reference/components/prometheus/prometheus.relabel/
[prometheus.remote_write]: ../../reference/components/prometheus/prometheus.remote_write/

