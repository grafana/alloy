---
canonical: https://grafana.com/docs/alloy/latest/reference/components/kafka/kafka.tenant_producer/
description: Learn about kafka.tenant_producer
labels:
  stage: experimental
  products:
    - oss
title: kafka.tenant_producer
---

# `kafka.tenant_producer`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`kafka.tenant_producer` receives Prometheus, Loki, Pyroscope, and OTLP requests over HTTP and writes each request to the Kafka topic of its signal, in the partition assigned to the request's tenant.
A [tenant registry](#tenant-registry) maps each signal to a topic, and each tenant to exactly one partition number, which the tenant uses in every topic.
Use [`kafka.tenant_consumer`][kafka.tenant_consumer] to read the records back into {{< param "PRODUCT_NAME" >}} pipelines.

You can specify multiple `kafka.tenant_producer` components by giving them different labels.

[kafka.tenant_consumer]: ../kafka.tenant_consumer/

## Usage

```alloy
kafka.tenant_producer "<LABEL>" {
  registry = <TENANT_REGISTRY>

  client {
    brokers = <BROKER_LIST>
  }
}
```

The component starts an HTTP server that supports the following endpoints:

- `POST /api/v1/push`: Receives Prometheus remote write v1 requests. Remote write v2 isn't supported.
- `POST /loki/api/v1/push`: Receives Loki push API requests in JSON or protobuf format.
- `POST /ingest`: Receives Pyroscope ingest API requests.
- `POST /v1/metrics`, `POST /v1/logs`, and `POST /v1/traces`: Receive OTLP/HTTP requests in protobuf or JSON encoding.

OTLP over gRPC and the Pyroscope `push.v1.PusherService/Push` Connect API aren't supported.

### Tenants and responses

`kafka.tenant_producer` reads the tenant from the `X-Scope-OrgID` request header and looks up the tenant's partition in the registry.
It writes the request to the topic the registry maps the endpoint's signal to.
The component never falls back to hashing, so a tenant that isn't in the registry is always rejected.
It replies with a `2xx` status code only after Kafka acknowledged the record.

| Condition                                                                             | Status code                                                 |
| ------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| Kafka acknowledged the record.                                                        | `204` for Prometheus and Loki, `200` for Pyroscope and OTLP |
| The `X-Scope-OrgID` header is missing.                                                | `401`                                                       |
| The `X-Scope-OrgID` header contains more than one tenant, separated by a pipe (`\|`). | `400`                                                       |
| The tenant isn't in the registry.                                                     | `403`                                                       |
| The registry doesn't map the endpoint's signal to a topic.                            | `404`                                                       |
| The request body is larger than `max_body_size`.                                      | `413`                                                       |
| The producer buffer is full. Refer to `max_buffered_records`.                         | `429`                                                       |
| The topics aren't verified, Kafka is unavailable, or `produce_timeout` passed.        | `503`                                                       |

### Records

Each request becomes one Kafka record.
The record value is the request body, passed through byte-for-byte without decoding, including any compression.
`kafka.tenant_producer` doesn't validate payloads, so `kafka.tenant_consumer` drops malformed payloads later.

The record key is the tenant ID, and each record has the following headers:

- `tenant_id`: The tenant ID from the `X-Scope-OrgID` header.
- `signal`: One of `metrics`, `logs`, `traces`, or `profiles`.
- `format`: One of `prom_rw_v1`, `loki_push`, `pyroscope_ingest`, or `otlp`.
- `content_type`: The request's `Content-Type` header.
- `content_encoding`: The request's `Content-Encoding` header.
- `url`: The request path and query string.
- `schema_version`: The record format version, currently `1`.

Records of one tenant keep their order only within a single `kafka.tenant_producer` instance.
If you run several instances behind a load balancer, requests of one tenant that reach different instances can be written in any order.

## Arguments

You can use the following arguments with `kafka.tenant_producer`:

| Name                        | Type                 | Description                                                        | Default  | Required |
| --------------------------- | -------------------- | ------------------------------------------------------------------ | -------- | -------- |
| `registry`                  | `string` or `secret` | Contents of the tenant registry file.                              |          | yes      |
| `compression`               | `string`             | Compression codec for record batches.                              | `"none"` | no       |
| `graceful_shutdown_timeout` | `duration`           | Timeout for the HTTP server's graceful shutdown.                   | `"30s"`  | no       |
| `linger`                    | `duration`           | How long to wait for more records before sending a batch to Kafka. | `"10ms"` | no       |
| `max_body_size`             | `size`               | Maximum size of a request body.                                    | `"1MiB"` | no       |
| `max_buffered_records`      | `int`                | Maximum number of records waiting for Kafka to acknowledge them.   | `10000`  | no       |
| `produce_timeout`           | `duration`           | Maximum time to wait for Kafka to acknowledge a record.            | `"10s"`  | no       |

The `registry` argument takes the contents of the [tenant registry](#tenant-registry) file.

The `compression` argument accepts `"none"`, `"gzip"`, `"snappy"`, `"lz4"`, or `"zstd"`.

`kafka.tenant_producer` allows record batches of up to `max_body_size` plus 64 KiB for record headers and batch overhead.
Set the maximum message size of each topic, `max.message.bytes`, to at least this value.
Otherwise, Kafka rejects large requests.

`kafka.tenant_producer` never creates topics.
Every topic named in the registry must exist and have at least as many partitions as the registry's `partitions` field.
Until the component verifies the topics, it reports itself as unhealthy and rejects requests with `503`.

### Tenant registry

The tenant registry is a YAML file that maps each signal to a topic, and each tenant to one partition number.
A tenant uses the same partition number in every topic.
`kafka.tenant_producer` and `kafka.tenant_consumer` must use the same file.
Load the file with [`local.file`][local.file] and pass `local.file.<LABEL>.content` to the `registry` argument.

[local.file]: ../../local/local.file/

The following example shows a tenant registry:

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
  team-c: 2
retired:
  - 3
```

The tenant registry supports the following fields:

| Field        | Type          | Description                                                         | Required |
| ------------ | ------------- | ------------------------------------------------------------------- | -------- |
| `topics`     | `map(string)` | Map of signals to Kafka topics.                                     | yes      |
| `partitions` | `int`         | Minimum number of partitions every topic must have.                 | yes      |
| `tenants`    | `map(int)`    | Map of tenant IDs to partition numbers.                             | no       |
| `retired`    | `list(int)`   | Partitions that belonged to a removed tenant and aren't reassigned. | no       |

The following rules apply to the tenant registry:

- `topics` must map at least one signal to a non-empty topic name. Valid signals are `metrics`, `logs`, `traces`, and `profiles`.
- `partitions` must be greater than `0`.
- Every partition in `tenants` and `retired` must be in the range `[0, partitions)`.
- Two tenants can't share a partition.
- A tenant can't be assigned to a retired partition.
- Tenant IDs can't be empty.

If the file breaks any of these rules, the component rejects the whole file and keeps using the previous registry.

A topic per signal keeps signals independent: a stalled downstream for one signal of a tenant doesn't delay that tenant's other signals.
Signals without a topic are rejected with `404`.
Several signals can share a topic, and their records are then processed in one order.
Retention is set per topic, so each signal can have its own retention period.

{{< admonition type="caution" >}}
When you remove a tenant, add its partition to `retired`.
Don't assign a retired partition to a new tenant until the longest retention period of the topics has passed.
Until then, the partition still holds the previous tenant's records, and the new tenant could be sent the previous tenant's data.
`kafka.tenant_consumer` drops records whose `tenant_id` header doesn't match the tenant of the partition, but don't rely on this check alone.
{{< /admonition >}}

## Blocks

You can use the following blocks with `kafka.tenant_producer`:

{{< docs/alloy-config >}}

| Block                                 | Description                                        | Required |
| ------------------------------------- | -------------------------------------------------- | -------- |
| [`client`][client]                    | Configures the connection to the Kafka cluster.    | yes      |
| `client` > [`sasl`][sasl]             | Configures SASL authentication to Kafka.           | no       |
| `client` > [`tls` client][tls_client] | Configures TLS for the connection to Kafka.        | no       |
| [`http`][http]                        | Configures the HTTP server that receives requests. | no       |
| `http` > [`tls`][tls]                 | Configures TLS for the HTTP server.                | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `client` > `sasl` refers to a `sasl` block defined inside a `client` block.

[client]: #client
[sasl]: #sasl
[tls_client]: #tls-client
[http]: #http
[tls]: #tls

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

### `tls` client

The `tls` block inside the `client` block configures TLS for the connection to Kafka.
If you don't provide the `tls` block, the connection is unencrypted.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `http`

{{< docs/shared lookup="reference/components/server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block inside the `http` block configures TLS for the HTTP server.

{{< docs/shared lookup="reference/components/server-tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`kafka.tenant_producer` doesn't export any fields.

## Component health

`kafka.tenant_producer` is reported as unhealthy if it's given an invalid configuration, or while it can't verify the topics.
The topic check fails if a topic doesn't exist, if a topic has fewer partitions than the registry declares, or if the brokers are unreachable.
While the topic check fails, the component rejects requests with `503` and retries the check every 10 seconds.

## Debug information

`kafka.tenant_producer` doesn't expose any component-specific debug information.

## Debug metrics

The following Prometheus metrics are exposed:

| Name                                             | Type        | Description                                                           |
| ------------------------------------------------ | ----------- | --------------------------------------------------------------------- |
| `kafka_tenant_producer_produce_duration_seconds` | `histogram` | Time until a produced record was acknowledged or failed.              |
| `kafka_tenant_producer_produced_bytes_total`     | `counter`   | Total number of request body bytes produced to Kafka, by `signal`.    |
| `kafka_tenant_producer_rejected_total`           | `counter`   | Total number of requests rejected before producing, by `reason`.      |
| `kafka_tenant_producer_requests_total`           | `counter`   | Total number of requests, by `signal`, `format`, and response `code`. |

The `reason` label of `kafka_tenant_producer_rejected_total` has one of the following values: `no_tenant`, `multi_tenant`, `unknown_tenant`, `signal_disabled`, or `too_large`.

The HTTP server also exposes metrics with the `kafka_tenant_producer_` prefix, for example `kafka_tenant_producer_request_duration_seconds` and `kafka_tenant_producer_tcp_connections`.

## Example

The following example loads the tenant registry from a file and writes incoming requests to a three-broker Kafka cluster:

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

A client sends data for a tenant by setting the `X-Scope-OrgID` header.
For example, the following `prometheus.remote_write` component sends metrics for the `team-a` tenant to the producer:

```alloy
prometheus.remote_write "to_kafka" {
  endpoint {
    url = "http://alloy-producer:8080/api/v1/push"

    headers = {
      "X-Scope-OrgID" = "team-a",
    }
  }
}
```

To read the records back and forward them to Mimir, Loki, Pyroscope, and an OTLP endpoint, refer to the [`kafka.tenant_consumer` example][consumer-example].

[consumer-example]: ../kafka.tenant_consumer/#example
