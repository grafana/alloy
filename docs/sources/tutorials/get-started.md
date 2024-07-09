---
canonical: https://grafana.com/docs/alloy/latest/tutorials/get-started/
description: Get started with Grafana Alloy
title: Get started with Grafana Alloy
weight: 10
---

## Get started with {{% param "FULL_PRODUCT_NAME" %}}

This tutorial shows you how to configure {{< param "PRODUCT_NAME" >}} to collect logs from your local computer, filter non-essential log lines, send them to Loki, and use a Grafana dashboard to query and visualize the results.

## Before you begin

To complete this tutorial:

* You must have a basic understanding of Alloy and telemetry collection in general.
* You should be familiar with Prometheus, PromQL, Loki, LogQL, and basic Grafana navigation.
* You don't need to understand the {{< param "PRODUCT_NAME" >}} [configuration syntax][configuration] concepts.

## Install {{% param "PRODUCT_NAME" %}} and start the service

This tutorial requires a Linux or macOS environment with Docker installed.

### Linux

Install and run {{< param "PRODUCT_NAME" >}} on Linux.

1. [Install {{< param "PRODUCT_NAME" >}}][Linux Install].

1. [Run {{< param "PRODUCT_NAME" >}}][Run on Linux].

### macOS

Install  and run {{< param "PRODUCT_NAME" >}} on macOS.

1. [Install {{< param "PRODUCT_NAME" >}}][macOS Install].

1. [Run {{< param "PRODUCT_NAME" >}}][Run on macOS].

## Set up a local Grafana instance

To allow {{< param "PRODUCT_NAME" >}} to write data to Loki running in the local Grafana stack, you can use the following Docker Compose file to set up a local Grafana instance alongside Loki and Prometheus, which are pre-configured as data sources.

```yaml
version: '3'
services:
  loki:
    image: grafana/loki:3.0.0
    ports:
      - "3100:3100"
    command: -config.file=/etc/loki/local-config.yaml
  prometheus:
    image: prom/prometheus:v2.47.0
    command:
      - --web.enable-remote-write-receiver
      - --config.file=/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
  grafana:
    environment:
      - GF_PATHS_PROVISIONING=/etc/grafana/provisioning
      - GF_AUTH_ANONYMOUS_ENABLED=true
      - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
    entrypoint:
      - sh
      - -euc
      - |
        mkdir -p /etc/grafana/provisioning/datasources
        cat <<EOF > /etc/grafana/provisioning/datasources/ds.yaml
        apiVersion: 1
        datasources:
        - name: Loki
          type: loki
          access: proxy
          orgId: 1
          url: http://loki:3100
          basicAuth: false
          isDefault: false
          version: 1
          editable: false
        - name: Prometheus
          type: prometheus
          orgId: 1
          url: http://prometheus:9090
          basicAuth: false
          isDefault: true
          version: 1
          editable: false
        EOF
        /run.sh
    image: grafana/grafana:11.0.0
    ports:
      - "3000:3000"
```

Run `docker compose up` to start your Docker container and open [http://localhost:3000](http://localhost:3000) in your browser to view the Grafana UI.

 {{< admonition type="note" >}}
If you the following error when you start your Docker container, `docker: 'compose' is not a docker command`, use the command `docker-compose up` to start your Docker container.
 {{< /admonition >}}

## Configure {{% param "PRODUCT_NAME" %}}

After the local Grafana instance is setup, the next step is to configure {{< param "PRODUCT_NAME" >}}.
You use components in the `config.alloy` file to tell {{< param "PRODUCT_NAME" >}} which logs you want to scrape, how you want to process that data, and where you want the data sent.

The examples run on a single host so that you can run them on your laptop or in a Virtual Machine.
You can try the examples using a `config.alloy` file and experiment with the examples yourself.

For the following steps, create a file called `config.alloy` in your current working directory.

### First component: Log files

Paste the following component configuration at the top of your `config.alloy` file:

```alloy
local.file_match "local_files" {
    path_targets = [{"__path__" = "/var/log/*.log"}]
    sync_period = "5s"
}
```

This configuration creates a [local.file_match][] component named `local_files` which does the following:

1. It tells {{< param "PRODUCT_NAME" >}} which files to source.
1. It checks for new files every 5 seconds.

### Second component: Scraping

Paste this component configuration below the previous component in your `config.alloy` file:

```alloy
loki.source.file "log_scrape" {
   targets    = local.file_match.local_files.targets
   forward_to = [loki.process.filter_logs.receiver]
   tail_from_end = true
}
```

This configuration creates a [loki.source.file][] component named `log_scrape`, and shows the pipeline concept of {{< param "PRODUCT_NAME" >}} in action. This component does the following:

1. It connects to the `local_files` component as its "source" or target.
1. It forwards the logs it scrapes to the receiver of another component called `filter_logs`.
1. It provides extra attributes and options to tail the log files from the end so you don't ingest the entire log file history.

### Third component: Filter non-essential logs

Filtering non-essential logs before sending them to a data source can help you manage log volumes to reduce costs.
The filtering strategy of each organization differs because they have different monitoring needs and setups.

The following example demonstrates filtering out or dropping logs before sending them to Loki.

Paste this component configuration below the previous component in your `config.alloy` file:

```alloy
loki.process "filter_logs" {
  stage.drop {
       source = ""
       expression  = ".*Connection closed by authenticating user root"
       drop_counter_reason = "noisy"
    }
  forward_to = [loki.write.grafana_loki.receiver]
  }
```

The `loki.process` component allows you to transform, filter, parse, and enrich log data.
Within this component, you can define one or more processing stages to specify how you would like to process log entries before they're stored or forwarded.

This configuration creates a `filter_logs` component. This component does the following:

* It receives scraped log entries from the default `log_scrape` component.
* It uses the `stage.drop` block to define what to drop from the scraped logs.
* It uses the `expression` parameter to identify the specific log entries to drop.
* It uses an optional string label `drop_counter_reason` to show the reason for dropping the log entries.
* It forwards the processed logs to the receiver of another component called `grafana_loki`.

The following [tutorial][] and the [`loki.process` documentation][loki.process] provide more comprehensive information on processing logs.

### Fourth component: Write logs to Loki

Paste this component configuration below the previous component in your `config.alloy` file:

```alloy
loki.write "grafana_loki" {
  endpoint {
    url = "http://localhost:3100/loki/api/v1/push"

    // basic_auth {
    //  username = "admin"
    //  password = "admin"
    // }
  }
}
```

This final component creates a [`loki.write`][] component named `grafana_loki` that points to `http://localhost:3100/loki/api/v1/push`.

This completes the simple configuration pipeline.

{{< admonition type="tip" >}}
The `basic_auth` block is commented out because the local `docker compose` stack doesn't require it.
It's included in this example to show how you can configure authorization for other environments.
For further authorization options, refer to the [loki.write][] component reference.

[loki.write]: ../../reference/components/loki.write/
{{< /admonition >}}

With this configuration, {{< param "PRODUCT_NAME" >}} connects directly to the Loki instance running in the Docker container.

## Reload the configuration

1. Copy your local `config.alloy` file into the default configuration file location.

   {{< code >}}

   ```macos
   sudo cp config.alloy $(brew --prefix)/etc/alloy/config.alloy
   ```

   ```linux
   sudo cp config.alloy /etc/alloy/config.alloy
   ```

   {{< /code >}}

1. Call the `/-/reload` endpoint to tell {{< param "PRODUCT_NAME" >}} to reload the configuration file without a system service restart.

   ```bash
   curl -X POST http://localhost:12345/-/reload
   ```

   {{< admonition type="tip" >}}
   This step uses the {{< param "PRODUCT_NAME" >}} UI on `localhost` port `12345`.
   If you chose to run {{< param "PRODUCT_NAME" >}} in a Docker container, make sure you use the `--server.http.listen-addr=0.0.0.0:12345` argument.
   If you don’t use this argument, the [debugging UI][debug] won’t be available outside of the Docker container.

   [debug]: ../../tasks/debug/#alloy-ui
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

Open <http://localhost:12345> and click the Graph tab at the top.
The graph should look similar to the following:

{{< figure src="/media/docs/alloy/tutorial/Inspect-your-config-in-the-Alloy-UI-image.png" alt="Your configuration in the Alloy UI" >}}

The {{< param "PRODUCT_NAME" >}} UI shows you a visual representation of the pipeline you built with your {{< param "PRODUCT_NAME" >}} component configuration.

You can see that the components are healthy, and you are ready to go.

## Log in to Grafana and explore Loki logs

Open <http://localhost:3000/explore> to access **Explore** feature in Grafana.
Select Loki as the data source and click the **Label Browser** button to select a file that {{< param "PRODUCT_NAME" >}} has sent to Loki.

Here you can see that logs are flowing through to Loki as expected, and the end-to-end configuration was successful.

{{< figure src="/media/docs/alloy/tutorial/loki-logs.png" alt="Logs reported by Alloy in Grafana" >}}

## Summary

You have installed and configured {{< param "PRODUCT_NAME" >}}, and sent logs from your local host to a Grafana stack.

In the following tutorials, you learn more about configuration concepts and metrics.

[MacOS Install]: ../../get-started/install/macos/
[Linux Install]: ../../get-started/install/linux/
[Run on Linux]: ../../get-started/run/linux/
[Run on MacOS]: ../../get-started/run/macos/
[local.file_match]: ../../reference/components/local.file_match/
[loki.write]: ../../reference/components/loki.write/
[loki.source.file]: ../../reference/components/loki.source.file/
[alloy]: https://grafana.com/docs/alloy/latest/
[configuration]: ../../concepts/configuration-syntax/
[install]: ../../get-started/install/binary/#install-alloy-as-a-standalone-binary
[debugging your configuration]: ../../tasks/debug/
[parse]: ../../reference/components/loki.process/
[tutorial]: ../processing-logs/
[loki.process]: ../../reference/components/loki.process/
