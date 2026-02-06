---
canonical: https://grafana.com/docs/alloy/latest/introduction/requirements/
description: Understand supported environments, deployment expectations, and common constraints when running Grafana Alloy in production
menuTitle: Requirements
title: Requirements and expectations
weight: 250
---

# Requirements and expectations

Before you put {{< param "FULL_PRODUCT_NAME" >}} into production, it helps to have a clear picture of where it runs well, how it's usually deployed, and where people most often get surprised.

Before a first deployment, people usually want answers to a few basic questions:

- Will {{< param "PRODUCT_NAME" >}} run in my environment?
- How should I deploy it the first time?
- What kinds of constraints or trade-offs should I expect?

The guidance here focuses on the common, supported paths that work well for most users, without diving into every possible edge case.

## Design expectations

{{< param "FULL_PRODUCT_NAME" >}} makes telemetry collection explicit and predictable, even when that means exposing trade-offs that other tools try to hide.

A few design choices are worth keeping in mind:

- {{< param "PRODUCT_NAME" >}} favors explicit configuration over implicit behavior.
  You define pipelines, routing, and scaling decisions in configuration rather than relying on automatic inference.
- {{< param "PRODUCT_NAME" >}} exposes deployment and scaling choices instead of masking them.
  Changes in topology—such as switching from a DaemonSet to a centralized deployment—can affect behavior, and those effects are intentional and visible.
- {{< param "PRODUCT_NAME" >}} consolidates multiple collectors, but it doesn't replicate every default or assumption from these other collectors.
  Similar concepts may behave differently when the underlying goals differ.
- {{< param "PRODUCT_NAME" >}} prioritizes predictability over "magic" defaults.
  Understanding how components connect and how work distributes is part of operating {{< param "PRODUCT_NAME" >}} successfully.

Keeping these expectations in mind makes it easier to reason about configuration changes, scaling decisions, and observed behavior in production.

## Supported platforms

{{< param "PRODUCT_NAME" >}} runs on the following platforms:

- Linux
- Windows
- macOS
- FreeBSD

For supported architectures and version requirements, refer to [Supported platforms][supported platforms].

For setup instructions, refer to [Set up {{< param "PRODUCT_NAME" >}}][set up].

## Network requirements

{{< param "PRODUCT_NAME" >}} requires network access for its HTTP server and for sending data to backends.

### HTTP server

{{< param "PRODUCT_NAME" >}} runs an HTTP server for its UI, API, and metrics endpoints.
By default, it listens on `127.0.0.1:12345`.

For more information, refer to [HTTP endpoints][http].

### Outbound connectivity

{{< param "PRODUCT_NAME" >}} needs outbound network access to send telemetry to your backends.
Ensure firewall rules and egress rules allow connections to:

- Remote write or OTLP endpoints for metrics, such as Mimir, Prometheus, or Thanos
- Log ingestion endpoints, such as Loki, Elasticsearch, or OTLP-compatible backends
- Trace ingestion endpoints, such as Tempo, Jaeger, or OTLP-compatible backends
- Profile ingestion endpoints, such as Pyroscope

### Cluster communication

When you enable [clustering][], {{< param "PRODUCT_NAME" >}} nodes communicate over HTTP/2 using the same HTTP server port.
Each node must be reachable by other cluster members on the configured listen address.

## Permissions and access

Some {{< param "PRODUCT_NAME" >}} components interact closely with the host, container runtime, or Kubernetes APIs.
When that happens, {{< param "PRODUCT_NAME" >}} needs enough access to complete the work.

This requirement most often comes up when collecting:

- Host-level metrics, logs, traces, or profiles
- Container or runtime information
- Data that lives outside the application sandbox

Not every component can run in a fully locked-down environment.
When {{< param "PRODUCT_NAME" >}} runs with restricted permissions, certain components might fail or behave unexpectedly.

For information about running as a non-root user, refer to [Run as a non-root user][nonroot].

When you enable a component, check its documented requirements first.
Refer to the [component reference][reference] for component-specific constraints and limitations.

## Security

{{< param "PRODUCT_NAME" >}} supports TLS for secure communication.
Configure TLS in component `tls` blocks for backend connections, or use the [`--cluster.enable-tls` flag][run] for [clustered mode][clustering].
Authentication methods such as basic auth, OAuth2, and bearer tokens are configured per component.

### Secrets management

Store sensitive values like API keys and passwords outside your configuration files.
{{< param "PRODUCT_NAME" >}} supports environment variable references and integrations such as HashiCorp Vault, Kubernetes Secrets, AWS S3, and local files.

Refer to the [component documentation][reference] for specific options.

## Deployment patterns

{{< param "PRODUCT_NAME" >}} supports edge, gateway, and hybrid deployment patterns.
Refer to [How {{< param "PRODUCT_NAME" >}} works][how alloy works] for guidance on choosing the right pattern for your architecture.

For detailed setup instructions, refer to [Deploy {{< param "PRODUCT_NAME" >}}][deploy].

## Clustering and scaling behavior

Some {{< param "PRODUCT_NAME" >}} behavior depends on how you deploy it, not just on configuration.

{{< param "PRODUCT_NAME" >}} supports [clustering][] to distribute work across multiple instances.
Clustering uses a gossip protocol and consistent hashing to distribute scrape targets automatically.

{{< admonition type="note" >}}
Target auto-distribution requires enabling clustering at both the instance level and the component level.
Refer to [Clustering][clustering] for configuration details.

[clustering]: ../../get-started/clustering/
{{< /admonition >}}

A few things that often surprise users:

- More {{< param "PRODUCT_NAME" >}} instances means more meta-monitoring metrics.
- A switch between DaemonSet and centralized deployments can change observed series counts.
- Scaling clustered collectors changes how targets distribute, even when the target list stays the same.

For resource planning guidance, refer to [Estimate resource usage][estimate resource usage].

## Data durability

{{< param "PRODUCT_NAME" >}} uses a Write-Ahead Log (WAL) for metrics to handle temporary backend outages.
The WAL buffers data locally and retries sending when the backend becomes available.

For the WAL to persist across restarts, configure persistent storage using the [`--storage.path` flag][run].

{{< admonition type="note" >}}
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses buffered data on restart.
By default, {{< param "PRODUCT_NAME" >}} stores data in a temporary directory.
{{< /admonition >}}

Push-based pipelines for logs, traces, and profiles have different durability characteristics.
Refer to [component documentation][reference] for more information.

## Monitor Alloy

{{< param "PRODUCT_NAME" >}} exposes metrics about its own health and performance at the `/metrics` endpoint.

Key monitoring capabilities:

- **Internal metrics:** Controller and component metrics in Prometheus format
- **Health endpoints:** `/-/ready` and `/-/healthy` for load balancer checks
- **Debugging UI:** Visual component graph and live debugging at `/`

Refer to [Set up meta-monitoring][metamonitoring] for configuration examples.

## Component capabilities

Each {{< param "PRODUCT_NAME" >}} component has its own capabilities and limits.
Before you rely on a component in production, check:

- Which signal types it accepts and emits: metrics, logs, traces, and profiles
- Whether the component is stable or still evolving
- Whether it's a native {{< param "PRODUCT_NAME" >}} component or wraps upstream OpenTelemetry Collector functionality

Refer to the [component reference][reference] for this information.

## Troubleshoot issues

If something doesn't behave as expected after deployment:

1. Review [Troubleshooting and debugging][debug].
1. Check the [component documentation][reference].
1. Revisit deployment patterns and clustering assumptions.

## Next steps

- [Set up {{< param "PRODUCT_NAME" >}}][set up]
- [Learn about clustering][clustering]
- [Explore components][reference]

[supported platforms]: ../../set-up/supported-platforms/
[http]: ../../reference/http/
[reference]: ../../reference/
[run]: ../../reference/cli/run/
[nonroot]: ../../configure/nonroot/
[deploy]: ../../set-up/deploy/
[clustering]: ../../get-started/clustering/
[estimate resource usage]: ../../set-up/estimate-resource-usage/
[metamonitoring]: ../../collect/metamonitoring/
[debug]: ../../troubleshoot/debug/
[set up]: ../../set-up/
[how alloy works]: ../how-alloy-works/
