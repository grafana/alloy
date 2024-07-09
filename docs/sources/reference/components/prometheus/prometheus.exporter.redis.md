---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.redis/
aliases:
  - ../prometheus.exporter.redis/ # /docs/alloy/latest/reference/components/prometheus.exporter.redis/
description: Learn about prometheus.exporter.redis
title: prometheus.exporter.redis
---

# prometheus.exporter.redis

The `prometheus.exporter.redis` component embeds the [`redis_exporter`](https://github.com/oliver006/redis_exporter) for collecting metrics from a Redis database.

## Usage

```alloy
prometheus.exporter.redis "LABEL" {
    redis_addr = REDIS_ADDRESS
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name                          | Type           | Description                                                                                                             | Default    | Required |
| ----------------------------- | -------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------- | -------- |
| `redis_addr`                  | `string`       | Address (host and port) of the Redis instance to connect to.                                                            |            | yes      |
| `redis_user`                  | `string`       | User name to use for authentication. Redis ACL for Redis 6.0 and newer.                                                 |            | no       |
| `redis_password`              | `secret`       | Password of the Redis instance.                                                                                         |            | no       |
| `redis_password_file`         | `string`       | Path of a file containing a password.                                                                                   |            | no       |
| `redis_password_map_file`     | `string`       | Path of a JSON file containing a map of Redis URIs to passwords.                                                        |            | no       |
| `namespace`                   | `string`       | Namespace for the metrics.                                                                                              | `"redis"`  | no       |
| `config_command`              | `string`       | What to use for the CONFIG command.                                                                                     | `"CONFIG"` | no       |
| `check_keys`                  | `list(string)` | List of key-patterns to export value and length/size, searched for with SCAN.                                           |            | no       |
| `check_key_groups`            | `list(string)` | List of Lua regular expressions (regex) for grouping keys.                                                              |            | no       |
| `check_key_groups_batch_size` | `int`          | Check key or key groups batch size hint for the underlying SCAN.                                                        | `10000`    | no       |
| `max_distinct_key_groups`     | `int`          | The maximum number of distinct key groups with the most memory utilization to present as distinct metrics per database. | `100`      | no       |
| `check_single_keys`           | `list(string)` | List of single keys to export value and length/size.                                                                    |            | no       |
| `check_streams`               | `list(string)` | List of stream-patterns to export info about streams, groups, and consumers to search for with SCAN.                    |            | no       |
| `check_single_streams`        | `list(string)` | List of single streams to export info about streams, groups, and consumers.                                             |            | no       |
| `export_key_values`           | `bool`         | Whether to export key values as labels when using `check_keys` or `check_single_keys`.                                  | `true`     | no       |
| `count_keys`                  | `list(string)` | List of individual keys to export counts for.                                                                           |            | no       |
| `script_path`                 | `string`       | Path to Lua Redis script for collecting extra metrics.                                                                  |            | no       |
| `script_paths`                | `list(string)` | List of paths to Lua Redis scripts for collecting extra metrics.                                                        |            | no       |
| `connection_timeout`          | `duration`     | Timeout for connection to Redis instance (in Golang duration format).                                                   | `"15s"`    | no       |
| `tls_client_key_file`         | `string`       | Name of the client key file (including full path) if the server requires TLS client authentication.                     |            | no       |
| `tls_client_cert_file`        | `string`       | Name of the client certificate file (including full path) if the server requires TLS client authentication.             |            | no       |
| `tls_ca_cert_file`            | `string`       | Name of the CA certificate file (including full path) if the server requires TLS client authentication.                 |            | no       |
| `set_client_name`             | `bool`         | Whether to set client name to `redis_exporter`.                                                                         | `true`     | no       |
| `is_tile38`                   | `bool`         | Whether to scrape Tile38-specific metrics.                                                                              |            | no       |
| `is_cluster`                  | `bool`         | Whether the connection is to a Redis cluster.                                                                           |            | no       |
| `export_client_list`          | `bool`         | Whether to scrape Client List specific metrics.                                                                         |            | no       |
| `export_client_port`          | `bool`         | Whether to include the client's port when exporting the client list.                                                    |            | no       |
| `redis_metrics_only`          | `bool`         | Whether to just export metrics or to also export go runtime metrics.                                                    |            | no       |
| `ping_on_connect`             | `bool`         | Whether to ping the Redis instance after connecting.                                                                    |            | no       |
| `incl_system_metrics`         | `bool`         | Whether to include system metrics. For example `redis_total_system_memory_bytes`.                                       |            | no       |
| `skip_tls_verification`       | `bool`         | Whether to skip TLS verification.                                                                                       |            | no       |

If `redis_password_file` is defined, it will take precedence over `redis_password`.

When `check_key_groups` isn't set, no key groups are made.

The `check_key_groups_batch_size` argument name reflects key groups for backwards compatibility, but applies to both key and key groups.

The `script_path` argument may also be specified as a comma-separated string of paths, though it's encouraged to use `script_paths` when using multiple Lua scripts.

Any leftover key groups beyond `max_distinct_key_groups` are aggregated in the 'overflow' bucket.

The `is_cluster` argument must be set to `true` when connecting to a Redis cluster and using either of the `check_keys` and `check_single_keys` arguments.

{{< admonition type="note" >}}
Setting `export_client_port` increases the cardinality of all Redis metrics.
{{< /admonition >}}

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.redis` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.redis` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.redis` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.redis`:

```alloy
prometheus.exporter.redis "example" {
  redis_addr = "localhost:6379"
}

// Configure a prometheus.scrape component to collect Redis metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.redis.example.targets
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

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the `remote_write` API.
- `PASSWORD`: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.redis` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
