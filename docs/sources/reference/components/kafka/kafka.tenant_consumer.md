---
canonical: https://grafana.com/docs/alloy/latest/reference/components/kafka/kafka.tenant_consumer/
description: Learn about kafka.tenant_consumer
labels:
  stage: experimental
  products:
    - oss
title: kafka.tenant_consumer
---

# `kafka.tenant_consumer`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`kafka.tenant_consumer` reads the records that [`kafka.tenant_producer`][kafka.tenant_producer] writes to Kafka and forwards them to other components, keeping the tenant of each record.
Each signal has its own topic, and each tenant owns the same partition number in every topic.
Each signal of a tenant is therefore processed by exactly one consumer at a time, and a stalled signal doesn't delay any other signal or tenant.

You can specify multiple `kafka.tenant_consumer` components by giving them different labels.

[kafka.tenant_producer]: ../kafka.tenant_producer/

## Usage

```alloy
kafka.tenant_consumer "<LABEL>" {
  registry = <TENANT_REGISTRY>

  client {
    brokers = <BROKER_LIST>
  }
}
```

## Arguments

You can use the following arguments with `kafka.tenant_consumer`:

| Name                      | Type                     | Description                                                            | Default             | Required |
| ------------------------- | ------------------------ | ---------------------------------------------------------------------- | ------------------- | -------- |
| `registry`                | `string` or `secret`     | Contents of the tenant registry file.                                  |                     | yes      |
| `group_id`                | `string`                 | Kafka consumer group to join.                                          | `"alloy-consumers"` | no       |
| `instance_id`             | `string`                 | Static group member ID of this consumer.                               |                     | no       |
| `logs_forward_to`         | `list(LogsReceiver)`     | Receivers for Loki push records.                                       |                     | no       |
| `max_backoff`             | `duration`               | Maximum backoff between retries of a record.                           | `"10s"`             | no       |
| `max_retries`             | `int`                    | Maximum number of retries of a record before it's dropped.             | `5`                 | no       |
| `metrics_forward_to`      | `list(MetricsReceiver)`  | Receivers for Prometheus remote write records.                         |                     | no       |
| `min_backoff`             | `duration`               | Initial backoff between retries of a record.                           | `"100ms"`           | no       |
| `profiles_forward_to`     | `list(ProfilesReceiver)` | Receivers for Pyroscope ingest records.                                |                     | no       |
| `session_timeout`         | `duration`               | Time after which the group removes a consumer that stopped responding. | `"45s"`             | no       |
| `start_offset`            | `string`                 | Where to start reading a partition that has no committed offset.       | `"earliest"`        | no       |

The `registry` argument takes the contents of the same tenant registry file that `kafka.tenant_producer` uses.
Refer to [Tenant registry][registry] for the file format and validation rules.
If the file is invalid, the component rejects the whole file and keeps using the previous registry.

The `start_offset` argument accepts `"earliest"` or `"latest"`.
It only applies when the consumer group has no committed offset for a partition.

When you change the `client` block, `group_id`, `instance_id`, `session_timeout`, `start_offset`, or the registry's `topics`, the consumer leaves the group and joins it again.

`kafka.tenant_consumer` never creates topics.
Every topic named in the registry must exist and have at least as many partitions as the registry's `partitions` field.

[registry]: ../kafka.tenant_producer/#tenant-registry

### Partition ownership

All `kafka.tenant_consumer` components with the same `group_id` join one consumer group and read every topic named in the registry.
Kafka assigns each partition of each topic to one consumer in the group.
Because each partition belongs to one tenant, and each topic holds one signal, each signal of a tenant is processed by exactly one consumer at a time.
Different signals of the same tenant can be processed by different consumers.

`kafka.tenant_consumer` uses the cooperative-sticky balancer.
Each assigned partition has its own worker, which processes the partition's records in order.
Before a rebalance moves a partition to another consumer, the component stops the partition's worker and commits the offsets of the records it finished.
The component commits only the offsets of processed records, every second and on rebalance or shutdown.

Affinity means one owner at a time, not always the same consumer.
A rebalance, for example when a consumer joins, leaves, or restarts, can move a tenant's partition to another consumer.
The new owner starts from the last committed offset, so it processes again any records that the previous owner processed but didn't commit.

Set the `instance_id` argument to enable static group membership.
A consumer that restarts within `session_timeout` gets its partitions back without a rebalance.
Use a stable name that's unique for each consumer, for example the pod name of a Kubernetes StatefulSet.

### Record handling

Before it forwards a record, `kafka.tenant_consumer` checks the record against the registry.
It drops records on partitions that aren't assigned to a tenant, records whose `tenant_id` header doesn't match the tenant of the partition, records in a topic that doesn't match their `signal` header, and records with an unsupported `schema_version` header.

`kafka.tenant_consumer` drops records it can't decode, so malformed payloads never block a partition.
It also drops records whose signal has no downstream configured, for example Loki records when `logs_forward_to` is empty.

If a downstream component returns an error, `kafka.tenant_consumer` retries the record with exponential backoff between `min_backoff` and `max_backoff`, up to `max_retries` times, and then drops it.
Retries block only the partition of the record, which holds one signal of one tenant.
While a partition's worker falls behind, the component pauses fetching that partition and keeps fetching all others.

### Tenant propagation

`kafka.tenant_consumer` forwards each record based on its `format` header and carries the tenant in a different way for each format:

| Format             | Forwarded to          | How the tenant is carried                                                                              |
| ------------------ | --------------------- | ------------------------------------------------------------------------------------------------------ |
| `prom_rw_v1`       | `metrics_forward_to`  | In the request context. Use [`kafka.tenant_prometheus_write`][kafka.tenant_prometheus_write].          |
| `loki_push`        | `logs_forward_to`     | In the `__tenant_id__` label of each entry. `loki.write` uses this label as the tenant and removes it. |
| `pyroscope_ingest` | `profiles_forward_to` | In the request context. Use [`pyroscope.write`][pyroscope.write] with `tenant_from_context = true`.    |
| `otlp`             | [`output`][output]    | In the `X-Scope-OrgID` client metadata key.                                                            |

To send the tenant of OTLP data to an endpoint, configure an [`otelcol.auth.headers`][otelcol.auth.headers] component with a `header` block that sets `key = "X-Scope-OrgID"` and `from_context = "X-Scope-OrgID"`, and use it as the `auth` handler of the exporter.
If the pipeline includes [`otelcol.processor.batch`][otelcol.processor.batch], set `metadata_keys = ["X-Scope-OrgID"]`.
Without it, the batch processor loses the tenant.

{{< admonition type="caution" >}}
`prometheus.remote_write` ignores the tenant in the request context, and so does `pyroscope.write` unless `tenant_from_context` is `true`.
If you connect them to `kafka.tenant_consumer`, they send the data of every tenant with the same tenant ID.
{{< /admonition >}}

The following components preserve the tenant and are tested between `kafka.tenant_consumer` and the write components:

- `prometheus.relabel`
- `pyroscope.relabel`
- `loki.process`
- `loki.relabel`
- `otelcol.processor.batch` with `metadata_keys = ["X-Scope-OrgID"]`

Other components are untested and aren't supported in `kafka.tenant_consumer` pipelines.

[kafka.tenant_prometheus_write]: ../kafka.tenant_prometheus_write/
[pyroscope.write]: ../../pyroscope/pyroscope.write/
[otelcol.auth.headers]: ../../otelcol/otelcol.auth.headers/
[otelcol.processor.batch]: ../../otelcol/otelcol.processor.batch/

### Delivery guarantees

`kafka.tenant_consumer` commits the offset of a record after the downstream call returns.
Delivery is at-least-once end-to-end only if the downstream call is synchronous:

- **`kafka.tenant_prometheus_write`:** Sends each request synchronously and has no WAL, because Kafka is the durable log. Delivery is at-least-once.
- **`pyroscope.write`:** Sends each request synchronously. Delivery is at-least-once.
- **`otelcol.exporter.*`:** Delivery is at-least-once only if the exporter has `sending_queue { enabled = false }` and no asynchronous component sits between `kafka.tenant_consumer` and the exporter.
  `otelcol.processor.batch` accepts data before it's exported, so data in its buffer is lost if {{< param "PRODUCT_NAME" >}} crashes.
- **`loki.write`:** Queues entries and sends them asynchronously. Delivery is best effort: queued entries are lost if {{< param "PRODUCT_NAME" >}} crashes.

Records that `kafka.tenant_consumer` drops aren't delivered.

The throughput of one signal of a tenant is limited to what one partition and one consumer can handle.

## Blocks

You can use the following blocks with `kafka.tenant_consumer`:

{{< docs/alloy-config >}}

| Block                     | Description                                     | Required |
| ------------------------- | ----------------------------------------------- | -------- |
| [`client`][client]        | Configures the connection to the Kafka cluster. | yes      |
| `client` > [`sasl`][sasl] | Configures SASL authentication to Kafka.        | no       |
| `client` > [`tls`][tls]   | Configures TLS for the connection to Kafka.     | no       |
| [`output`][output]        | Configures where to send OTLP records.          | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `client` > `sasl` refers to a `sasl` block defined inside a `client` block.

[client]: #client
[sasl]: #sasl
[tls]: #tls
[output]: #output

{{< /docs/alloy-config >}}

### `client`

The `client` block configures the connection to the Kafka cluster.

The following arguments are supported:

| Name        | Type           | Description                                   | Default | Required |
| ----------- | -------------- | --------------------------------------------- | ------- | -------- |
| `brokers`   | `list(string)` | Addresses of the Kafka brokers to connect to. |         | yes      |
| `client_id` | `string`       | Client ID to send to the brokers.             |         | no       |

### `sasl`

The `sasl` block configures SASL authentication to Kafka.

The following arguments are supported:

| Name        | Type     | Description                    | Default   | Required |
| ----------- | -------- | ------------------------------ | --------- | -------- |
| `password`  | `secret` | Password to authenticate with. |           | yes      |
| `username`  | `string` | Username to authenticate with. |           | yes      |
| `mechanism` | `string` | SASL mechanism to use.         | `"PLAIN"` | no       |

The `mechanism` argument accepts `"PLAIN"`, `"SCRAM-SHA-256"`, or `"SCRAM-SHA-512"`.

### `tls`

The `tls` block configures TLS for the connection to Kafka.
If you don't provide the `tls` block, the connection is unencrypted.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `output`

The `output` block configures the components that receive OTLP records.

The following arguments are supported:

| Name      | Type                     | Description                           | Default | Required |
| --------- | ------------------------ | ------------------------------------- | ------- | -------- |
| `logs`    | `list(otelcol.Consumer)` | List of consumers to send logs to.    | `[]`    | no       |
| `metrics` | `list(otelcol.Consumer)` | List of consumers to send metrics to. | `[]`    | no       |
| `traces`  | `list(otelcol.Consumer)` | List of consumers to send traces to.  | `[]`    | no       |

`kafka.tenant_consumer` drops OTLP records whose signal has no consumer in the `output` block.

## Exported fields

`kafka.tenant_consumer` doesn't export any fields.

## Component health

`kafka.tenant_consumer` is reported as unhealthy if it's given an invalid configuration, if it can't create the Kafka consumer, or while it can't verify the topics.
The topic check fails if a topic doesn't exist, if a topic has fewer partitions than the registry declares, or if the brokers are unreachable.
While the topic check fails, the component doesn't consume records and retries the check every 10 seconds.

## Debug information

`kafka.tenant_consumer` doesn't expose any component-specific debug information.

## Debug metrics

The following Prometheus metrics are exposed:

| Name                                             | Type        | Description                                                                                              |
| ------------------------------------------------ | ----------- | -------------------------------------------------------------------------------------------------------- |
| `kafka_tenant_consumer_assigned_partitions`      | `gauge`     | Number of partitions currently assigned to this consumer.                                                |
| `kafka_tenant_consumer_dropped_total`            | `counter`   | Total number of records dropped, by `reason` and `format`.                                               |
| `kafka_tenant_consumer_lag`                      | `gauge`     | Records between the last processed record and the high watermark of the partition, as of its fetch.      |
| `kafka_tenant_consumer_process_duration_seconds` | `histogram` | Time spent processing one record, including retries, by `format`.                                        |
| `kafka_tenant_consumer_rebalances_total`         | `counter`   | Total number of times partitions were assigned to this consumer.                                         |
| `kafka_tenant_consumer_records_consumed_total`   | `counter`   | Total number of records forwarded downstream, by `signal` and `format`.                                  |

The `kafka_tenant_consumer_lag` metric has `topic` and `partition` labels.

The `reason` label of `kafka_tenant_consumer_dropped_total` has one of the following values:

- `unassigned_partition`: The record's partition isn't assigned to a tenant, for example because it's retired.
- `tenant_mismatch`: The record's `tenant_id` header doesn't match the tenant of the partition.
- `topic_mismatch`: The record's topic isn't the topic the registry maps the record's `signal` header to.
- `unsupported_schema_version`: The record's `schema_version` header isn't supported.
- `decode_error`: The record's payload can't be decoded, or the downstream component rejected it as invalid.
- `no_downstream`: No downstream component is configured for the record's signal.
- `downstream_error`: The downstream component still returned an error after `max_retries` retries.

## Example

This example shows a producer instance and a consumer instance of {{< param "PRODUCT_NAME" >}} that share the following tenant registry file, `/etc/alloy/tenant-registry.yaml`:

```yaml
topics:
  metrics: alloy-metrics
  logs: alloy-logs
  traces: alloy-traces
  profiles: alloy-profiles
partitions: 8
tenants:
  team-a: 0
  team-b: 1
retired:
  - 2
```

The producer instance receives data over HTTP and writes it to Kafka:

```alloy
local.file "tenant_registry" {
  filename = "/etc/alloy/tenant-registry.yaml"
}

kafka.tenant_producer "default" {
  registry = local.file.tenant_registry.content

  http {
    listen_address = "0.0.0.0"
    listen_port    = 8080
  }

  client {
    brokers = ["kafka-0.kafka:9092", "kafka-1.kafka:9092", "kafka-2.kafka:9092"]
  }
}
```

The consumer instance reads the records and sends metrics to Mimir, logs to Loki, profiles to Pyroscope, and OTLP data to an OTLP endpoint, each with the tenant of the record:

```alloy
local.file "tenant_registry" {
  filename = "/etc/alloy/tenant-registry.yaml"
}

kafka.tenant_consumer "default" {
  registry    = local.file.tenant_registry.content
  instance_id = sys.env("POD_NAME")

  client {
    brokers = ["kafka-0.kafka:9092", "kafka-1.kafka:9092", "kafka-2.kafka:9092"]
  }

  metrics_forward_to  = [kafka.tenant_prometheus_write.mimir.receiver]
  logs_forward_to     = [loki.write.loki.receiver]
  profiles_forward_to = [pyroscope.write.pyroscope.receiver]

  output {
    metrics = [otelcol.processor.batch.default.input]
    logs    = [otelcol.processor.batch.default.input]
    traces  = [otelcol.processor.batch.default.input]
  }
}

kafka.tenant_prometheus_write "mimir" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}

loki.write "loki" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}

pyroscope.write "pyroscope" {
  tenant_from_context = true

  endpoint {
    url = "http://pyroscope:4040"
  }
}

otelcol.processor.batch "default" {
  metadata_keys = ["X-Scope-OrgID"]

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.auth.headers "tenant" {
  header {
    key          = "X-Scope-OrgID"
    from_context = "X-Scope-OrgID"
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = "otlp-gateway.example.com:4317"
    auth     = otelcol.auth.headers.tenant.handler
  }

  sending_queue {
    enabled = false
  }
}
```

In this example, `otelcol.processor.batch` makes OTLP delivery best effort.
For at-least-once OTLP delivery, connect the `output` block directly to `otelcol.exporter.otlp.default.input`.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`kafka.tenant_consumer` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)
- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)
- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)
- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
