---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.influxdb/
description: Learn about otelcol.receiver.influxdb
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.influxdb
---

# `otelcol.receiver.influxdb`

`otelcol.receiver.influxdb` receives InfluxDB metrics, converts them into OpenTelemetry (OTEL) format, and forwards them to other `otelcol.*` components over the network.

You can specify multiple `otelcol.receiver.influxdb` components by giving them different labels.

{{< admonition type="note" >}}
`otelcol.receiver.influxdb` is a wrapper over the upstream OpenTelemetry Collector [`influxdb`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`influxdb`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/influxdbreceiver
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.influxdb "influxdb_metrics" {
  endpoint = "localhost:8086"  // InfluxDB metrics ingestion endpoint

  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.influxdb`:

| Name                     | Type                       | Description                                                                  | Default                                                    | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|------------------------------------------------------------|----------|
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                                                            | no       |
| `compression_algorithms` | `list(string)`             | A list of compression algorithms the server can accept.                      | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"localhost:8086"`                                         | no       |
| `include_metadata`       | `bool`                     | Propagate incoming connection metadata to downstream consumers.              | `false`                                                    | no       |
| `keep_alives_enabled`    | `boolean`                  | Whether or not HTTP keep-alives are enabled                                  | `true`                                                     | no       |
| `max_request_body_size`  | `string`                   | Maximum request body size the server will allow.                             | `"20MiB"`                                                  | no       |

By default, `otelcol.receiver.influxdb` listens for HTTP connections on `localhost`.
To expose the HTTP server to other machines on your network, configure `endpoint` with the IP address to listen on, or `0.0.0.0:8086` to listen on all network interfaces.

## Blocks

You can use the following blocks with `otelcol.receiver.influxdb`:

| Block                            | Description                                           | Required |
|----------------------------------|-------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received metrics.            | yes      |
| [`cors`][cors]                   | Configures CORS for the HTTP server.                  | no       |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates. | no       |
| [`tls`][tls]                     | Configures TLS for the HTTP server.                   | no       |
| `tls` > [`tpm`][tpm]             | Configures TPM settings for the TLS key_file.         | no       |

The > symbol indicates deeper levels of nesting.
For example, `tls` > `tpm` refers to a `tpm` block defined inside a `tls` block.

[tls]: #tls
[tpm]: #tpm
[cors]: #cors
[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `cors`

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

| Name              | Type           | Description                              | Default                | Required |
|-------------------|----------------|------------------------------------------|------------------------|----------|
| `allowed_origins` | `list(string)` | Allowed values for the `Origin` header.  |                        | no       |
| `allowed_headers` | `list(string)` | Accepted headers from CORS requests.     | `["X-Requested-With"]` | no       |
| `max_age`         | `number`       | Configures the `Access-Control-Max-Age`. |                        | no       |

The `allowed_headers` argument specifies which headers are acceptable from a
CORS request. The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers are permitted.

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.influxdb` doesn't export any fields.

## Component health

`otelcol.receiver.influxdb` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.influxdb` doesn't expose any component-specific debug information.

## Example

This example forwards received telemetry through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.influxdb "influxdb_metrics" {
  output {
    metrics = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

This example forwards received telemetry to Prometheus Remote Write (Mimir):

```alloy
otelcol.receiver.influxdb "influxdb_metrics" {
  output {
    metrics = [otelcol.exporter.prometheus.influx_output.input]  // Forward metrics to Prometheus exporter
  }
}

otelcol.exporter.prometheus "influx_output" {
  forward_to = [prometheus.remote_write.mimir.receiver]  // Forward metrics to Prometheus remote write (Mimir)
}

prometheus.remote_write "mimir" {
  endpoint {
    url = "https://prometheus-xxx.grafana.net/api/prom/push"

    basic_auth {
      username = "xxxxx"
      password = "xxxx=="
    }
  }
}
```

## Enable authentication

You can create a `otelcol.receiver.influxdb` component that requires authentication for requests.
This is useful for limiting who can push data to the server.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.receiver.influxdb "influxdb_metrics" {
  output {
    metrics = [...]
  }
  auth = otelcol.auth.basic.creds.handler
}

otelcol.auth.basic "creds" {
    username = sys.env("<USERNAME>")
    password = sys.env("<PASSWORD>")
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.influxdb` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
