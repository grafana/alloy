---
aliases:
- ./chaining/
- /docs/grafana-cloud/agent/flow/tutorials/chaining/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tutorials/chaining/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tutorials/chaining/
- /docs/grafana-cloud/send-data/agent/flow/tutorials/chaining/
canonical: https://grafana.com/docs/agent/latest/flow/tutorials/chaining/
description: Learn how to chain Prometheus components
menuTitle: Chain Prometheus components
title: Chain Prometheus components
weight: 400
---

# Chain Prometheus components

This tutorial shows how to use [multiple-inputs.river][] to send data to several different locations. This tutorial uses the same base as [Filtering metrics][].

A new concept introduced in Flow is chaining components together in a composable pipeline. This promotes the reusability of components while offering flexibility.

## Prerequisites

* [Docker](https://www.docker.com/products/docker-desktop)

## Run the example

Run the following

```bash
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/runt.sh -O && bash ./runt.sh multiple-inputs.river
```

The `runt.sh` script does:

1. Downloads the configurations necessary for Mimir, Grafana, and {{< param "PRODUCT_ROOT_NAME" >}}.
2. Downloads the docker image for {{< param "PRODUCT_ROOT_NAME" >}} explicitly.
3. Runs the `docker-compose up` command to bring all the services up.

Allow {{< param "PRODUCT_ROOT_NAME" >}} to run for two minutes, then navigate to [Grafana][] to see {{< param "PRODUCT_ROOT_NAME" >}} scrape metrics. The [node_exporter][] metrics also show up now.

There are two scrapes each sending metrics to one filter. Note the `job` label lists the full name of the scrape component.

## Multiple outputs

```river
prometheus.scrape "agent" {
    targets    = [{"__address__" = "localhost:12345"}]
    forward_to = [prometheus.relabel.service.receiver]
}

prometheus.exporter.unix "default" {
    set_collectors = ["cpu", "diskstats"]
}

prometheus.scrape "unix" {
    targets    = prometheus.exporter.unix.default.targets
    forward_to = [prometheus.relabel.service.receiver]
}

prometheus.relabel "service" {
    rule {
        source_labels = ["__name__"]
        regex         = "(.+)"
        replacement   = "api_server"
        target_label  = "service"
    }
    forward_to = [prometheus.remote_write.prom.receiver]
}

prometheus.remote_write "prom" {
    endpoint {
        url = "http://mimir:9009/api/v1/push"
    }
}
```

In the Flow block, `prometheus.relabel.service` is being forwarded metrics from two sources `prometheus.scrape.agent` and `prometheus.exporter.unix.default`. This allows for a single relabel component to be used with any number of inputs.

## Adding another relabel

In `multiple-input.river` add a new `prometheus.relabel` component that adds a `version` label with the value of `v2` to all metrics after the `prometheus.relabel.service`.

![Add a new label with the value v2](/media/docs/agent/screenshot-grafana-agent-chaining-scrape-v2.png)

[multiple-inputs.river]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/tutorials/assets/flow_configs/multiple-inputs.river
[Grafana]: http://localhost:3000/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Mimir%22,%7B%22refId%22:%22A%22,%22instant%22:true,%22range%22:true,%22exemplar%22:true,%22expr%22:%22agent_build_info%7B%7D%22%7D%5D
[node_exporter]: http://localhost:3000/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Mimir%22,%7B%22refId%22:%22A%22,%22instant%22:true,%22range%22:true,%22exemplar%22:true,%22expr%22:%22node_cpu_seconds_total%22%7D%5D

{{% docs/reference %}}
[Filtering metrics]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tutorials/filtering-metrics.md"
[Filtering metrics]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tutorials/filtering-metrics.md"
{{% /docs/reference %}}
