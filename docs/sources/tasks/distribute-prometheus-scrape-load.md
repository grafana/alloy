---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/distribute-prometheus-scrape-load/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/distribute-prometheus-scrape-load/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/distribute-prometheus-scrape-load/
- /docs/grafana-cloud/send-data/agent/flow/tasks/distribute-prometheus-scrape-load/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/getting-started/distribute-prometheus-scrape-load/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/getting-started/distribute-prometheus-scrape-load/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/getting-started/distribute-prometheus-scrape-load/
- /docs/grafana-cloud/send-data/agent/flow/getting-started/distribute-prometheus-scrape-load/
- ../getting-started/distribute-prometheus-scrape-load/ # /docs/agent/latest/flow/getting-started/distribute-prometheus-scrape-load/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/distribute-prometheus-scrape-load/
description: Learn how to distribute your Prometheus metrics scrape load
menuTitle: Distribute Prometheus metrics scrape load
title: Distribute Prometheus metrics scrape load
weight: 500
---

# Distribute Prometheus metrics scrape load

A good predictor for the size of an {{< param "PRODUCT_NAME" >}} deployment is the number of Prometheus targets each {{< param "PRODUCT_ROOT_NAME" >}} scrapes.
[Clustering][] with target auto-distribution allows a fleet of {{< param "PRODUCT_ROOT_NAME" >}}s to work together to dynamically distribute their scrape load, providing high-availability.

> **Note:** Clustering is a [beta][] feature. Beta features are subject to breaking
> changes and may be replaced with equivalent functionality that covers the same use case.

## Before you begin

- Familiarize yourself with how to [configure existing {{< param "PRODUCT_NAME" >}} installations][configure-grafana-agent].
- [Configure Prometheus metrics collection][].
- [Configure clustering][].
- Ensure that all of your clustered {{< param "PRODUCT_ROOT_NAME" >}}s have the same configuration file.

## Steps

To distribute Prometheus metrics scrape load with clustering:

1. Add the following block to all `prometheus.scrape` components, which should use auto-distribution:

   ```river
   clustering {
     enabled = true
   }
   ```

1. Restart or reload {{< param "PRODUCT_ROOT_NAME" >}}s for them to use the new configuration.

1. Validate that auto-distribution is functioning:

   1. Using the {{< param "PRODUCT_ROOT_NAME" >}} [UI][] on each {{< param "PRODUCT_ROOT_NAME" >}}, navigate to the details page for one of the `prometheus.scrape` components you modified.

   1. Compare the Debug Info sections between two different {{< param "PRODUCT_ROOT_NAME" >}} to ensure that they're not scraping the same sets of targets.

{{% docs/reference %}}
[Clustering]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/clustering.md"
[Clustering]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/clustering.md"
[beta]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/stability.md#beta"
[beta]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/stability.md#beta"
[configure-grafana-agent]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure"
[configure-grafana-agent]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure"
[Configure Prometheus metrics collection]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/collect-prometheus-metrics.md"
[Configure Prometheus metrics collection]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/collect-prometheus-metrics.md"
[Configure clustering]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure-agent-clustering.md"
[Configure clustering]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure-agent-clustering.md"
[UI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#component-detail-page"
[UI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#component-detail-page"
{{% /docs/reference %}}
