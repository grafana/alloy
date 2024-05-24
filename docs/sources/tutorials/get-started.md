---
canonical: https://grafana.com/docs/alloy/latest/tutorials/get-started/
description: Getting started with {{% param "PRODUCT_NAME" %}}
title: Get started with {{% param "PRODUCT_NAME" %}}
weight: 10
---

## Get started with {{% param "PRODUCT_NAME" %}}

This set of tutorials contains a collection of examples that build on each other to demonstrate how to configure and use [{{% param "PRODUCT_NAME" >}}][alloy].
To follow these tutorials, you need to have a basic understanding of what {{% param "PRODUCT_NAME" >}} is and telemetry collection in general.
You should also be familiar with with Prometheus and PromQL, Loki and LogQL, and basic Grafana navigation.
You don't need to know about the {{% param "PRODUCT_NAME" >}} [configuration syntax][configuration] concepts.

## Prerequisites

This first tutorial requires a Linux, Unix, or Mac environment with Docker installed.
The examples run on a single host so that you can run them on your laptop or in a Virtual Machine.
You are encouraged to try the examples using a `config.alloy` file and experiment with the examples yourself.

## Install {{% param "PRODUCT_NAME"> }} and start the service.

### Linux

Follow the instructions on the [../get-started/install/linux] Linux install page for the steps for several popular
Linux distributions.  

{{< admonition type="tip" >}}
Make sure to follow the optional install step to enable the UI, we will be referring to it in this tutorial.
{{< /admonition >}}

Once you have completed this, follow the instructions to [../get-started/run/linux](Run Grafana Alloy on Linux) using `systemctl`.

### Mac

Follow the instructions on the [../get-started/install/macos] MacOS install page for homebrew instructions. Once you have
completed this, follow the instructions to [../get-started/run/macos](Run Grafana Alloy on MacOS) which will start
{{% param "PRODUCT_NAME" %}} as a Homebrew service.

## Set up a local Grafana instance

You can use the following Docker Compose file to set up a local Grafana instance alongside Loki and Prometheus which are pre-configured as datasources. You can run and experiment with the examples on your local system. In this tutorial,
{{% param "PRODUCT_NAME"> }} will report logs to the loki running in this stack, and we can use Grafana to query and
visualize them.

```yaml
version: '3'
services:
  loki:
    image: grafana/loki:2.9.0
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
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
```

After running `docker-compose up`, open [http://localhost:3000](http://localhost:3000) in your browser to view the Grafana UI.

## Configure {{% param "PRODUCT_NAME"> }}

Alloy is configured through `config.alloy` file which contains a set of components. Components do basic things 
like identifying which logs we want to scrape, how we want to process them, and where we will send them. 
Our configuration will connect components into a workflow.

For the following steps, create a file called `config.alloy` in your current working directory. 
{{% param "PRODUCT_NAME"> }} has a 
feature that allows us to "hot reload" a configuration from a file. In a later step, we will copy this file to where
Alloy will pick it up, and be able to reload without restarting the system service.

### First Component: Log files

Put this component into the top of the `config.alloy` file:

```
local.file_match "local_files" {
    path_targets = [{"__path__" = "/var/log/*.log"}]
    sync_period = "5s"
}
```

In {{% param "PRODUCT_NAME"> }}'s configuration language, this creates a `[../../reference/components/local.file_match/](local.file_match)` named `systemlogs` with an attribute that tells {{% param "PRODUCT_NAME"> }} which files to source, and to check every 5 seconds.

### Second Component: Scraping

Put this component next in the `config.alloy` file:

```
loki.source.file "log_scrape" {
    targets    = local.file_match.local_files.targets
    forward_to = [loki.write.grafana_loki.receiver]
    tail_from_end = true
}
```

This configuration creates a `[../../reference/components/loki.source_file/](loki.source_file)` component named `log_scrape`, and
shows the pipeline concept of {{% param "PRODUCT_NAME"> }} in action:

1. It applies to the `local_files` component (its "source" or target)
2. It forwards the logs it scrapes to the "receiver" of another component called `grafana_loki` that we will define next
3. It provides extra attributes and options, in this case, we will tail log files from the end and not ingest the entire
past history

### Third Component: Write Logs to Loki

Place this component last in your configuration file:

```
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

We create a `[../../reference/components/loki.write/](loki.write)` component named `grafana_loki` that points to `http://localhost:3100/loki/api/v1/push`. This completes a simple configuration pipeline.

{{< admonition type="tip" >}}
Notice that the `basic_auth` is commented out. Our local `docker-compose` stack does not require it; we include it in this example
to show how you can configure auth for other environments. For further auth options, consult the 
`[../../reference/components/loki.write/](loki.write)` component reference.
{{< /admonition >}}

This connects directly to the Loki instance running via `docker-compose` from the earlier step.

### Reload the Configuration

Copy your local `config.alloy` file into the default configuration file location, which depends on your OS.

   {{< code >}}
   ```macOS
   sudo cp config.alloy $(brew --prefix)/etc/alloy/config.alloy
   ```

   ```linux
  sudo cp config.alloy /etc/alloy/config.alloy
   ```
   {{< /code >}}

Finally, call the reload endpoint to alert {{% param "PRODUCT_NAME"> }} to the configuration change without the need
for restarting the system service.

```bash
    curl -X POST http://localhost:12345/-/reload
```

{{< admonition type="tip" >}}
If this step does not work for you, please note that in the install instructions, enabling it requires
one extra optional step for Linux, while this is enabled by default on MacOS.
{{< /admonition >}}

## Inspect your Configuration in the {{% param "PRODUCT_NAME"> }} UI

Open `[http://localhost:12345](http://localhost:12345)` and click the Graph tab at the top, which will show
something similar to the following:

{{< figure src="/media/docs/alloy/tutorial/healthy-config.png" alt="Logs reported by Alloy in Grafana" >}}

The UI allows us to see a visual representation of the pipeline we are building with our {{% param "PRODUCT_NAME"> }}
component configuration.  We can further see that the components are healthy, and we are ready to go.

## Log into Grafana and Explore Loki Logs

Open `[http://localhost:3000/explore](http://localhost:3000/explore)` to access Grafana's Explore feature. Select Loki as 
the data source, and click the "Label Browser" button to select a file that {{% param "PRODUCT_NAME"> }} as sent to Loki.

Here we can see that logs are flowing through to Loki as expected, and the end-to-end configuration was successful!

{{< figure src="/media/docs/alloy/tutorial/loki-logs.png" alt="Logs reported by Alloy in Grafana" >}}

## Conclusion

Congratulations, you have fully installed and configured {{% param "PRODUCT_NAME"> }}, and shipped logs from your local
host to a Grafana stack. In the following tutorials, you will learn more about configuration concepts, metrics, and more
advanced log scraping.

[alloy]: https://grafana.com/docs/alloy/latest/
[configuration]: ../../concepts/configuration-syntax/
[install]: ../../get-started/install/binary/#install-alloy-as-a-standalone-binary
