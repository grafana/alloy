---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.docker/
aliases:
  - ../discovery.docker/ # /docs/alloy/latest/reference/components/discovery.docker/
description: Learn about discovery.docker
title: discovery.docker
---

# discovery.docker

`discovery.docker` discovers [Docker Engine][] containers and exposes them as targets.

[Docker Engine]: https://docs.docker.com/engine/

## Usage

```alloy
discovery.docker "LABEL" {
  host = DOCKER_ENGINE_HOST
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default       | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|---------------|---------
`host`                   | `string`            | Address of the Docker Daemon to connect to.                                                      |               | yes
`port`                   | `number`            | Port to use for collecting metrics when containers don't have any port mappings.                 | `80`          | no
`host_networking_host`   | `string`            | Host to use if the container is in host networking mode.                                         | `"localhost"` | no
`refresh_interval`       | `duration`          | Frequency to refresh list of containers.                                                         | `"1m"`        | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |               | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |               | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`        | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`        | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |               | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |               | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`       | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |               | no

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of
`discovery.docker`:

Hierarchy           | Block             | Description                                              | Required
--------------------|-------------------|----------------------------------------------------------|---------
filter              | [filter][]        | Filters discoverable resources.                          | no
basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting.
For example, `oauth2 > tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[filter]: #filter-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### filter block

The `filter` block configures a filter to pass to the Docker Engine to limit the amount of containers returned.
The `filter` block can be specified multiple times to provide more than one filter.

Name     | Type           | Description                   | Default | Required
---------|----------------|-------------------------------|---------|---------
`name`   | `string`       | Filter name to use.           |         | yes
`values` | `list(string)` | Values to pass to the filter. |         | yes

Refer to [List containers][List containers] from the Docker Engine API documentation for the list of supported filters and their meaning.

[List containers]: https://docs.docker.com/engine/api/v1.41/#tag/Container/operation/ContainerList

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

Name      | Type                | Description
----------|---------------------|---------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the docker API.

Each target includes the following labels:

* `__meta_docker_container_id`: ID of the container.
* `__meta_docker_container_name`: Name of the container.
* `__meta_docker_container_network_mode`: Network mode of the container.
* `__meta_docker_container_label_<labelname>`: Each label from the container.
* `__meta_docker_network_id`: ID of the Docker network the container is in.
* `__meta_docker_network_name`: Name of the Docker network the container is in.
* `__meta_docker_network_ingress`: Set to `true` if the Docker network is an ingress network.
* `__meta_docker_network_internal`: Set to `true` if the Docker network is an internal network.
* `__meta_docker_network_label_<labelname>`: Each label from the network the container is in.
* `__meta_docker_network_scope`: The scope of the network the container is in.
* `__meta_docker_network_ip`: The IP of the container in the network.
* `__meta_docker_port_private`: The private port on the container.
* `__meta_docker_port_public`: The publicly exposed port from the container, if a port mapping exists.
* `__meta_docker_port_public_ip`: The public IP of the container, if a port mapping exists.

Each discovered container maps to one target per unique combination of networks and port mappings used by the container.

## Component health

`discovery.docker` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.docker` does not expose any component-specific debug information.

## Debug metrics

`discovery.docker` does not expose any component-specific debug metrics.

## Examples

### Linux or macOS hosts

This example discovers Docker containers when the host machine is macOS or Linux:

```alloy
discovery.docker "containers" {
  host = "unix:///var/run/docker.sock"
}

prometheus.scrape "demo" {
  targets    = discovery.docker.containers.targets
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

### Windows hosts

This example discovers Docker containers when the host machine is Windows:

```alloy
discovery.docker "containers" {
  host = "tcp://localhost:2375"
}

prometheus.scrape "demo" {
  targets    = discovery.docker.containers.example.targets
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

> **NOTE**: This example requires the "Expose daemon on tcp://localhost:2375
> without TLS" setting to be enabled in the Docker Engine settings.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.docker` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
