---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.docker/
aliases:
  - ../loki.source.docker/ # /docs/alloy/latest/reference/components/loki.source.docker/
description: Learn about loki.source.docker
title: loki.source.docker
---

# loki.source.docker

`loki.source.docker` reads log entries from Docker containers and forwards them to other `loki.*` components. Each component can read from a single Docker daemon.

Multiple `loki.source.docker` components can be specified by giving them different labels.

## Usage

```alloy
loki.source.docker "LABEL" {
  host       = HOST
  targets    = TARGET_LIST
  forward_to = RECEIVER_LIST
}
```

## Arguments
The component starts a new reader for each of the given `targets` and fans out log entries to the list of receivers passed in `forward_to`.

`loki.source.docker` supports the following arguments:

Name               | Type                 | Description                                                                    | Default | Required
-------------------|----------------------|--------------------------------------------------------------------------------|---------|---------
`host`             | `string`             | Address of the Docker daemon.                                                  |         | yes
`targets`          | `list(map(string))`  | List of containers to read logs from.                                          |         | yes
`forward_to`       | `list(LogsReceiver)` | List of receivers to send log entries to.                                      |         | yes
`labels`           | `map(string)`        | The default set of labels to apply on entries.                                 | `"{}"`  | no
`relabel_rules`    | `RelabelRules`       | Relabeling rules to apply on log entries.                                      | `"{}"`  | no
`refresh_interval` | `duration`           | The refresh interval to use when connecting to the Docker daemon over HTTP(S). | `"60s"` | no

## Blocks

The following blocks are supported inside the definition of `loki.source.docker`:

Hierarchy                    | Block             | Description                                              | Required
-----------------------------|-------------------|----------------------------------------------------------|---------
client                       | [client][]        | HTTP client settings when connecting to the endpoint.    | no
client > basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
client > authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
client > oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
client > oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
client > tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting. For example, `client >
basic_auth` refers to an `basic_auth` block defined inside a `client` block.

These blocks are only applicable when connecting to a Docker daemon over HTTP
or HTTPS and has no effect when connecting via a `unix:///` socket

[client]: #client-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### client block

The `client` block configures settings used to connect to HTTP(S) Docker daemons.

{{< docs/shared lookup="reference/components/http-client-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### basic_auth block

The `basic_auth` block configures basic authentication for HTTP(S) Docker daemons.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

The `authorization` block configures custom authorization to use for the Docker daemon.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

The `oauth2` block configures OAuth2 authorization to use for the Docker daemon.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

The `tls_config` block configures TLS settings for connecting to HTTPS Docker daemons.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.docker` does not export any fields.

## Component health

`loki.source.docker` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`loki.source.docker` exposes some debug information per target:
* Whether the target is ready to tail entries.
* The labels associated with the target.
* The most recent time a log line was read.

## Debug metrics

* `loki_source_docker_target_entries_total` (gauge): Total number of successful entries sent to the Docker target.
* `loki_source_docker_target_parsing_errors_total` (gauge): Total number of parsing errors while receiving Docker messages.

## Component behavior
The component uses its data path, a directory named after the domain's
fully qualified name, to store its _positions file_. The positions file is used
to store read offsets, so that if a component or {{< param "PRODUCT_NAME" >}} restarts,
`loki.source.docker` can pick up tailing from the same spot.

If the target's argument contains multiple entries with the same container
ID (for example as a result of `discovery.docker` picking up multiple exposed
ports or networks), `loki.source.docker` will deduplicate them, and only keep
the first of each container ID instances, based on the
`__meta_docker_container_id` label. As such, the Docker daemon is queried
for each container ID only once, and only one target will be available in the
component's debug info.

## Example

This example collects log entries from the files specified in the `targets`
argument and forwards them to a `loki.write` component to be written to Loki.

```alloy
discovery.docker "linux" {
  host = "unix:///var/run/docker.sock"
}

loki.source.docker "default" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.docker.linux.targets
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.docker` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
