---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.serverset/
aliases:
  - ../discovery.serverset/ # /docs/alloy/latest/reference/components/discovery.serverset/
description: Learn about discovery.serverset
labels:
  stage: general-availability
  products:
    - oss
title: discovery.serverset
---

# `discovery.serverset`

`discovery.serverset` discovers [Serversets][] stored in Zookeeper and exposes them as targets.
Serversets are commonly used by [Finagle][] and [Aurora][].

[Serversets]: https://github.com/twitter/finagle/tree/develop/finagle-serversets
[Finagle]: https://twitter.github.io/finagle/
[Aurora]: https://aurora.apache.org/

## Usage

```alloy
discovery.serverset "<LABEL>" {
    servers = "<SERVERS_LIST>"
    paths   = "<ZOOKEEPER_PATHS_LIST>"
}
```

Serverset data stored in Zookeeper must be in JSON format.
The Thrift format isn't supported.

## Arguments

You can use the following arguments with `discovery.serverset`:

| Name      | Type           | Description                                      | Default | Required |
| --------- | -------------- | ------------------------------------------------ | ------- | -------- |
| `paths`   | `list(string)` | The Zookeeper paths to discover Serversets from. |         | yes      |
| `servers` | `list(string)` | The Zookeeper servers to connect to.             |         | yes      |
| `timeout` | `duration`     | The Zookeeper session timeout                    | `"10s"` | no       |

## Blocks

The `discovery.serverset` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                    |
| --------- | ------------------- | ------------------------------ |
| `targets` | `list(map(string))` | The set of targets discovered. |

The following metadata labels are available on targets during relabeling:

* `__meta_serverset_endpoint_host_<endpoint>`: The host of the given endpoint.
* `__meta_serverset_endpoint_host`: The host of the default endpoint.
* `__meta_serverset_endpoint_port_<endpoint>`: The port of the given endpoint.
* `__meta_serverset_endpoint_port`: The port of the default endpoint.
* `__meta_serverset_path`: The full path to the serverset member node in Zookeeper.
* `__meta_serverset_shard`: The shard number of the member.
* `__meta_serverset_status`: The status of the member.

## Component health

`discovery.serverset` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.serverset` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.serverset` doesn't expose any component-specific debug metrics.

## Example

The configuration below connects to one of the Zookeeper servers, either `zk1`, `zk2`, or `zk3`, and discovers JSON Serversets at paths `/path/to/znode1` and `/path/to/znode2`.
The discovered targets are scraped by the `prometheus.scrape.default` component and forwarded to the `prometheus.remote_write.default` component, which sends the samples to specified `remote_write` URL.

```alloy
discovery.serverset "zookeeper" {
    servers = ["zk1", "zk2", "zk3"]
    paths   = ["/path/to/znode1", "/path/to/znode2"]
    timeout = "30s"
}

prometheus.scrape "default" {
    targets    = discovery.serverset.zookeeper.targets
    forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
    endpoint {
        url = "http://remote-write-url1"
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.serverset` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
