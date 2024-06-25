---
canonical: https://grafana.com/docs/alloy/latest/tutorials/send-metrics-to-prometheus/
description: Learn how to send metrics to Prometheus
menuTitle: Send metrics to Prometheus
title: Send 
weight: 15
---
# Send metrics to Prometheus
In the [Get started with {{< param "PRODUCT_NAME" >}} tutorial][get started], you learned how to configure {{< param "PRODUCT_NAME" >}} to collect and process logs from your local machine and send them to Loki, running in the local Grafana stack. 

As a next step, you will collect and process metrics from the same machine using {{< param "PRODUCT_NAME" >}} and send them to Prometheus, running in the same Grafana stack. 

This process will enable you to query and visualize the metrics sent to Prometheus using the Grafana dashboard.

## Prerequisites

Complete the [previous tutorial][get started] to:
1. Install {{< param "PRODUCT_NAME" >}} and start the service in your environment.
1. Set up a local Grafana instance.
1. Create a `config.alloy` file.

## Configure {{% param "PRODUCT_NAME" %}}

Once the prerequisite steps have been completed, the next step is to configure {{< param "PRODUCT_NAME" >}}.

Same as you did for logs, you will use the components in the `config.alloy` file to tell {{< param "PRODUCT_NAME" >}} which metrics you want to scrape, how you want to process that data, and where you want the data sent.

Add the following to the `config.alloy` file you created in the prerequisite steps.  

### First component: Scraping

Paste this component into the top of the `config.alloy` file:

```alloy
prometheus.exporter.unix "local_system" { }

prometheus.scrape "scrape_metrics" {
  targets    = prometheus.exporter.unix.local_system.targets
  forward_to = [prometheus.relabel.filter_metrics.receiver]
  scrape_interval = "10s"
}
```
This configuration defines a Prometheus exporter for a local system from which the metrics will be collected. 

It also creates a [`prometheus.scrape`][prometheus.scrape] component named `scrape_metrics` which does the following:

1. It connects to the `local_system` component (its "source" or target).
1. It forwards the metrics it scrapes to the "receiver" of another component called `filter_metrics` which you will define next.
1. It tells {{< param "PRODUCT_NAME" >}} to scrape metrics every 10 seconds. 

### Second component: Filter metrics

Filtering non-essential metrics before sending them to a data source can help you reduce costs and enable you to focus on the data that matters most. The filtering strategy of each organization will differ as they have different monitoring needs and setups. 

The following example demonstrates filtering out or dropping metrics before sending them to Prometheus. 

Paste this component next in your configuration file:
```alloy
prometheus.relabel "filter_metrics" {
   rule {
    action = "drop"
    source_labels =[ "env"]
    regex = "dev"
  }
 forward_to = [prometheus.remote_write.metrics_service.receiver]
}
```

1. `prometheus.relabel` is a component most commonly used to filter Prometheus metrics or standardize the label set passed to one or more downstream receivers. 
1. In this example, you create a `prometheus.relabel` component named “filter_metrics”. 
   This component receives scraped metrics from the `scrape_metrics` component you created in the previous step. 
1. There are many ways to [process metrics][prometheus.relabel]. 
   Within this component, you can define rule block(s) to specify how you would like to process metrics before they are stored or forwarded. 
1. This example assumes that you are monitoring a production environment and the metrics collected from the dev environment will not be needed for this particular use case. 
1. To instruct {{< param "PRODUCT_NAME" >}} to drop metrics whose environment label, `[”env]”`, is equal to `”dev”`, you set the `action` parameter to `”drop”`, set the `source_labels` parameter equal to `[“env”]`, and the `regex` parameter to `“dev”`.  
1. You use the `forward_to` parameter to specify where to send the processed metrics.
   In this case, you will send the processed metrics to a component you will create next called `metrics_service`. 

### Third component: Write metrics to Prometheus

Paste this component next in your configuration file:

```alloy

prometheus.remote_write "metrics_service" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"

        //basic_auth {
            //username = "admin"
            //password = "admin"
      // }
    }
}

```
This last component creates a [prometheus.remote_write][prometheus.remote_write] component named `metrics_service` that points to `http://localhost:9090/api/v1/write`.

This completes the simple configuration pipeline.

{{< admonition type="tip" >}}
The `basic_auth` is commented out because the local `docker compose` stack doesn't require it. 
It is included in this example to show how you can configure authorization for other environments.

For further authorization options, refer to the [`prometheus.remote_write`][prometheus.remote_write] component documentation.

[prometheus.remote_write]: ../../reference/components/prometheus.remote_write/
{{< /admonition >}}

This connects directly to the Prometheus instance running in the Docker container.

## Reload the Configuration

Copy your local `config.alloy` file into the default configuration file location, which depends on your OS.

{{< code >}}

```macos
sudo cp config.alloy $(brew --prefix)/etc/alloy/config.alloy
```

```linux
sudo cp config.alloy /etc/alloy/config.alloy
```
{{< /code >}}

Finally, call the reload endpoint to notify {{< param "PRODUCT_NAME" >}} to the configuration change without the need for restarting the system service.
```bash
    curl -X POST http://localhost:12345/-/reload
```

{{< admonition type="tip" >}}
This step uses the {{< param "PRODUCT_NAME" >}} UI, which is exposed on `localhost` port `12345`.
If you choose to run Alloy in a Docker container, make sure you use the `--server.http.listen-addr=0.0.0.0:12345` argument.

If you don’t use this argument, the [debugging UI][https://grafana.com/docs/alloy/latest/tasks/debug/#alloy-ui] won’t be available outside of the Docker container.

[debug]: ../../tasks/debug/#alloy-ui
{{< /admonition >}}

The alternative to using this endpoint is to reload the {{< param "PRODUCT_NAME" >}} configuration, which can be done as follows:

{{< code >}}

```macos
brew services restart alloy
```

```linux
sudo systemctl reload alloy
```

{{< /code >}}

## Inspect your configuration in the {{% param "PRODUCT_NAME" %}} UI

Open [http://localhost:12345] and click the Graph tab at the top.
The graph should look similar to the following: 
{{< figure src="/media/docs/alloy/tutorial/Metrics-inspect-your-config.png" alt="Your configuration in the Alloy UI" >}}

The {{< param "PRODUCT_NAME" >}} UI shows you a visual representation of the pipeline you built with your {{< param "PRODUCT_NAME" >}} component configuration.

You can see that the components are healthy, and you are ready to go.

## Log into Grafana and explore metrics in Prometheus 

Open [http://localhost:3000/explore] to access the Explore feature in Grafana.

Select Prometheus as the data source and click the **Metrics Browser** button to select the metric, labels, and values for your labels.

Here you can see that metrics are flowing through to Prometheus as expected, and the end-to-end configuration was successful.

{{< figure src="/media/docs/alloy/tutorial/Metrics_visualization.png" alt="Your data flowing through Prometheus." >}}

## Conclusion
Well done. You have configured {{< param "PRODUCT_NAME" >}} to collect and process metrics from your local host and send them to a Grafana stack. 

[get started]: ../get-started/
[prometheus.scrape]: ../../reference/components/prometheus.scrape/
[prometheus.relabel]: ../../reference/components/prometheus.relabel/
[prometheus.remote_write]: ../../reference/components/prometheus.remote_write/

