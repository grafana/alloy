---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.cadvisor/
aliases:
  - ../prometheus.exporter.cadvisor/ # /docs/alloy/latest/reference/components/prometheus.exporter.cadvisor/
description: Learn about the prometheus.exporter.cadvisor
title: prometheus.exporter.cadvisor
---

# prometheus.exporter.cadvisor
The `prometheus.exporter.cadvisor` component exposes container metrics using
[cAdvisor](https://github.com/google/cadvisor).

## Usage

```alloy
prometheus.exporter.cadvisor "LABEL" {
}
```

## Arguments
The following arguments can be used to configure the exporter's behavior.
All arguments are optional. Omitted fields take their default values.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`store_container_labels` | `bool` | Whether to convert container labels and environment variables into labels on Prometheus metrics for each container. | `true` | no
`allowlisted_container_labels` | `list(string)` | Allowlist of container labels to convert to Prometheus labels. | `[]`  | no
`env_metadata_allowlist` | `list(string)` | Allowlist of environment variable keys matched with a specified prefix that needs to be collected for containers. | `[]` | no
`raw_cgroup_prefix_allowlist` | `list(string)` | List of cgroup path prefixes that need to be collected, even when docker_only is specified. | `[]` | no
`perf_events_config` | `string` | Path to a JSON file containing the configuration of perf events to measure. | `""` | no
`resctrl_interval` | `duration` | Interval to update resctrl mon groups. | `0` | no
`disabled_metrics` | `list(string)` | List of metrics to be disabled which, if set, overrides the default disabled metrics. | (see below) | no
`enabled_metrics` | `list(string)` | List of metrics to be enabled which, if set, overrides disabled_metrics. | `[]` | no
`storage_duration` | `duration` | Length of time to keep data stored in memory. | `2m` | no
`containerd_host` | `string` | Containerd endpoint. | `/run/containerd/containerd.sock` | no
`containerd_namespace` | `string` | Containerd namespace. | `k8s.io` | no
`docker_host` | `string` | Docker endpoint. | `unix:///var/run/docker.sock` | no
`use_docker_tls` | `bool` | Use TLS to connect to docker. | `false` | no
`docker_tls_cert` | `string` | Path to client certificate for TLS connection to docker. | `cert.pem` | no
`docker_tls_key` | `string` | Path to private key for TLS connection to docker. | `key.pem` | no
`docker_tls_ca` | `string` | Path to a trusted CA for TLS connection to docker. | `ca.pem` | no
`docker_only` | `bool` | Only report docker containers in addition to root stats. | `false` | no
`disable_root_cgroup_stats` | `bool` | Disable collecting root Cgroup stats. | `false` | no

For `allowlisted_container_labels` to take effect, `store_container_labels` must be set to `false`.

`env_metadata_allowlist` is only supported for containerd and Docker runtimes.

If `perf_events_config` is not set, measurement of perf events is disabled.

A `resctrl_interval` of `0` disables updating mon groups.

The values for `enabled_metrics` and `disabled_metrics` do not correspond to
Prometheus metrics, but to kinds of metrics that should (or shouldn't) be
exposed. The full list of values that can be used is 
```
"cpu", "sched", "percpu", "memory", "memory_numa", "cpuLoad", "diskIO", "disk",
"network", "tcp", "advtcp", "udp", "app", "process", "hugetlb", "perf_event",
"referenced_memory", "cpu_topology", "resctrl", "cpuset", "oom_event"
```

By default the following metric kinds are disabled: `"memory_numa", "tcp", "udp", "advtcp", "process", "hugetlb", "referenced_memory", "cpu_topology", "resctrl", "cpuset"`

## Blocks

The `prometheus.exporter.cadvisor` component does not support any blocks, and is configured
fully through arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.cadvisor` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.cadvisor` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.cadvisor` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.cadvisor`:

```alloy
prometheus.exporter.cadvisor "example" {
  docker_host = "unix:///var/run/docker.sock"

  storage_duration = "5m"
}

// Configure a prometheus.scrape component to collect cadvisor metrics.
prometheus.scrape "scraper" {
  targets    = prometheus.exporter.cadvisor.example.targets
  forward_to = [ prometheus.remote_write.demo.receiver ]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.cadvisor` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
