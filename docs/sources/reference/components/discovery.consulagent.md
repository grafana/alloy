---
aliases:
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/discovery.consulagent/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/discovery.consulagent/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/discovery.consulagent/
description: Learn about discovery.consulagent
title: discovery.consulagent
---

# discovery.consulagent

`discovery.consulagent` allows you to retrieve scrape targets from [Consul's Agent API][].
Only the services registered with the local agent running on the same host will be watched.
This is suitable for very large Consul clusters for which using the Catalog API would be too slow or resource intensive.

[Consul's Agent API]: https://developer.hashicorp.com/consul/api-docs/agent

## Usage

```river
discovery.consulagent "LABEL" {
  server = CONSUL_SERVER
}
```

## Arguments

The following arguments are supported:

| Name               | Type           | Description                                                                                                                               | Default          | Required |
| ------------------ | -------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| `server`           | `string`       | Host and port of the Consul Agent API.                                                                                                    | `localhost:8500` | no       |
| `token`            | `secret`       | Secret token used to access the Consul Agent API.                                                                                         |                  | no       |
| `datacenter`       | `string`       | Datacenter in which the Consul Agent is configured to run. If not provided, the datacenter will be retrieved from the local Consul Agent. |                  | no       |
| `tag_separator`    | `string`       | The string by which Consul tags are joined into the tag label.                                                                            | `,`              | no       |
| `scheme`           | `string`       | The scheme to use when talking to the Consul Agent.                                                                                       | `http`           | no       |
| `username`         | `string`       | The username to use.                                                                                                                      |                  | no       |
| `password`         | `secret`       | The password to use.                                                                                                                      |                  | no       |
| `services`         | `list(string)` | A list of services for which targets are retrieved. If omitted, all services are scraped.                                                 |                  | no       |
| `tags`             | `list(string)` | An optional list of tags used to filter nodes for a given service. Services must contain all tags in the list.                            |                  | no       |
| `refresh_interval` | `duration`     | Frequency to refresh list of containers.                                                                                                  | `"30s"`          | no       |

## Blocks

The following blocks are supported inside the definition of
`discovery.consulagent`:

| Hierarchy  | Block          | Description                                            | Required |
| ---------- | -------------- | ------------------------------------------------------ | -------- |
| tls_config | [tls_config][] | Configure TLS settings for connecting to the endpoint. | no       |

[tls_config]: #tls_config-block

### tls_config block

{{< docs/shared lookup="flow/reference/components/tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                              |
| --------- | ------------------- | -------------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the Consul Agent API. |

Each target includes the following labels:

- `__meta_consulagent_address`: the address of the target.
- `__meta_consulagent_dc`: the datacenter name for the target.
- `__meta_consulagent_health`: the health status of the service.
- `__meta_consulagent_metadata_<key>`: each node metadata key value of the target.
- `__meta_consulagent_node`: the node name defined for the target.
- `__meta_consulagent_service`: the name of the service the target belongs to.
- `__meta_consulagent_service_address`: the service address of the target.
- `__meta_consulagent_service_id`: the service ID of the target.
- `__meta_consulagent_service_metadata_<key>`: each service metadata key value of the target.
- `__meta_consulagent_service_port`: the service port of the target.
- `__meta_consulagent_tagged_address_<key>`: each node tagged address key value of the target.
- `__meta_consulagent_tags`: the list of tags of the target joined by the tag separator.

## Component health

`discovery.consulagent` is only reported as unhealthy when given an invalid
configuration. In those cases, exported fields retain their last healthy
values.

## Debug information

`discovery.consulagent` does not expose any component-specific debug information.

## Debug metrics

- `discovery_consulagent_rpc_failures_total` (Counter): The number of Consul Agent RPC call failures.
- `discovery_consulagent_rpc_duration_seconds` (SummaryVec): The duration of a Consul Agent RPC call in seconds.

## Example

<!-- TODO: Include a logging example -->
This example discovers targets from a Consul Agent for the specified list of services:

```river
discovery.consulagent "example" {
  server = "localhost:8500"
  services = [
    "service1",
    "service2",
  ]
}

prometheus.scrape "demo" {
  targets    = discovery.consul.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
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

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.consulagent` has exports that can be consumed by the following components:

- Components that consume [Targets](../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
