---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.kafka/
aliases:
  - ../prometheus.exporter.kafka/ # /docs/alloy/latest/reference/components/prometheus.exporter.kafka/
description: Learn about prometheus.exporter.kafka
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.kafka
---

# `prometheus.exporter.kafka`

The `prometheus.exporter.kafka` component embeds the [`kafka_exporter`][kafka-exporter] to collect metrics from a Kafka server.

You can specify multiple `prometheus.exporter.kafka` components by giving them different labels.

[kafka-exporter]: https://github.com/grafana/kafka_exporter

## Usage

```alloy
prometheus.exporter.kafka "<LABEL>" {
  kafka_uris = ["<KAFKA_URI>"]
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.kafka`:

| Name                          | Type           | Description                                                                          | Default   | Required |
| ----------------------------- | -------------- | ------------------------------------------------------------------------------------ | --------- | -------- |
| `kafka_uris`                  | `list(string)` | Addresses (host:port) of the Kafka servers.                                          |           | yes      |
| `allow_auto_topic_creation`   | `bool`         | Whether the broker may auto-create requested topics that don't exist.                |           | no       |
| `allow_concurrency`           | `bool`         | Whether each scrape triggers its own Kafka operations.                               | `true`    | no       |
| `ca_file`                     | `string`       | The optional certificate authority file for TLS client authentication.               |           | no       |
| `cert_file`                   | `string`       | The optional certificate file for TLS client authentication.                         |           | no       |
| `groups_exclude_regex`        | `string`       | Regex that determines which consumer groups to exclude.                              | `"^$"`    | no       |
| `groups_filter_regex`         | `string`       | Regex filter for consumer groups to monitor.                                         | `".*"`    | no       |
| `gssapi_kerberos_auth_type`   | `string`       | Kerberos auth type. Either `keytabAuth` or `userAuth`.                               |           | no       |
| `gssapi_kerberos_config_path` | `string`       | Kerberos configuration path.                                                         |           | no       |
| `gssapi_key_tab_path`         | `string`       | Kerberos keytab path.                                                                |           | no       |
| `gssapi_realm`                | `string`       | Kerberos realm.                                                                      |           | no       |
| `gssapi_service_name`         | `string`       | Service name when using Kerberos Authorization.                                      |           | no       |
| `insecure_skip_verify`        | `bool`         | Whether to skip validation of the server's certificate.                              |           | no       |
| `instance`                    | `string`       | The `instance` label for metrics.                                                    |           | no       |
| `kafka_cluster_name`          | `string`       | Kafka cluster name.                                                                  |           | no       |
| `kafka_version`               | `string`       | Kafka broker version.                                                                | `"2.0.0"` | no       |
| `key_file`                    | `string`       | The optional key file for TLS client authentication.                                 |           | no       |
| `max_offsets`                 | `int`          | Maximum number of offsets to store per partition in the interpolation table.         | `1000`    | no       |
| `metadata_refresh_interval`   | `duration`     | Metadata refresh interval.                                                           | `"1m"`    | no       |
| `offset_show_all`             | `bool`         | Whether to show the offset and lag for all consumer groups, not just connected ones. | `true`    | no       |
| `prune_interval_seconds`      | `int`          | Deprecated (no-op), use `metadata_refresh_interval` instead.                         | `30`      | no       |
| `sasl_disable_pafx_fast`      | `bool`         | Configure the Kerberos client to not use PA_FX_FAST.                                 |           | no       |
| `sasl_mechanism`              | `string`       | SASL mechanism. One of `plain`, `scram-sha256`, `scram-sha512`, or `gssapi`.         |           | no       |
| `sasl_password`               | `secret`       | SASL user password.                                                                  |           | no       |
| `sasl_username`               | `string`       | SASL user name.                                                                      |           | no       |
| `tls_server_name`             | `string`       | Server name used to verify the hostname on returned certificates.                    |           | no       |
| `topic_workers`               | `int`          | Number of concurrent workers that collect topic metrics.                             | `100`     | no       |
| `topics_exclude_regex`        | `string`       | Regex that determines which topics to exclude.                                       | `"^$"`    | no       |
| `topics_filter_regex`         | `string`       | Regex filter for topics to monitor.                                                  | `".*"`    | no       |
| `use_sasl`                    | `bool`         | Connect using SASL/PLAIN.                                                            |           | no       |
| `use_sasl_handshake`          | `bool`         | Only set this to false if using a non-Kafka SASL proxy.                              | `true`    | no       |
| `use_tls`                     | `bool`         | Connect using TLS.                                                                   |           | no       |
| `use_zookeeper_lag`           | `bool`         | If set to true, use a group from Apache ZooKeeper.                                   |           | no       |
| `zookeeper_uris`              | `list(string)` | Addresses (hosts) of the Apache ZooKeeper servers.                                   |           | no       |

When `allow_concurrency` is `true`, every scrape triggers its own Kafka operations.
When it's `false`, concurrent scrapes share a single set of results.

{{< admonition type="caution" >}}
Set `allow_concurrency` to `false` on large clusters.
Leaving it enabled makes each concurrent scrape issue its own Kafka operations, which can place significant load on your brokers.
{{< /admonition >}}

When `insecure_skip_verify` is `true`, the component doesn't check the server's certificate for validity.
This makes your HTTPS connections insecure.

The `instance` label defaults to the host and port of the first entry in `kafka_uris`.
If `kafka_uris` contains more than one address, you must set `instance` explicitly.

The component verifies the hostname on returned certificates unless you enable `insecure_skip_verify`.
If you don't set `tls_server_name`, the component takes the hostname from the URL.

## Blocks

The `prometheus.exporter.kafka` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.kafka` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.kafka` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.kafka` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.kafka`:

```alloy
prometheus.exporter.kafka "example" {
  kafka_uris = ["localhost:9092"]
}

// Configure a prometheus.scrape component to collect Kafka metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.kafka.example.targets
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

`prometheus.exporter.kafka` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
