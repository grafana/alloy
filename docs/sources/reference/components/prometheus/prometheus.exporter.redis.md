---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.redis/
aliases:
  - ../prometheus.exporter.redis/ # /docs/alloy/latest/reference/components/prometheus.exporter.redis/
description: Learn about prometheus.exporter.redis
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-19
title: prometheus.exporter.redis
---

# `prometheus.exporter.redis`

The `prometheus.exporter.redis` component embeds the [`redis_exporter`][redis-exporter] to collect metrics from a Redis database.

You can specify multiple `prometheus.exporter.redis` components by giving them different labels.

[redis-exporter]: https://github.com/oliver006/redis_exporter

## Usage

```alloy
prometheus.exporter.redis "<LABEL>" {
  redis_addr = "<REDIS_ADDRESS>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.redis`:

| Name                          | Type           | Description                                                                                                             | Default    | Required |
| ----------------------------- | -------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------- | -------- |
| `redis_addr`                  | `string`       | Address of the Redis instance to connect to, with or without a URI scheme.                                              |            | yes      |
| `check_key_groups`            | `list(string)` | List of Lua regular expressions for grouping keys.                                                                      |            | no       |
| `check_key_groups_batch_size` | `int`          | Approximate number of keys to process in each SCAN execution.                                                           | `10000`    | no       |
| `check_keys`                  | `list(string)` | List of key-patterns to export value and length/size, searched for with SCAN.                                           |            | no       |
| `check_single_keys`           | `list(string)` | List of single keys to export value and length/size.                                                                    |            | no       |
| `check_single_streams`        | `list(string)` | List of single streams to export info about streams, groups, and consumers.                                             |            | no       |
| `check_streams`               | `list(string)` | List of stream-patterns to export info about streams, groups, and consumers to search for with SCAN.                    |            | no       |
| `config_command`              | `string`       | What to use for the CONFIG command. Set it to `-` to skip config metrics extraction.                                    | `"CONFIG"` | no       |
| `connection_timeout`          | `duration`     | Timeout for the connection to the Redis instance.                                                                       | `"15s"`    | no       |
| `count_keys`                  | `list(string)` | List of key patterns to count, searched for with SCAN.                                                                  |            | no       |
| `export_client_list`          | `bool`         | Scrape Client List specific metrics.                                                                                    | `false`    | no       |
| `export_client_port`          | `bool`         | Include the client port when exporting the client list.                                                                 | `false`    | no       |
| `export_key_values`           | `bool`         | Export key values as labels with `check_keys` or `check_single_keys`.                                                   | `true`     | no       |
| `incl_system_metrics`         | `bool`         | Include system metrics, for example `redis_total_system_memory_bytes`.                                                  | `false`    | no       |
| `include_exporter_metrics`    | `bool`         | Report metrics about the exporter itself.                                                                               | `false`    | no       |
| `is_cluster`                  | `bool`         | Treat the connection as a Redis cluster.                                                                                | `false`    | no       |
| `is_tile38`                   | `bool`         | Scrape Tile38-specific metrics.                                                                                         | `false`    | no       |
| `max_distinct_key_groups`     | `int`          | The maximum number of distinct key groups with the most memory utilization to present as distinct metrics per database. | `100`      | no       |
| `namespace`                   | `string`       | Namespace for the metrics.                                                                                              | `"redis"`  | no       |
| `ping_on_connect`             | `bool`         | Ping the Redis instance after connecting.                                                                               | `false`    | no       |
| `redis_metrics_only`          | `bool`         | Export only Redis metrics and omit Go process and runtime metrics.                                                      | `false`    | no       |
| `redis_password`              | `secret`       | Password of the Redis instance.                                                                                         |            | no       |
| `redis_password_file`         | `string`       | Path of a file containing a password.                                                                                   |            | no       |
| `redis_password_map_file`     | `string`       | Path of a JSON file containing a map of Redis URIs to passwords.                                                        |            | no       |
| `redis_user`                  | `string`       | User name for authentication. Requires Redis 6.0 or later with ACL support.                                             |            | no       |
| `script_path`                 | `string`       | Path to Lua Redis script for collecting extra metrics.                                                                  |            | no       |
| `script_paths`                | `list(string)` | List of paths to Lua Redis scripts for collecting extra metrics.                                                        |            | no       |
| `set_client_name`             | `bool`         | Set the client name to `redis_exporter`.                                                                                | `true`     | no       |
| `skip_tls_verification`       | `bool`         | Skip TLS verification.                                                                                                  | `false`    | no       |
| `tls_ca_cert_file`            | `string`       | Full path to the CA certificate file that the component uses to verify the Redis server certificate.                    |            | no       |
| `tls_client_cert_file`        | `string`       | Full path to the client certificate file if the server requires TLS client authentication.                              |            | no       |
| `tls_client_key_file`         | `string`       | Full path to the client key file if the server requires TLS client authentication.                                      |            | no       |

The `redis_addr` argument accepts a host and port, such as `localhost:6379`, or a URI with one of the following schemes:

- `redis://`: Connects without TLS. This is the default when you omit the scheme.
- `rediss://`: Connects with TLS.
- `valkey://`: Equivalent to `redis://`.
- `valkeys://`: Equivalent to `rediss://`.

{{< admonition type="caution" >}}
The exporter enables TLS only when `redis_addr` starts with `rediss://` or `valkeys://`.
The `tls_ca_cert_file`, `tls_client_cert_file`, `tls_client_key_file`, and `skip_tls_verification` arguments have no effect on a connection that uses any other scheme.
{{< /admonition >}}

You can supply the Redis password with `redis_password`, `redis_password_file`, or `redis_password_map_file`.
Set only one of `redis_password_file` and `redis_password_map_file`.
If you set both of them, the component fails with an invalid configuration error.

You can set `redis_password` together with one of the file arguments.
If you set `redis_password_file`, the component uses the password from that file and ignores `redis_password`.
If you set `redis_password_map_file`, the component ignores `redis_password` only for the addresses that the file maps.

Set `tls_client_cert_file` and `tls_client_key_file` together.
If you set only one of them, the component fails with an invalid configuration error.

If you don't set `tls_ca_cert_file`, the component verifies the Redis server certificate against the system CA certificates.

If you don't set `check_key_groups`, the component creates no key groups.

The `check_key_groups_batch_size` argument name reflects key groups for backwards compatibility, but applies to both key and key groups.

The component aggregates any leftover key groups beyond `max_distinct_key_groups` into the `overflow` bucket.

Set `is_cluster` to `true` when you connect to a Redis cluster and use `check_keys` or `check_single_keys`.

The `script_path` argument accepts a comma-separated string of paths, but use `script_paths` when you have multiple Lua scripts.
Set either `script_path` or `script_paths`, not both.
If you set both, the component fails with an invalid configuration error.

{{< admonition type="note" >}}
Setting `export_client_port` increases the cardinality of all Redis metrics.
{{< /admonition >}}

The component sets the `instance` label on its exported targets to the value of `redis_addr`.

## Blocks

The `prometheus.exporter.redis` component doesn't support any blocks. You can configure this component with arguments.

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
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

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
