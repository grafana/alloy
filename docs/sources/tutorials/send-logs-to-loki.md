---
canonical: https://grafana.com/docs/alloy/latest/tutorials/send-logs-to-loki/
aliases:
  - ./get-started/ #/docs/alloy/latest/tutorials/get-started/
description: Learn how to use Grafana Alloy to send logs to Loki
menuTitle: Send logs to Loki
title: Use Grafana Alloy to send logs to Loki
weight: 100
killercoda:
  title: Use Grafana Alloy to send logs to Loki
  description: Learn how to use Grafana Alloy to send logs to Loki
  preprocessing:
    substitutions:
      - regexp: '{{[%<] *param *"FULL_PRODUCT_NAME" *[%>]}}'
        replacement: Grafana Alloy
      - regexp: '{{[%<] *param *"PRODUCT_NAME" *[%>]}}'
        replacement: Alloy
      - regexp: 'docker compose'
        replacement: docker-compose
      - regexp: '\.\./\.\./'
        replacement: 'https://grafana.com/docs/alloy/latest/'
      - regexp: '../send-metrics-to-prometheus/'
        replacement: 'https://grafana.com/docs/alloy/latest/tutorials/send-metrics-to-prometheus/'

  backend:
    imageid: ubuntu
---

<!-- INTERACTIVE page intro.md START -->

## Use {{% param "FULL_PRODUCT_NAME" %}} to send logs to Loki

This tutorial shows you how to configure {{< param "PRODUCT_NAME" >}} to collect logs from your local machine, filter non-essential log lines, send them to Loki, and use Grafana to explore the results.

## Before you begin

To complete this tutorial:

* You must have a basic understanding of {{< param "PRODUCT_NAME" >}} and telemetry collection in general.
* You should be familiar with Prometheus, PromQL, Loki, LogQL, and basic Grafana navigation.
<!-- INTERACTIVE ignore START -->
{{< admonition type="tip" >}}
Alternatively, you can try out this example in the interactive learning environment: [Sending Logs to Loki](https://killercoda.com/grafana-labs/course/alloy/send-logs-to-loki).

It's a fully configured environment with all the dependencies already installed.

![Interactive](/media/docs/alloy/Alloy-Interactive-Learning-Environment-(Doc-Banner).png)
{{< /admonition >}}
<!-- INTERACTIVE ignore END -->

<!-- INTERACTIVE page intro.md END -->

<!-- INTERACTIVE page step1.md START -->

## Install {{% param "PRODUCT_NAME" %}} and start the service

<!-- INTERACTIVE ignore START -->
This tutorial requires a Linux or macOS environment with Docker installed.
<!-- INTERACTIVE ignore END -->

{{< docs/ignore >}}
> This online sandbox environment is based on an Ubuntu image and has Docker pre-installed. To install {{% param "PRODUCT_NAME" %}} in the sandbox, perform the following steps.

{{< /docs/ignore >}}

### Linux

Install and run {{< param "PRODUCT_NAME" >}} on Linux.

1. [Install {{< param "PRODUCT_NAME" >}}][Linux Install].
{{< docs/ignore >}}
1. To view the {{% param "PRODUCT_NAME" %}} UI within the sandbox, {{% param "PRODUCT_NAME" %}} must run on all interfaces. Run the following command before you start the {{% param "PRODUCT_NAME" %}} service.

   ```bash
   sed -i -e 's/CUSTOM_ARGS=""/CUSTOM_ARGS="--server.http.listen-addr=0.0.0.0:12345"/' /etc/default/alloy
   ```

{{< /docs/ignore >}}
1. [Run {{< param "PRODUCT_NAME" >}}][Run on Linux].

<!-- INTERACTIVE ignore START -->
### macOS

Install and run {{< param "PRODUCT_NAME" >}} on macOS.

1. [Install {{< param "PRODUCT_NAME" >}}][macOS Install].

1. [Run {{< param "PRODUCT_NAME" >}}][Run on macOS].
<!-- INTERACTIVE ignore END -->

You can access the {{< param "PRODUCT_NAME" >}} UI at [http://localhost:12345](http://localhost:12345).

<!-- INTERACTIVE page step1.md END -->

<!-- INTERACTIVE page step2.md START -->

## Set up a local Grafana instance

In this tutorial, you configure {{< param "PRODUCT_NAME" >}} to collect logs from your local machine and send them to Loki.
You can use the following Docker Compose file to set up a local Grafana instance.
This Docker Compose file includes Loki and Prometheus configured as data sources.

{{< docs/ignore >}}
> The interactive sandbox has a VSCode-like editor that allows you to access files and folders. To access this feature, click on the `Editor` tab. The editor also has a terminal that you can use to run commands. Since some commands assume you are within a specific directory, we recommend running the commands in `tab1`.
{{< /docs/ignore >}}

1. Create a directory and save the Docker Compose file as `docker-compose.yml`.

   ```bash
   mkdir alloy-tutorial
   cd alloy-tutorial
   touch docker-compose.yml
   ```

1. Copy the following Docker Compose file into `docker-compose.yml`.
   {{< docs/ignore >}}
    > We recommend using the `Editor` tab to copy and paste the Docker Compose file. However, you can also use a terminal editor like `nano` or `vim`.
   {{< /docs/ignore >}}

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

1. To start the local Grafana instance, run the following command.

   ```bash
   docker compose up -d
   ```
    <!-- INTERACTIVE ignore START -->
    {{< admonition type="note" >}}
      If you encounter the following error when you start your Docker container, `docker: 'compose' is not a docker command`, use the command `docker-compose up` to start your Docker container.
    {{< /admonition >}}
    <!-- INTERACTIVE ignore END -->

1. Open [http://localhost:3000](http://localhost:3000) in your browser to access the Grafana UI.

<!-- INTERACTIVE page step2.md END -->

<!-- INTERACTIVE page step3.md START -->

## Configure {{% param "PRODUCT_NAME" %}}

After the local Grafana instance is set up, the next step is to configure {{< param "PRODUCT_NAME" >}}.
You use components in the `config.alloy` file to tell {{< param "PRODUCT_NAME" >}} which logs you want to scrape, how you want to process that data, and where you want the data sent.

The examples run on a single host so that you can run them on your laptop or in a Virtual Machine.
You can try the examples using a `config.alloy` file and experiment with the examples.

### Create a `config.alloy` file

Create a `config.alloy` file within your current working directory.

```bash
touch config.alloy
```

### First component: Log files

Copy and paste the following component configuration at the top of the file.

```alloy
local.file_match "local_files" {
  path_targets = [{"__path__" = "/var/log/*.log"}]
  sync_period = "5s"
}
```

This configuration creates a [local.file_match][] component named `local_files` which does the following:

* It tells {{< param "PRODUCT_NAME" >}} which files to source.
* It checks for new files every 5 seconds.

### Second component: Scraping

Copy and paste the following component configuration below the previous component in your `config.alloy` file:

```alloy
loki.source.file "log_scrape" {
  targets    = local.file_match.local_files.targets
  forward_to = [loki.process.filter_logs.receiver]
  tail_from_end = true
}
```

This configuration creates a [`loki.source.file`][loki.source.file] component named `log_scrape` which does the following:

* It connects to the `local_files` component as its source or target.
* It forwards the logs it scrapes to the receiver of another component called `filter_logs`.
* It provides extra attributes and options to tail the log files from the end so you don't ingest the entire log file history.

### Third component: Filter non-essential logs

Filtering non-essential logs before sending them to a data source can help you manage log volumes to reduce costs.

The following example demonstrates how you can filter out or drop logs before sending them to Loki.

Copy and paste the following component configuration below the previous component in your `config.alloy` file:

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

This configuration creates a [`loki.process`][loki.process] component named `filter_logs` which does the following:

* It receives scraped log entries from the default `log_scrape` component.
* It uses the `stage.drop` block to define what to drop from the scraped logs.
* It uses the `expression` parameter to identify the specific log entries to drop.
* It uses an optional string label `drop_counter_reason` to show the reason for dropping the log entries.
* It forwards the processed logs to the receiver of another component called `grafana_loki`.

The [`loki.process`][loki.process] documentation provides more comprehensive information on processing logs.

### Fourth component: Write logs to Loki

Copy and paste this component configuration below the previous component in your `config.alloy` file.

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

This final component creates a [`loki.write`][loki.write] component named `grafana_loki` that points to `http://localhost:3100/loki/api/v1/push`.

This completes the simple configuration pipeline.

<!-- INTERACTIVE ignore START -->
{{< admonition type="tip" >}}
The `basic_auth` block is commented out because the local `docker compose` stack doesn't require it.
It's included in this example to show how you can configure authorization for other environments.
For further authorization options, refer to the [`loki.write`][loki.write] component reference.

[loki.write]: ../../reference/components/loki/loki.write/
{{< /admonition >}}
<!-- INTERACTIVE ignore END -->

{{< docs/ignore >}}
> The `basic_auth` block is commented out because the local `docker compose` stack doesn't require it. It's included in this example to show how you can configure authorization for other environments. For further authorization options, refer to the [`loki.write`](../../reference/components/loki/loki.write/) component reference.
{{< /docs/ignore >}}

With this configuration, {{< param "PRODUCT_NAME" >}} connects directly to the Loki instance running in the Docker container.
<!-- INTERACTIVE page step3.md END -->

<!-- INTERACTIVE page step4.md START -->

## Reload the configuration

1. Copy your local `config.alloy` file into the default {{< param "PRODUCT_NAME" >}} configuration file location.

   {{< docs/ignore >}}

   ```bash
   sudo cp config.alloy /etc/alloy/config.alloy
   ```

   {{< /docs/ignore >}}

   <!-- INTERACTIVE ignore START -->
   {{< code >}}

   ```macos
   sudo cp config.alloy $(brew --prefix)/etc/alloy/config.alloy
   ```

   ```linux
   sudo cp config.alloy /etc/alloy/config.alloy
   ```

   {{< /code >}}
   <!-- INTERACTIVE ignore END -->

1. Call the `/-/reload` endpoint to tell {{< param "PRODUCT_NAME" >}} to reload the configuration file without a system service restart.

   ```bash
   curl -X POST http://localhost:12345/-/reload
   ```
   <!-- INTERACTIVE ignore START -->
   {{< admonition type="tip" >}}
   This step uses the {{< param "PRODUCT_NAME" >}} UI on `localhost` port `12345`.
   If you chose to run {{< param "PRODUCT_NAME" >}} in a Docker container, make sure you use the `--server.http.listen-addr=0.0.0.0:12345` argument.
   If you don't use this argument, the [debugging UI][debug] won't be available outside of the Docker container.

   [debug]: ../../troubleshoot/debug/#alloy-ui
   {{< /admonition >}}
   <!-- INTERACTIVE ignore END -->

   {{< docs/ignore >}}

   > This step uses the {{< param "PRODUCT_NAME" >}} UI on `localhost` port `12345`. If you chose to run {{< param "PRODUCT_NAME" >}} in a Docker container, make sure you use the `--server.http.listen-addr=` argument. If you don't use this argument, the [debugging UI](../../troubleshoot/debug/#alloy-ui) won't be available outside of the Docker container.

   {{< /docs/ignore >}}

1. Optional: You can do a system service restart {{< param "PRODUCT_NAME" >}} and load the configuration file.

{{< docs/ignore >}}

   ```bash
   sudo systemctl reload alloy
   ```

{{< /docs/ignore >}}
<!-- INTERACTIVE ignore START -->
   {{< code >}}

   ```macos
   brew services restart grafana/grafana/alloy
   ```

   ```linux
   sudo systemctl reload alloy
   ```

   {{< /code >}}
<!-- INTERACTIVE ignore END -->

## Inspect your configuration in the {{% param "PRODUCT_NAME" %}} UI

Open [http://localhost:12345](http://localhost:12345) and click the **Graph** tab at the top.
The graph should look similar to the following:

{{< figure src="/media/docs/alloy/tutorial/Inspect-your-config-in-the-Alloy-UI-image.png" alt="Your configuration in the Alloy UI" >}}

The {{< param "PRODUCT_NAME" >}} UI shows you a visual representation of the pipeline you built with your {{< param "PRODUCT_NAME" >}} component configuration.

You can see that the components are healthy, and you are ready to explore the logs in Grafana.

<!-- INTERACTIVE page step4.md END -->

<!-- INTERACTIVE page step5.md START -->

## Log in to Grafana and explore Loki logs

Open [http://localhost:3000/explore](http://localhost:3000/explore) to access **Explore** feature in Grafana.

Select Loki as the data source and click the **Label Browser** button to select a file that {{< param "PRODUCT_NAME" >}} has sent to Loki.

Here you can see that logs are flowing through to Loki as expected, and the end-to-end configuration was successful.

{{< figure src="/media/docs/alloy/tutorial/loki-logs.png" alt="Logs reported by Alloy in Grafana" >}}

<!-- INTERACTIVE page step5.md END -->

<!-- INTERACTIVE page finish.md START -->

## Summary

You have installed and configured {{< param "PRODUCT_NAME" >}}, and sent logs from your local host to your local Grafana stack.

In the [next tutorial][], you learn more about configuration concepts and metrics.

<!-- INTERACTIVE page finish.md END -->

[MacOS Install]: ../../set-up/install/macos/
[Linux Install]: ../../set-up/install/linux/
[Run on Linux]: ../../set-up/run/linux/
[Run on MacOS]: ../../set-up/run/macos/
[local.file_match]: ../../reference/components/local/local.file_match/
[loki.write]: ../../reference/components/loki/loki.write/
[loki.source.file]: ../../reference/components/loki/loki.source.file/
[loki.process]: ../../reference/components/loki/loki.process/
[next tutorial]: ../send-metrics-to-prometheus/
[loki.process]: ../../reference/components/loki/loki.process/
