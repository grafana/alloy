---
aliases:
- ./filtering-metrics/
- /docs/grafana-cloud/agent/flow/tutorials/filtering-metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tutorials/filtering-metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tutorials/filtering-metrics/
- /docs/grafana-cloud/send-data/agent/flow/tutorials/filtering-metrics/
canonical: https://grafana.com/docs/agent/latest/flow/tutorials/filtering-metrics/
description: Learn how to filter Prometheus metrics
menuTitle: Filter Prometheus metrics
title: Filter Prometheus metrics
weight: 300
---

# Filter Prometheus metrics

In this tutorial, you'll add a new component [prometheus.relabel][] using [relabel.river][] to filter metrics. This tutorial uses the same base as [Collecting Prometheus metrics][].

## Prerequisites

* [Docker][]

## Run the example

The `prometheus.relabel` component is used to drop, add, or filter metrics.

Run the following:

```bash
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/runt.sh -O && bash ./runt.sh relabel.river
```

The `runt.sh` script does:

1. Downloads the configurations necessary for Mimir, Grafana and {{< param "PRODUCT_ROOT_NAME" >}}.
1. Downloads the docker image for {{< param "PRODUCT_ROOT_NAME" >}} explicitly.
1. Runs the docker-compose up command to bring all the services up.


Allow {{< param "PRODUCT_ROOT_NAME" >}} to run for two minutes, then navigate to [Grafana][] page and the `service` label will be there with the `api_server` value.

![Dashboard showing api_server](/media/docs/agent/screenshot-grafana-agent-filtering-metrics-filter.png)

## What's happening?

1. The Prometheus scraper is sending the metrics to the filter.
1. The filter is adding a new label named `service` with the value `api_server`.
1. The filter is then sending metrics to the remote_write endpoint.

## Update the service value

Open the `relabel.river` file that was downloaded and change the name of the service to `api_server_v2`, then run `bash ./runt.sh relabel.river`. Allow {{< param "PRODUCT_ROOT_NAME" >}} to run for two minutes, then navigate to the [Grafana][] page, and the new label will be updated. The old value `api_server` may still show up in the graph but hovering over the lines will show that that value stopped being scraped and was replaced with `api_server_v2`.

![Updated dashboard showing api_server_v2](/media/docs/agent/screenshot-grafana-agent-filtering-metrics-transition.png)


[Docker]: https://www.docker.com/products/docker-desktop
[Grafana]: http://localhost:3000/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Mimir%22,%7B%22refId%22:%22A%22,%22instant%22:true,%22range%22:true,%22exemplar%22:true,%22expr%22:%22agent_build_info%7B%7D%22%7D%5D
[relabel.river]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/tutorials/assets/flow_configs/relabel.river

{{% docs/reference %}}
[prometheus.relabel]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.relabel.md"
[prometheus.relabel]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.relabel.md"
[Collecting Prometheus metrics]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tutorials/collecting-prometheus-metrics.md"
[Collecting Prometheus metrics]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tutorials/collecting-prometheus-metrics.md"
{{% /docs/reference %}}
