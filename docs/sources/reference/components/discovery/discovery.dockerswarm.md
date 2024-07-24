---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.dockerswarm/
aliases:
  - ../discovery.dockerswarm/ # /docs/alloy/latest/reference/components/discovery.dockerswarm/
description: Learn about discovery.dockerswarm
title: discovery.dockerswarm
---

# discovery.dockerswarm

`discovery.dockerswarm` allows you to retrieve scrape targets from [Docker Swarm](https://docs.docker.com/engine/swarm/key-concepts/).

## Usage

```alloy
discovery.dockerswarm "LABEL" {
  host = "DOCKER_DAEMON_HOST"
  role = "SWARM_ROLE"
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                                                   | Default | Required
-------------------------|---------------------|-------------------------------------------------------------------------------------------------------------------------------|---------|---------
`host`                   | `string`            | Address of the Docker daemon.                                                                                                 |         | yes
`role`                   | `string`            | Role of the targets to retrieve. Must be `services`, `tasks`, or `nodes`.                                                     |         | yes
`port`                   | `number`            | The port to scrape metrics from, when `role` is nodes, and for discovered tasks and services that don't have published ports. | `80`    | no
`refresh_interval`       | `duration`          | Interval at which to refresh the list of targets.                                                                             | `"60s"` | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                                                          |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                                                                                            |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                                                      | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                                                  | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                                                          |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying.                              |         | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                                                         | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                                                 |         | no

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

[arguments]: #arguments

## Blocks

The following blocks are supported inside the definition of
`discovery.dockerswarm`:

| Hierarchy           | Block             | Description                                                                        | Required |
| ------------------- | ----------------- | ---------------------------------------------------------------------------------- | -------- |
| filter              | [filter][]        | Optional filter to limit the discovery process to a subset of available resources. | no       |
| basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint.                           | no       |
| authorization       | [authorization][] | Configure generic authorization to the endpoint.                                   | no       |
| oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.                               | no       |
| oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.                             | no       |
| tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.                             | no       |

The `>` symbol indicates deeper levels of nesting. For example,
`oauth2 > tls_config` refers to a `tls_config` block defined inside
an `oauth2` block.

[filter]: #filter-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### filter block

Filters can be used to limit the discovery process to a subset of available resources.
It is possible to define multiple `filter` blocks within the `discovery.dockerswarm` block.
The list of available filters depends on the `role`:

- [services filters](https://docs.docker.com/engine/api/v1.40/#operation/ServiceList)
- [tasks filters](https://docs.docker.com/engine/api/v1.40/#operation/TaskList)
- [nodes filters](https://docs.docker.com/engine/api/v1.40/#operation/NodeList)

The following arguments can be used to configure a filter.

| Name     | Type           | Description                                | Default | Required |
| -------- | -------------- | ------------------------------------------ | ------- | -------- |
| `name`   | `string`       | Name of the filter.                        |         | yes      |
| `values` | `list(string)` | List of values associated with the filter. |         | yes      |

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                               |
| --------- | ------------------- | ----------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from Swarm. |

## Roles

The `role` attribute decides the role of the targets to retrieve.

### services

The `services` role discovers all [Swarm services](https://docs.docker.com/engine/swarm/key-concepts/#services-and-tasks) and exposes their ports as targets.
For each published port of a service, a single target is generated.
If a service has no published ports, a target per service is created using the `port` attribute defined in the arguments.

Available meta labels:

- `__meta_dockerswarm_service_id`: the ID of the service.
- `__meta_dockerswarm_service_name`: the name of the service.
- `__meta_dockerswarm_service_mode`: the mode of the service.
- `__meta_dockerswarm_service_endpoint_port_name`: the name of the endpoint port, if available.
- `__meta_dockerswarm_service_endpoint_port_publish_mode`: the publish mode of the endpoint port.
- `__meta_dockerswarm_service_label_<labelname>`: each label of the service.
- `__meta_dockerswarm_service_task_container_hostname`: the container hostname of the target, if available.
- `__meta_dockerswarm_service_task_container_image`: the container image of the target.
- `__meta_dockerswarm_service_updating_status`: the status of the service, if available.
- `__meta_dockerswarm_network_id`: the ID of the network.
- `__meta_dockerswarm_network_name`: the name of the network.
- `__meta_dockerswarm_network_ingress`: whether the network is ingress.
- `__meta_dockerswarm_network_internal`: whether the network is internal.
- `__meta_dockerswarm_network_label_<labelname>`: each label of the network.
- `__meta_dockerswarm_network_scope`: the scope of the network.

### tasks

The `tasks` role discovers all [Swarm tasks](https://docs.docker.com/engine/swarm/key-concepts/#services-and-tasks) and exposes their ports as targets.
For each published port of a task, a single target is generated.
If a task has no published ports, a target per task is created using the `port` attribute defined in the arguments.

Available meta labels:

- `__meta_dockerswarm_container_label_<labelname>`: each label of the container.
- `__meta_dockerswarm_task_id`: the ID of the task.
- `__meta_dockerswarm_task_container_id`: the container ID of the task.
- `__meta_dockerswarm_task_desired_state`: the desired state of the task.
- `__meta_dockerswarm_task_slot`: the slot of the task.
- `__meta_dockerswarm_task_state`: the state of the task.
- `__meta_dockerswarm_task_port_publish_mode`: the publish mode of the task port.
- `__meta_dockerswarm_service_id`: the ID of the service.
- `__meta_dockerswarm_service_name`: the name of the service.
- `__meta_dockerswarm_service_mode`: the mode of the service.
- `__meta_dockerswarm_service_label_<labelname>`: each label of the service.
- `__meta_dockerswarm_network_id`: the ID of the network.
- `__meta_dockerswarm_network_name`: the name of the network.
- `__meta_dockerswarm_network_ingress`: whether the network is ingress.
- `__meta_dockerswarm_network_internal`: whether the network is internal.
- `__meta_dockerswarm_network_label_<labelname>`: each label of the network.
- `__meta_dockerswarm_network_label`: each label of the network.
- `__meta_dockerswarm_network_scope`: the scope of the network.
- `__meta_dockerswarm_node_id`: the ID of the node.
- `__meta_dockerswarm_node_hostname`: the hostname of the node.
- `__meta_dockerswarm_node_address`: the address of the node.
- `__meta_dockerswarm_node_availability`: the availability of the node.
- `__meta_dockerswarm_node_label_<labelname>`: each label of the node.
- `__meta_dockerswarm_node_platform_architecture`: the architecture of the node.
- `__meta_dockerswarm_node_platform_os`: the operating system of the node.
- `__meta_dockerswarm_node_role`: the role of the node.
- `__meta_dockerswarm_node_status`: the status of the node.

The `__meta_dockerswarm_network_*` meta labels are not populated for ports which are published with mode=host.

### nodes

The `nodes` role is used to discover [Swarm nodes](https://docs.docker.com/engine/swarm/key-concepts/#nodes).

Available meta labels:

- `__meta_dockerswarm_node_address`: the address of the node.
- `__meta_dockerswarm_node_availability`: the availability of the node.
- `__meta_dockerswarm_node_engine_version`: the version of the node engine.
- `__meta_dockerswarm_node_hostname`: the hostname of the node.
- `__meta_dockerswarm_node_id`: the ID of the node.
- `__meta_dockerswarm_node_label_<labelname>`: each label of the node.
- `__meta_dockerswarm_node_manager_address`: the address of the manager component of the node.
- `__meta_dockerswarm_node_manager_leader`: the leadership status of the manager component of the node (true or false).
- `__meta_dockerswarm_node_manager_reachability`: the reachability of the manager component of the node.
- `__meta_dockerswarm_node_platform_architecture`: the architecture of the node.
- `__meta_dockerswarm_node_platform_os`: the operating system of the node.
- `__meta_dockerswarm_node_role`: the role of the node.
- `__meta_dockerswarm_node_status`: the status of the node.

## Component health

`discovery.dockerswarm` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.dockerswarm` does not expose any component-specific debug information.

## Debug metrics

`discovery.dockerswarm` does not expose any component-specific debug metrics.

## Example

This example discovers targets from Docker Swarm tasks:

```alloy
discovery.dockerswarm "example" {
  host = "unix:///var/run/docker.sock"
  role = "tasks"

  filter {
    name = "id"
    values = ["0kzzo1i0y4jz6027t0k7aezc7"]
  }

  filter {
    name = "desired-state"
    values = ["running", "accepted"]
  }
}

prometheus.scrape "demo" {
  targets    = discovery.dockerswarm.example.targets
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

`discovery.dockerswarm` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
