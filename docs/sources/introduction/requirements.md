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

{{< param "PRODUCT_NAME" >}} runs on the following [platforms][supported platforms]:

- **Linux:** AMD64, ARM64
- **Windows:** Server 2016 or later, Windows 10 or later, AMD64
- **macOS:** 10.13 or later, Intel and Apple Silicon
- **FreeBSD:** AMD64

## Network requirements

{{< param "PRODUCT_NAME" >}} requires network access for its HTTP server and for sending data to backends.

### HTTP server

{{< param "PRODUCT_NAME" >}} runs an HTTP server for its UI, API, and metrics endpoints.
By default, it listens on `127.0.0.1:12345`.

The HTTP server exposes:

- `/` - Debugging UI
- `/metrics` - Internal metrics in Prometheus format
- `/-/ready` - Readiness check
- `/-/healthy` - Health check
- `/-/reload` - Configuration reload endpoint

In Kubernetes, the Helm chart defaults to `0.0.0.0:12345` to allow access from other pods.

Refer to [HTTP endpoints][http] for the complete list.

### Outbound connectivity

{{< param "PRODUCT_NAME" >}} needs outbound network access to send telemetry to your backends.
Ensure firewall rules and egress rules allow connections to:

- Remote write endpoints for metrics
- Loki endpoints for logs
- Tempo or OTLP endpoints for traces
- Pyroscope endpoints for profiles

### Cluster communication

When you enable clustering, {{< param "PRODUCT_NAME" >}} nodes communicate over HTTP/2 using the same HTTP server port.
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

When you enable a component, check its documented requirements first.
Refer to the [component reference][reference] for component-specific constraints and limitations.

## Security

{{< param "PRODUCT_NAME" >}} supports TLS and authentication for secure communication.

### TLS for backends

Most exporter and receiver components support TLS configuration for connecting to backends.
Configure TLS in the component's `tls` block or client configuration.

### Cluster TLS

When running in clustered mode, you can enable TLS for peer-to-peer communication using the `--cluster.enable-tls` flag and related certificate flags.

### Secrets management

Store sensitive values like API keys and passwords outside your configuration files.
{{< param "PRODUCT_NAME" >}} supports environment variable references and can integrate with secret management solutions.

Refer to component documentation for specific authentication options.

## Deployment patterns

{{< param "PRODUCT_NAME" >}} supports edge, gateway, and hybrid deployment patterns.
Refer to [How {{< param "PRODUCT_NAME" >}} works][how alloy works] for guidance on choosing the right pattern for your architecture.

For detailed setup instructions, refer to [Deploy {{< param "PRODUCT_NAME" >}}][deploy].

### Run outside Kubernetes

{{< param "PRODUCT_NAME" >}} also runs outside Kubernetes:

- **Linux:** Run as a systemd service. Refer to [Run on Linux][run linux].
- **Windows:** Run as a Windows service. Refer to [Run on Windows][run windows].
- **macOS:** Run as a launchd service or standalone binary. Refer to [Run on macOS][run macos].
- **Docker:** Run as a container. Refer to [Run in a Docker container][docker].

## Clustering and scaling behavior

Some {{< param "PRODUCT_NAME" >}} behavior depends on how you deploy it, not just on configuration.

{{< param "PRODUCT_NAME" >}} supports [clustering][] to distribute work across multiple instances.
Clustering uses a gossip protocol and consistent hashing to distribute scrape targets automatically.

{{< admonition type="note" >}}
Target auto-distribution only works when you explicitly enable it.
Add a `clustering { enabled = true }` block to components that should participate in work distribution.
Without this block, each instance processes all targets independently.
{{< /admonition >}}

A few things that often surprise users:

- More {{< param "PRODUCT_NAME" >}} instances means more meta-monitoring metrics.
- A switch between DaemonSet and centralized deployments can change observed series counts.
- Scaling clustered collectors changes how targets distribute, even when the target list stays the same.

Validate changes in deployment topology as part of any rollout.

For resource planning, a common rule of thumb for Prometheus metrics collection is approximately 10 KB of memory per active series.
Refer to [Estimate resource usage][estimate resource usage] for detailed guidance.

## Kubernetes integrations

{{< param "PRODUCT_NAME" >}} integrates well with Kubernetes, but it doesn't automatically behave like every Prometheus-based setup.

{{< param "PRODUCT_NAME" >}} supports the following Prometheus Operator CRDs through dedicated components:

- `ServiceMonitor` through `prometheus.operator.servicemonitors`
- `PodMonitor` through `prometheus.operator.podmonitors`
- `Probe` through `prometheus.operator.probes`
- `ScrapeConfig` through `prometheus.operator.scrapeconfigs`

Generic Kubernetes discovery components don't interpret these CRDs on their own.
When you use Prometheus Operator resources, configure the corresponding `prometheus.operator.*` components instead of relying on generic discovery.

## Data durability

{{< param "PRODUCT_NAME" >}} uses a Write-Ahead Log (WAL) for Prometheus metrics to handle temporary backend outages.
The WAL buffers data locally and retries sending when the backend becomes available.

For the WAL to persist across restarts, configure persistent storage using the `--storage.path` flag.
In Kubernetes, use a StatefulSet with a PersistentVolumeClaim.

{{< admonition type="note" >}}
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses buffered data on restart.
By default, {{< param "PRODUCT_NAME" >}} stores data in a temporary directory.
{{< /admonition >}}

Push-based pipelines for logs, traces, and profiles have different durability characteristics.
Refer to component documentation for specific delivery guarantees.

## Monitor Alloy

{{< param "PRODUCT_NAME" >}} exposes metrics about its own health and performance at the `/metrics` endpoint.

Key monitoring capabilities:

- **Internal metrics:** Controller and component metrics in Prometheus format
- **Health endpoints:** `/-/ready` and `/-/healthy` for load balancer checks
- **Debugging UI:** Visual component graph and live debugging at `/`

Use the `prometheus.exporter.self` component to scrape {{< param "PRODUCT_NAME" >}}'s own metrics and forward them to your monitoring backend.

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
1. Check the component documentation.
1. Revisit deployment patterns and clustering assumptions.

Most issues come down to mismatched expectations rather than incorrect configuration.

## Next steps

- [Set up {{< param "PRODUCT_NAME" >}}][set up]
- [Learn about clustering][clustering]
- [Explore components][reference]

[supported platforms]: ../../set-up/supported-platforms/
[http]: ../../reference/http/
[reference]: ../../reference/
[deploy]: ../../set-up/deploy/
[run linux]: ../../set-up/run/linux/
[run windows]: ../../set-up/run/windows/
[run macos]: ../../set-up/run/macos/
[docker]: ../../set-up/install/docker/
[clustering]: ../../get-started/clustering/
[estimate resource usage]: ../../set-up/estimate-resource-usage/
[metamonitoring]: ../../collect/metamonitoring/
[debug]: ../../troubleshoot/debug/
[set up]: ../../set-up/
[how alloy works]: ../how-alloy-works/
