---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.kafka/
aliases:
  - ../prometheus.exporter.kafka/ # /docs/alloy/latest/reference/components/prometheus.exporter.kafka/
description: Learn about prometheus.exporter.kafka
title: prometheus.exporter.kafka
---

# prometheus.exporter.kafka

The `prometheus.exporter.kafka` component embeds
[kafka_exporter](https://github.com/grafana/kafka_exporter) for collecting metrics from a kafka server.

## Usage

```alloy
prometheus.exporter.kafka "LABEL" {
    kafka_uris = KAFKA_URI_LIST
}
```

## Arguments

You can use the following arguments to configure the exporter's behavior.
Omitted fields take their default values.

| Name                          | Type            | Description                                                                                                                                                                            | Default | Required |
| ----------------------------- | --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `kafka_uris`                  | `array(string)` | Address array (host:port) of Kafka server.                                                                                                                                             |         | yes      |
| `instance`                    | `string`        | The`instance`label for metrics, default is the hostname:port of the first kafka_uris. You must manually provide the instance value if there is more than one string in kafka_uris.     |         | no       |
| `use_sasl`                    | `bool`          | Connect using SASL/PLAIN.                                                                                                                                                              |         | no       |
| `use_sasl_handshake`          | `bool`          | Only set this to false if using a non-Kafka SASL proxy.                                                                                                                                | `true`  | no       |
| `sasl_username`               | `string`        | SASL user name.                                                                                                                                                                        |         | no       |
| `sasl_password`               | `string`        | SASL user password.                                                                                                                                                                    |         | no       |
| `sasl_mechanism`              | `string`        | The SASL SCRAM SHA algorithm sha256 or sha512 as mechanism.                                                                                                                            |         | no       |
| `sasl_disable_pafx_fast`      | `bool`          | Configure the Kerberos client to not use PA_FX_FAST.                                                                                                                                   |         | no       |
| `use_tls`                     | `bool`          | Connect using TLS.                                                                                                                                                                     |         | no       |
| `tls_server_name`             | `string`        | Used to verify the hostname on the returned certificates unless tls.insecure-skip-tls-verify is given. If you don't provide the Kafka server name, the hostname is taken from the URL. |         | no       |
| `ca_file`                     | `string`        | The optional certificate authority file for TLS client authentication.                                                                                                                 |         | no       |
| `cert_file`                   | `string`        | The optional certificate file for TLS client authentication.                                                                                                                           |         | no       |
| `key_file`                    | `string`        | The optional key file for TLS client authentication.                                                                                                                                   |         | no       |
| `insecure_skip_verify`        | `bool`          | If set to true, the server's certificate will not be checked for validity. This makes your HTTPS connections insecure.                                                                 |         | no       |
| `kafka_version`               | `string`        | Kafka broker version.                                                                                                                                                                  | `2.0.0` | no       |
| `use_zookeeper_lag`           | `bool`          | If set to true, use a group from zookeeper.                                                                                                                                            |         | no       |
| `zookeeper_uris`              | `array(string)` | Address array (hosts) of zookeeper server.                                                                                                                                             |         | no       |
| `kafka_cluster_name`          | `string`        | Kafka cluster name.                                                                                                                                                                    |         | no       |
| `metadata_refresh_interval`   | `duration`      | Metadata refresh interval.                                                                                                                                                             | `1m`    | no       |
| `gssapi_service_name`         | `string`        | Service name when using Kerberos Authorization                                                                                                                                         |         | no       |
| `gssapi_kerberos_config_path` | `string`        | Kerberos config path.                                                                                                                                                                  |         | no       |
| `gssapi_realm`                | `string`        | Kerberos realm.                                                                                                                                                                        |         | no       |
| `gssapi_key_tab_path`         | `string`        | Kerberos keytab file path.                                                                                                                                                             |         | no       |
| `gssapi_kerberos_auth_type`   | `string`        | Kerberos auth type. Either 'keytabAuth' or 'userAuth'.                                                                                                                                 |         | no       |
| `offset_show_all`             | `bool`          | If true, the broker may auto-create topics that we requested which do not already exist.                                                                                               | `true`  | no       |
| `topic_workers`               | `int`           | Minimum number of topics to monitor.                                                                                                                                                   | `100`   | no       |
| `allow_concurrency`           | `bool`          | If set to true, all scrapes trigger Kafka operations. Otherwise, they will share results. WARNING: Disable this on large clusters.                                                     | `true`  | no       |
| `allow_auto_topic_creation`   | `bool`          | If true, the broker may auto-create topics that we requested which do not already exist.                                                                                               |         | no       |
| `max_offsets`                 | `int`           | The maximum number of offsets to store in the interpolation table for a partition.                                                                                                     | `1000`  | no       |
| `prune_interval_seconds`      | `int`           | Deprecated (no-op), use `metadata_refresh_interval` instead.                                                                                                                           | `30`    | no       |
| `topics_filter_regex`         | `string`        | Regex filter for topics to be monitored.                                                                                                                                               | `.*`    | no       |
| `topics_exclude_regex`        | `string`        | Regex that determines which topics to exclude.                                                                                                                                         | `^$`    | no       |
| `groups_filter_regex`         | `string`        | Regex filter for consumer groups to be monitored.                                                                                                                                      | `.*`    | no       |
| `groups_exclude_regex`        | `string`        | Regex that determines which consumer groups to exclude.                                                                                                                                | `^$`    | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.kafka` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.kafka` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.kafka` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.kafka`:

```alloy
prometheus.exporter.kafka "example" {
  kafka_uris = ["localhost:9200"]
}

// Configure a prometheus.scrape component to send metrics to.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.kafka.example.targets
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
