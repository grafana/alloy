---
canonical: https://grafana.com/docs/alloy/latest/configure/proxy/
description: Learn how to use Grafana Alloy as a proxy or aggregation layer
menuTitle: Proxy layer
title: Use Grafana Alloy as a proxy or aggregation layer
weight: 200
---

# Use {{% param "FULL_PRODUCT_NAME" %}} as a proxy or aggregation layer

In larger deployments, you can run one or more {{< param "PRODUCT_NAME" >}} instances as proxies in front of other {{< param "PRODUCT_NAME" >}} instances.
This pattern reduces direct connections to backends such as Mimir, Loki, and Tempo, while centralizing egress traffic.
You can apply consistent relabeling, filtering, or routing logic at the proxy layer, isolating edge instances from backend changes.
This architecture also supports sharding and load distribution across multiple proxy instances.

In OpenTelemetry terminology, this deployment model is often referred to as _gateway mode_.

{{< admonition type="note" >}}
The proxy configuration described here refers to using {{< param "PRODUCT_NAME" >}} as a telemetry proxy that aggregates and forwards telemetry between instances.
It doesn't cover configuring {{< param "PRODUCT_NAME" >}} to use a corporate HTTP proxy for outbound traffic, such as `proxy_url` or `proxy_from_environment` in `remote_write` or `loki.write`.
{{< /admonition >}}

## Before you begin

Before you begin, ensure you have the following:

- A working {{< param "PRODUCT_NAME" >}} installation on your edge nodes.
- Access to deploy additional {{< param "PRODUCT_NAME" >}} instances as proxies.
- A load balancer or ingress controller for routing traffic to proxy instances.
- Network connectivity between edge instances, proxy instances, and backend services.

## Architectural patterns

You can use two primary topologies when deploying {{< param "PRODUCT_NAME" >}} as a proxy layer: push to proxy and pull from edge.

### Push to proxy

In the push-to-proxy pattern, edge {{< param "PRODUCT_NAME" >}} instances push telemetry to a pool of proxy {{< param "PRODUCT_NAME" >}} instances.
This is the most common and recommended pattern because it provides a straightforward mental model, scales cleanly in dynamic environments, and works across networks with NAT or segmented connectivity.
You can centralize authentication and routing at the proxy layer, and the pattern is compatible with both Kubernetes and VM environments.

```text
[Edge Alloy] --remote_write--> [Load Balancer] --> [Proxy Alloy x N] --> Backend
```

For metrics, edge instances push data using `prometheus.remote_write` to proxy instances running `prometheus.receive_http`.
For logs, edge instances push data using `loki.write` to proxy instances running `loki.source.api`.

#### Sticky load balancing for metrics

Sticky load balancing ensures that requests with the same identifier, such as a time series or trace ID, are consistently routed to the same backend instance.

For Prometheus `remote_write` traffic, you must ensure consistent routing per time series.
When different proxy instances receive samples for the same series, you encounter out-of-order sample errors, increased ingestion load, and write-ahead log (WAL) churn.

{{< admonition type="warning" >}}
Without sticky load balancing, metrics proxying can result in data loss or ingestion errors.
{{< /admonition >}}

To avoid these issues, configure your load balancer with sticky sessions, consistent hashing, or L4 hash-based load balancing.

### Pull from edge

In the pull-from-edge pattern, proxy {{< param "PRODUCT_NAME" >}} instances scrape targets directly, using sharding such as `hashmod` to distribute targets across instances.

This pattern works for metrics because Prometheus-style scraping supports deterministic target sharding.
For more information on distributing scrape load, refer to [Distribute Prometheus metrics scrape load](../clustering/distribute-prometheus-scrape-load/).

{{< admonition type="note" >}}
The pull model doesn't apply to logs.
Logs must use a push model.
{{< /admonition >}}

While technically possible for metrics, using proxy instances to scrape other {{< param "PRODUCT_NAME" >}} instances isn't recommended as a primary aggregation strategy.
Push-based aggregation using `prometheus.remote_write` provides clearer scaling characteristics, simpler configuration management, and better compatibility with dynamic environments.

## Configure metrics proxying

You can use the push pattern to proxy metrics between edge and proxy instances.

### Configure edge instances for metrics

Edge instances scrape metrics locally and push them to proxy instances.
The following example configuration scrapes a local Node Exporter and pushes metrics to a proxy:

```alloy
prometheus.scrape "node" {
  targets = [{
    __address__ = "localhost:9100"
  }]
  forward_to = [prometheus.remote_write.to_proxy.receiver]
}

prometheus.remote_write "to_proxy" {
  endpoint {
    url = "https://<PROXY_LOAD_BALANCER>/api/v1/metrics/write"
  }
}
```

Replace the following:

- _`<PROXY_LOAD_BALANCER>`_: The URL of your load balancer in front of the proxy {{< param "PRODUCT_NAME" >}} instances.

### Configure proxy instances for metrics

Proxy instances receive metrics from edge instances and forward them to the backend.
The following example configuration receives metrics and forwards them to Mimir:

```alloy
prometheus.receive_http "ingest" {
  http {
    listen_address = "0.0.0.0"
    listen_port    = 12345
  }
  forward_to = [prometheus.remote_write.to_backend.receiver]
}

prometheus.remote_write "to_backend" {
  endpoint {
    url = "https://<MIMIR_ENDPOINT>/api/v1/push"
  }
}
```

Replace the following:

- _`<MIMIR_ENDPOINT>`_: The URL of your Mimir instance.

You can add relabeling, filtering, or tenant routing at the proxy layer by inserting a `prometheus.relabel` component between the receiver and the remote write.

## Configure logs proxying

Logs must use a push model because you can't pull logs from other {{< param "PRODUCT_NAME" >}} instances.
Use `loki.write` on edge instances and `loki.source.api` on proxy instances.

### Configure edge instances for logs

Edge instances collect logs and push them to proxy instances.
The following example configuration collects logs from files and pushes them to a proxy:

```alloy
loki.source.file "varlogs" {
  targets = [{
    __path__ = "/var/log/*.log"
  }]
  forward_to = [loki.write.to_proxy.receiver]
}

loki.write "to_proxy" {
  endpoint {
    url = "https://<PROXY_LOAD_BALANCER>/loki/api/v1/push"
  }
}
```

Replace the following:

- _`<PROXY_LOAD_BALANCER>`_: The URL of your load balancer in front of the proxy {{< param "PRODUCT_NAME" >}} instances.

### Configure proxy instances for logs

Proxy instances receive logs from edge instances and forward them to the backend.
The following example configuration receives logs and forwards them to Loki:

```alloy
loki.source.api "ingest" {
  http {
    listen_address = "0.0.0.0"
    listen_port    = 3100
  }
  forward_to = [loki.write.to_backend.receiver]
}

loki.write "to_backend" {
  endpoint {
    url = "https://<LOKI_ENDPOINT>/loki/api/v1/push"
  }
}
```

Replace the following:

- _`<LOKI_ENDPOINT>`_: The URL of your Loki instance.

## Configure load balancing

For metrics proxying, configure your load balancer to provide consistent routing so that samples for the same time series always reach the same proxy instance.

The following example shows a simplified NGINX configuration for consistent routing:

```nginx
upstream alloy_proxies {
    hash $remote_addr consistent;
    server proxy1:12345;
    server proxy2:12345;
    server proxy3:12345;
}

server {
    listen 443 ssl;

    location /api/v1/metrics/write {
        proxy_pass http://alloy_proxies;
    }
}
```

In production, prefer hashing based on series-identifying headers or use an L4 load balancer with source hashing for better distribution.

## Signal support

The following table shows what patterns each signal type supports:

| Signal   | Push through proxy | Pull with sharding | Notes                                  |
| -------- | ------------------ | ------------------ | -------------------------------------- |
| Metrics  | Supported          | Supported          | Sticky routing required for push       |
| Logs     | Supported          | Not supported      | Push only                              |
| Traces   | Depends            | Generally no       | Use OpenTelemetry-compatible receivers |
| Profiles | Limited            | No                 | Depends on backend ingestion model     |

For traces, you typically configure edge instances to send data to an OpenTelemetry-compatible receiver, such as `otelcol.receiver.otlp`, on proxy instances.
The proxy instances then export to the backend using an appropriate exporter.
Basic trace forwarding doesn't require sticky routing, but if proxy instances run trace-derived components such as `otelcol.connector.spanmetrics` or `otelcol.connector.servicegraph`, you need consistent routing so all spans for a trace or service reach the same instance.
You can use `otelcol.exporter.loadbalancing` on the edge instances to route by trace ID or service name.
Alternatively, you can add a unique label per proxy instance and aggregate the resulting metrics in PromQL or Adaptive Metrics.

## High availability and replication

When you run multiple proxy instances, ensure consistent routing for `remote_write` traffic to prevent out-of-order errors.
Avoid double-writing unless you intentionally want data replicated across backends.

For high availability pairs, configure proper external labels such as `cluster` and `replica` so your backend can deduplicate data correctly.
Refer to your backend documentation for specific high availability deduplication requirements.
For example, Mimir requires specific label configurations to handle replica traffic.

## Operational considerations

### Capacity planning

Proxy instances handle ingestion, WAL writes for metrics, retries, and fan-out to the backend.
Monitor CPU usage, memory usage, queue depth, remote write retries, and out-of-order sample errors to ensure your proxy instances have adequate capacity.

For metrics proxying, memory usage scales with the number of active time series passing through the proxy, even if the proxy doesn't scrape targets directly.
Each proxy instance maintains series state, WAL segments, and retry queues.
High-cardinality workloads can require significant memory, and you may need to scale proxy replicas to handle large active series counts.

Resource requirements vary significantly depending on active series count, sample rate, log volume, relabeling complexity, and retry behavior.
There is no fixed ratio of series to memory or CPU that applies universally.
Always validate sizing assumptions under representative load conditions before production deployment.

Test with realistic production write volume before rollout to establish baseline resource requirements.

### Failure modes

When a proxy fails, edge instances retry sending data, which causes WAL growth on the edge instances.
Load shifts to the remaining healthy proxies, which may increase their resource usage.

When load balancing isn't sticky, you encounter out-of-order errors and ingestion amplification as samples for the same series arrive at different proxy instances.

In environments with high ingestion rates, non-sticky routing can also amplify ingestion load on the backend.
When samples for the same series arrive at multiple proxy instances, retries and duplicate handling increase overall system pressure.
Always validate your load balancer configuration before rolling out proxying in production.

### Fleet management compared to proxying

Proxying is an architecture pattern for runtime data flow.
Fleet management, which includes centralized configuration distribution, rollout control, and secret management, helps you operate large numbers of {{< param "PRODUCT_NAME" >}} instances but is separate from the proxy behavior.

You can use fleet tooling to deploy proxy instances, manage their configurations, rotate credentials, and scale horizontally.
However, proxying itself doesn't require a fleet management solution.

If you use fleet management to deploy or manage proxy instances, configure remote write endpoints and self-monitoring pipelines consistently across edge and proxy layers.
Fleet tooling controls configuration distribution and rollout, but it doesn't automatically create or enforce a proxy topology.
You must explicitly design the data flow, including which instances push to proxies and how load balancing and routing are configured.

For information about configuring a proxy for Fleet Management API traffic in restricted network environments, refer to [Custom proxy setup][fm-proxy] in the Fleet Management documentation.

[fm-proxy]: https://grafana.com/docs/grafana-cloud/send-data/fleet-management/set-up/connectivity-options/self-managed/#custom-proxy-setup

## When to use a proxy layer

A proxy layer is especially useful when you operate large fleets of {{< param "PRODUCT_NAME" >}} instances.
Without aggregation, each instance maintains its own outbound connections to backends such as Grafana Cloud, Mimir, Loki, or Tempo.
In high-scale environments, this can lead to large numbers of TCP connections from a single network boundary, increasing firewall session load, ephemeral port usage, and operational risk.
A proxy layer consolidates outbound connections and reduces connection pressure on shared network infrastructure.

Use proxy {{< param "PRODUCT_NAME" >}} instances when you need to limit backend exposure, centralize relabeling or filtering, or isolate edge instances from backend authentication changes.
A proxy layer also helps when you want to reduce outbound internet access from edge nodes or operate in segmented or air-gapped environments.

Avoid adding a proxy layer if you don't need centralized control, already use a gateway such as the Mimir or Loki gateway, or want the simplest architecture possible.

## Next steps

- [Configure clustering](../clustering/) to distribute workload across {{< param "PRODUCT_NAME" >}} instances.
- [Distribute Prometheus metrics scrape load](../clustering/distribute-prometheus-scrape-load/) using clustering and auto-distribution.
- [Deploy {{< param "PRODUCT_NAME" >}}](../../set-up/deploy/) to learn about other deployment topologies.
