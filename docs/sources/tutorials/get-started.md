---
canonical: https://grafana.com/docs/alloy/latest/tutorials/get-started/
description: Getting started with Alloy
title: Get started with Alloy
weight: 10
---

## Get started with {{% param "PRODUCT_NAME" %}}

This tutorial shows you how to configure Alloy to collect logs from your local machine and send them to Loki, running in a local Grafana stack.
This process will enable you to query and visualize the logs sent to Loki using the Grafana dashboard.

To follow this tutorial, you must have a basic understanding of Alloy and telemetry collection in general.
You should also be familiar with Prometheus and PromQL, Loki and LogQL, and basic Grafana navigation.
You don't need to know about the {{< param "PRODUCT_NAME" >}} [configuration syntax][configuration] concepts.

## Prerequisites

This tutorial requires a Linux or macOS environment with Docker installed.

## Install {{% param "PRODUCT_NAME" %}} and start the service

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

Once the local Grafana instance is set up, the next step is to configure {{< param "PRODUCT_NAME" >}}.
You use components in the `config.alloy` file to tell {{< param "PRODUCT_NAME" >}} which logs you want to scrape, how you want to process that data, and where you want the data sent.

The examples run on a single host so that you can run them on your laptop or in a Virtual Machine.
You can try the examples using a `config.alloy` file and experiment with the examples yourself.

For the following steps, create a file called `config.alloy` in your current working directory. 
If you have enabled the {{< param "PRODUCT_NAME" >}} UI, you can "hot reload" a configuration from a file.
In a later step, you will copy this file to where {{< param "PRODUCT_NAME" >}} will pick it up, and be able to reload without restarting the system service.

### First component: Log files

Paste this component into the top of the `config.alloy` file:

```alloy
local.file_match "local_files" {
    path_targets = [{"__path__" = "/var/log/*.log"}]
    sync_period = "5s"
}
```

This component creates a [local.file_match][] component named `local_files` with an attribute that tells {{< param "PRODUCT_NAME" >}} which files to source, and to check for new files every 5 seconds.

### Second component: Scraping

Paste this component next in the `config.alloy` file:

```alloy
loki.source.file "log_scrape" {
    targets    = local.file_match.local_files.targets
    forward_to = [loki.write.grafana_loki.receiver]
    tail_from_end = true
}
```

This configuration creates a [loki.source.file][] component named `log_scrape`, and shows the pipeline concept of {{< param "PRODUCT_NAME" >}} in action. The `log_scrape` component does the following:

1. It connects to the `local_files` component (its "source" or target).
1. It forwards the logs it scrapes to the "receiver" of another component called `grafana_loki` that you will define next.
1. It provides extra attributes and options, in this case, you will tail log files from the end and not ingest the entire past history.

### Third component: Write logs to Loki

Paste this component last in your configuration file:

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

This last component creates a [loki.write][] component named `grafana_loki` that points to `http://localhost:3100/loki/api/v1/push`.
This completes the simple configuration pipeline.

{{< admonition type="tip" >}}
The `basic_auth` is commented out because the local `docker compose` stack doesn't require it. 
It is included in this example to show how you can configure authorization for other environments.
For further authorization options, refer to the [loki.write][] component reference.

{{< /admonition >}}

This connects directly to the Loki instance running in the Docker container.

## Reload the configuration

Copy your local `config.alloy` file into the default configuration file location, which depends on your OS.

{{< code >}}

```macos
sudo cp config.alloy $(brew --prefix)/etc/alloy/config.alloy
```

```linux
sudo cp config.alloy /etc/alloy/config.alloy
```

{{< /code >}}

Finally, call the reload endpoint to notify {{< param "PRODUCT_NAME" >}} to the configuration change without the need
for restarting the system service.

```bash
    curl -X POST http://localhost:12345/-/reload
```

{{< admonition type="tip" >}}
This step uses the Alloy UI, which is exposed on `localhost` port `12345`.
If you chose to run Alloy in a Docker container, make sure you use the `--server.http.listen-addr=0.0.0.0:12345` argument.
If you don’t use this argument, the [debugging UI][debug] won’t be available outside of the Docker container.

[debug]: ../../tasks/debug/#alloy-ui
{{< /admonition >}}

The alternative to using this endpoint is to reload the {{< param "PRODUCT_NAME" >}} configuration, which can
be done as follows:

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

{{< figure src="/media/docs/alloy/tutorial/healthy-config.png" alt="Logs reported by Alloy in Grafana" >}}

The UI allows you to see a visual representation of the pipeline you built with your {{< param "PRODUCT_NAME" >}} component configuration.
We can see that the components are healthy, and you are ready to go.

## Log in to Grafana and explore Loki logs

Open [http://localhost:3000/explore] to access Explore feature in Grafana.
Select Loki as the data source and click the "Label Browser" button to select a file that {{< param "PRODUCT_NAME" >}} has sent to Loki.

Here you can see that logs are flowing through to Loki as expected, and the end-to-end configuration was successful!

{{< figure src="/media/docs/alloy/tutorial/loki-logs.png" alt="Logs reported by Alloy in Grafana" >}}

## Conclusion

Congratulations, you have fully installed and configured {{< param "PRODUCT_NAME" >}}, and sent logs from your local host to a Grafana stack.
In the following tutorials, you will learn more about configuration concepts, metrics, and more advanced log scraping.

[http://localhost:3000/explore]: http://localhost:3000/explore
[http://localhost:12345]: http://localhost:12345
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