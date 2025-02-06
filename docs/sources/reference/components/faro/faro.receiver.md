---
canonical: https://grafana.com/docs/alloy/latest/reference/components/faro/faro.receiver/
aliases:
  - ../faro.receiver/ # /docs/alloy/latest/reference/components/faro.receiver/
description: Learn about the faro.receiver
labels:
  stage: general-availability
title: faro.receiver
---

# `faro.receiver`

`faro.receiver` accepts web application telemetry data from the [Grafana Faro Web SDK][faro-sdk] and forwards it to other components for processing.

[faro-sdk]: https://github.com/grafana/faro-web-sdk

## Usage

```alloy
faro.receiver "<LABEL>" {
    output {
        logs   = [<LOKI_RECEIVERS>]
        traces = [<OTELCOL_COMPONENTS>]
    }
}
```

## Arguments

You can use the following arguments with `faro.receiver`:

| Name               | Type          | Description                                  | Default  | Required |
| ------------------ | ------------- | -------------------------------------------- | -------- | -------- |
| `extra_log_labels` | `map(string)` | Extra labels to attach to emitted log lines. | `{}`     | no       |
| `log_format`       | `string`      | Export format for the logs.                  | `logfmt` | no       |

### Log format

The following strings are valid log line formats:

* `"json"`: Export logs as JSON objects.
* `"logfmt"`: Export logs as [`logfmt`](https://brandur.org/logfmt) lines.

## Blocks

You can use the following blocks with `faro.receiver`:

| Block                                        | Description                                          | Required |
| -------------------------------------------- | ---------------------------------------------------- | -------- |
| [`output`][output]                           | Configures where to send collected telemetry data.   | yes      |
| [`server`][server]                           | Configures the HTTP server.                          | no       |
| `server` >  [`rate_limiting`][rate_limiting] | Configures rate limiting for the HTTP server.        | no       |
| [`sourcemaps`][sourcemaps]                   | Configures sourcemap retrieval.                      | no       |
| `sourcemaps` >  [`location`][location]       | Configures on-disk location for sourcemap retrieval. | no       |

The > symbol indicates deeper levels of nesting.
For example, `sourcemaps` > `location` refers to a `location` block defined inside an `sourcemaps` block.

[location]: #location
[output]: #output
[rate_limiting]: #rate_limiting
[server]: #server
[sourcemaps]: #sourcemaps

### `output`

The `output` block specifies where to forward collected logs and traces.

| Name     | Type                     | Description                                          | Default | Required |
| -------- | ------------------------ | ---------------------------------------------------- | ------- | -------- |
| `logs`   | `list(LogsReceiver)`     | A list of `loki` components to forward logs to.      | `[]`    | no       |
| `traces` | `list(otelcol.Consumer)` | A list of `otelcol` components to forward traces to. | `[]`    | no       |

### `server`

The `server` block configures the HTTP server managed by the `faro.receiver` component.
Clients using the [Grafana Faro Web SDK][faro-sdk] forward telemetry data to this HTTP server for processing.

| Name                       | Type           | Description                                                     | Default     | Required |
| -------------------------- | -------------- | --------------------------------------------------------------- | ----------- | -------- |
| `listen_address`           | `string`       | Address to listen for HTTP traffic on.                          | `127.0.0.1` | no       |
| `listen_port`              | `number`       | Port to listen for HTTP traffic on.                             | `12347`     | no       |
| `cors_allowed_origins`     | `list(string)` | Origins for which cross-origin requests are permitted.          | `[]`        | no       |
| `api_key`                  | `secret`       | Optional API key to validate client requests with.              | `""`        | no       |
| `max_allowed_payload_size` | `string`       | Maximum size (in bytes) for client requests.                    | `"5MiB"`    | no       |
| `include_metadata`         | `boolean`      | Propagate incoming connection metadata to downstream consumers. | `false`     | no       |

By default, telemetry data is only accepted from applications on the same local network as the browser.
To accept telemetry data from a wider set of clients, modify the `listen_address` attribute to the IP address of the appropriate network interface to use.

The `cors_allowed_origins` argument determines what origins browser requests may come from.
The default value, `[]`, disables CORS support.
To support requests from all origins, set `cors_allowed_origins` to `["*"]`.
The `*` character indicates a wildcard.

When the `api_key` argument is non-empty, client requests must have an HTTP header called `X-API-Key` matching the value of the `api_key` argument.
Requests that are missing the header or have the wrong value are rejected with an `HTTP 401 Unauthorized` status code.
If the `api_key` argument is empty, no authentication checks are performed, and the `X-API-Key` HTTP header is ignored.

#### `rate_limiting`

The `rate_limiting` block configures rate limiting for client requests.

| Name         | Type     | Description                          | Default | Required |
| ------------ | -------- | ------------------------------------ | ------- | -------- |
| `enabled`    | `bool`   | Whether to enable rate limiting.     | `true`  | no       |
| `rate`       | `number` | Rate of allowed requests per second. | `50`    | no       |
| `burst_size` | `number` | Allowed burst size of requests.      | `100`   | no       |

Rate limiting functions as a [token bucket algorithm][token-bucket], where a bucket has a maximum capacity for up to `burst_size` requests and refills at a rate of `rate` per second.

Each HTTP request drains the capacity of the bucket by one. After the bucket is empty, HTTP requests are rejected with an `HTTP 429 Too Many Requests` status code until the bucket has more available capacity.

Configuring the `rate` argument determines how fast the bucket refills, and configuring the `burst_size` argument determines how many requests can be received in a burst before the bucket is empty and starts rejecting requests.

[token-bucket]: https://en.wikipedia.org/wiki/Token_bucket

### `sourcemaps`

The `sourcemaps` block configures how to retrieve sourcemaps.
Sourcemaps are then used to transform file and line information from minified code into the file and line information from the original source code.

| Name                    | Type           | Description                                | Default | Required |
| ----------------------- | -------------- | ------------------------------------------ | ------- | -------- |
| `download`              | `bool`         | Whether to download sourcemaps.            | `true`  | no       |
| `download_from_origins` | `list(string)` | Which origins to download sourcemaps from. | `["*"]` | no       |
| `download_timeout`      | `duration`     | Timeout when downloading sourcemaps.       | `"1s"`  | no       |

When exceptions are sent to the `faro.receiver` component, it can download sourcemaps from the web application.
You can disable this behavior by setting the `download` argument to `false`.

The `download_from_origins` argument determines which origins a sourcemap may be downloaded from.
The origin is attached to the URL that a browser is sending telemetry data from.
The default value, `["*"]`, enables downloading sourcemaps from all origins.
The `*` character indicates a wildcard.

By default, sourcemap downloads are subject to a timeout of `"1s"`, specified by the `download_timeout` argument.
Setting `download_timeout` to `"0s"` disables timeouts.

To retrieve sourcemaps from disk instead of the network, specify one or more [`location` blocks][location].
When `location` blocks are provided, they're checked first for sourcemaps before falling back to downloading.

#### `location`

The `location` block declares a location where sourcemaps are stored on the filesystem.
You can specify the `location` block multiple times to declare multiple locations where sourcemaps are stored.

| Name                   | Type     | Description                                         | Default | Required |
| ---------------------- | -------- | --------------------------------------------------- | ------- | -------- |
| `minified_path_prefix` | `string` | The prefix of the minified path sent from browsers. |         | yes      |
| `path`                 | `string` | The path on disk where sourcemaps are stored.       |         | yes      |

The `minified_path_prefix` argument determines the prefix of paths to JavaScript files, such as `http://example.com/`.
The `path` argument then determines where to find the sourcemap for the file.

For example, given the following location block:

```alloy
location {
    path                 = "/var/my-app/build"
    minified_path_prefix = "http://example.com/"
}
```

To look up the sourcemaps for a file hosted at `http://example.com/example.js`, the `faro.receiver` component:

1. Removes the minified path prefix to extract the path to the file `example.js`.
1. Searches the path for the file with a `.map` extension, for example `example.js.map` in the path `/var/my-app/build/example.js.map`.

Optionally, the value for the `path` argument may contain `{{ .Release }}` as a template value, such as `/var/my-app/{{ .Release }}/build`.
The template value is replaced with the release value provided by the [Faro Web App SDK][faro-sdk].

## Exported fields

`faro.receiver` doesn't export any fields.

## Component health

`faro.receiver` is reported as unhealthy when the integrated server fails to start.

## Debug information

`faro.receiver` doesn't expose any component-specific debug information.

## Debug metrics

`faro.receiver` exposes the following metrics for monitoring the component:

* `faro_receiver_logs_total` (counter): Total number of ingested logs.
* `faro_receiver_measurements_total` (counter): Total number of ingested measurements.
* `faro_receiver_exceptions_total` (counter): Total number of ingested exceptions.
* `faro_receiver_events_total` (counter): Total number of ingested events.
* `faro_receiver_exporter_errors_total` (counter): Total number of errors produced by an internal exporter.
* `faro_receiver_request_duration_seconds` (histogram): Time (in seconds) spent serving HTTP requests.
* `faro_receiver_request_message_bytes` (histogram): Size (in bytes) of HTTP requests received from clients.
* `faro_receiver_response_message_bytes` (histogram): Size (in bytes) of HTTP responses sent to clients.
* `faro_receiver_inflight_requests` (gauge): Current number of inflight requests.
* `faro_receiver_sourcemap_cache_size` (counter): Number of items in sourcemap cache per origin.
* `faro_receiver_sourcemap_downloads_total` (counter): Total number of sourcemap downloads performed per origin and status.
* `faro_receiver_sourcemap_file_reads_total` (counter): Total number of sourcemap retrievals using the filesystem per origin and status.

## Example

```alloy
faro.receiver "default" {
    server {
        listen_address = "<NETWORK_ADDRESS>"
    }

    sourcemaps {
        location {
            path                 = "<PATH_TO_SOURCEMAPS>"
            minified_path_prefix = "<WEB_APP_PREFIX>"
        }
    }

    output {
        logs   = [loki.write.default.receiver]
        traces = [otelcol.exporter.otlp.traces.input]
    }
}

loki.write "default" {
    endpoint {
        url = "https://LOKI_ADDRESS/loki/api/v1/push"
    }
}

otelcol.exporter.otlp "traces" {
    client {
        endpoint = "<OTLP_ADDRESS>"
    }
}
```

Replace the following:

* _`<NETWORK_ADDRESS>`_: The IP address of the network interface to listen to traffic on.
  This IP address must be reachable by browsers using the web application to instrument.
* _`<PATH_TO_SOURCEMAPS>`_: The path on disk where sourcemaps are located.
* _`<WEB_APP_PREFIX>`_: Prefix of the web application being instrumented.
* `LOKI_ADDRESS`: Address of the Loki server to send logs to.
  Refer to [`loki.write`][loki.write] if you want to use authentication to send logs to the Loki server.
* _`<OTLP_ADDRESS>`_: The address of the OTLP-compatible server to send traces to.
  Refer to[`otelcol.exporter.otlp`][otelcol.exporter.otlp] if you want to use authentication to send logs to the Loki server.

[loki.write]: ../../loki/loki.write/
[otelcol.exporter.otlp]: ../../otelcol/otelcol.exporter.otlp/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`faro.receiver` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)
- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
