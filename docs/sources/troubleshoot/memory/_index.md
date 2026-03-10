---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/
description: Learn how to troubleshoot memory issues in Grafana Alloy
title: Troubleshoot memory issues
menuTitle: Memory issues
weight: 200
---

# Troubleshoot memory issues

Most memory issues in {{< param "PRODUCT_NAME" >}} stem from misconfigured resource limits, WAL replay during startup, or back pressure when remote endpoints can't accept data fast enough.

Common symptoms include:

- Container restarts with `OOMKilled`, common in Kubernetes
- Memory spikes immediately after restart
- Memory grows steadily and never drops
- Memory remains high after traffic decreases

## Understand {{% param "PRODUCT_NAME" %}} memory behavior

{{< param "PRODUCT_NAME" >}} uses [`automemlimit`][automemlimit] to automatically set the [`GOMEMLIMIT`][env-vars] environment variable to 90% of the container memory limit.
`GOMEMLIMIT` is a soft limit.
When memory approaches this threshold, the Go runtime runs garbage collection more aggressively to try to stay under it.
Memory usage can still temporarily exceed this limit if the runtime can't free memory quickly enough.

To override the default, set `GOMEMLIMIT` manually.
Refer to [Environment variables][env-vars] for more information.

## Identify the source

Memory issues often have multiple contributing factors.
Start by identifying which category matches your symptoms:

- **`OOMKilled` or startup crashes**: Refer to [Kubernetes memory issues][kubernetes] for resource configuration and persistent storage guidance.
- **Memory spikes after restart or write-ahead log (WAL) issues**: Refer to [Prometheus component memory issues][prometheus] for WAL replay and retention configuration.
- **Back pressure from HTTP ingestion sources**: Refer to [Loki component memory issues][loki] for `loki.source.api` and `loki.source.firehose` troubleshooting.
- **Gradual memory growth**: Review endpoint latency and internal queue metrics. Refer to [Monitor components][monitor-components] for more information.

## Capture profiles for diagnosis

Heap and goroutine profiles help identify what consumes memory.
Collect two profiles several minutes apart and compare them to identify allocations that continue to grow over time.
Refer to [Profile resource consumption][profile] for more information.

## Report a potential memory leak

If local troubleshooting and profiling doesn't identify the root cause, collect the following information and [open an issue][alloy-issues]:

- [Support bundle][support-bundle]
- Profiles: heap and goroutine
- {{< param "PRODUCT_NAME" >}} configuration
- Kubernetes Pod specification

Redact any sensitive information before attaching files.

[env-vars]: ../../reference/cli/environment-variables/#gomemlimit
[automemlimit]: https://github.com/KimMachineGun/automemlimit
[profile]: ../profile/
[support-bundle]: ../support_bundle/
[alloy-issues]: https://github.com/grafana/alloy/issues/
[monitor-components]: ../component_metrics/
[kubernetes]: ./kubernetes/
[prometheus]: ./prometheus/
[loki]: ./loki/
