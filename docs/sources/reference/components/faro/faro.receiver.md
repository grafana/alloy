---
canonical: https://grafana.com/docs/alloy/latest/reference/components/faro/faro.receiver/
aliases:
  - ../faro.receiver/ # /docs/alloy/latest/reference/components/faro.receiver/
description: Learn about the faro.receiver
title: faro.receiver
---

# faro.receiver

`faro.receiver` accepts web application telemetry data from the [Grafana Faro Web SDK][faro-sdk] and forwards it to other components for future processing.

[faro-sdk]: https://github.com/grafana/faro-web-sdk

## Usage

```alloy
faro.receiver "LABEL" {
    output {
        logs   = [LOKI_RECEIVERS]
        traces = [OTELCOL_COMPONENTS]
    }
}
```

## Arguments

The following arguments are supported:

Name               | Type          | Description                                  | Default | Required
-------------------|---------------|----------------------------------------------|---------|---------
`extra_log_labels` | `map(string)` | Extra labels to attach to emitted log lines. | `{}`    | no

## Blocks

The following blocks are supported inside the definition of `faro.receiver`:

Hierarchy              | Block             | Description                                          | Required
-----------------------|-------------------|------------------------------------------------------|---------
server                 | [server][]        | Configures the HTTP server.                          | no
server > rate_limiting | [rate_limiting][] | Configures rate limiting for the HTTP server.        | no
sourcemaps             | [sourcemaps][]    | Configures sourcemap retrieval.                      | no
sourcemaps > location  | [location][]      | Configures on-disk location for sourcemap retrieval. | no
output                 | [output][]        | Configures where to send collected telemetry data.   | yes

[server]: #server-block
[rate_limiting]: #rate_limiting-block
[sourcemaps]: #sourcemaps-block
[location]: #location-block
[output]: #output-block

### server block

The `server` block configures the HTTP server managed by the `faro.receiver` component.
Clients using the [Grafana Faro Web SDK][faro-sdk] forward telemetry data to this HTTP server for processing.

Name                       | Type           | Description                                                     | Default     | Required
---------------------------|----------------|-----------------------------------------------------------------|-------------|---------
`listen_address`           | `string`       | Address to listen for HTTP traffic on.                          | `127.0.0.1` | no
`listen_port`              | `number`       | Port to listen for HTTP traffic on.                             | `12347`     | no
`cors_allowed_origins`     | `list(string)` | Origins for which cross-origin requests are permitted.          | `[]`        | no
`api_key`                  | `secret`       | Optional API key to validate client requests with.              | `""`        | no
`max_allowed_payload_size` | `string`       | Maximum size (in bytes) for client requests.                    | `"5MiB"`    | no
`include_metadata`         | `boolean`      | Propagate incoming connection metadata to downstream consumers. | `false`     | no

By default, telemetry data is only accepted from applications on the same local network as the browser.
To accept telemetry data from a wider set of clients, modify the `listen_address` attribute to the IP address of the appropriate network interface to use.

The `cors_allowed_origins` argument determines what origins browser requests may come from.
The default value, `[]`, disables CORS support.
To support requests from all origins, set `cors_allowed_origins` to `["*"]`.
The `*` character indicates a wildcard.

When the `api_key` argument is non-empty, client requests must have an HTTP header called `X-API-Key` matching the value of the `api_key` argument.
Requests that are missing the header or have the wrong value are rejected with an `HTTP 401 Unauthorized` status code.
If the `api_key` argument is empty, no authentication checks are performed, and the `X-API-Key` HTTP header is ignored.

### rate_limiting block

The `rate_limiting` block configures rate limiting for client requests.

Name         | Type     | Description                          | Default | Required
-------------|----------|--------------------------------------|---------|---------
`enabled`    | `bool`   | Whether to enable rate limiting.     | `true`  | no
`rate`       | `number` | Rate of allowed requests per second. | `50`    | no
`burst_size` | `number` | Allowed burst size of requests.      | `100`   | no

Rate limiting functions as a [token bucket algorithm][token-bucket], where a bucket has a maximum capacity for up to `burst_size` requests and refills at a rate of `rate` per second.

Each HTTP request drains the capacity of the bucket by one. Once the bucket is empty, HTTP requests are rejected with an `HTTP 429 Too Many Requests` status code until the bucket has more available capacity.

Configuring the `rate` argument determines how fast the bucket refills, and configuring the `burst_size` argument determines how many requests can be received in a burst before the bucket is empty and starts rejecting requests.

[token-bucket]: https://en.wikipedia.org/wiki/Token_bucket

### sourcemaps block

The `sourcemaps` block configures how to retrieve sourcemaps.
Sourcemaps are then used to transform file and line information from minified code into the file and line information from the original source code.

Name                    | Type           | Description                                | Default | Required
------------------------|----------------|--------------------------------------------|---------|---------
`download`              | `bool`         | Whether to download sourcemaps.            | `true`  | no
`download_from_origins` | `list(string)` | Which origins to download sourcemaps from. | `["*"]` | no
`download_timeout`      | `duration`     | Timeout when downloading sourcemaps.       | `"1s"`  | no

When exceptions are sent to the `faro.receiver` component, it can download sourcemaps from the web application.
You can disable this behavior by setting the `download` argument to `false`.

The `download_from_origins` argument determines which origins a sourcemap may be downloaded from.
The origin is attached to the URL that a browser is sending telemetry data from.
The default value, `["*"]`, enables downloading sourcemaps from all origins.
The `*` character indicates a wildcard.

By default, sourcemap downloads are subject to a timeout of `"1s"`, specified by the `download_timeout` argument.
Setting `download_timeout` to `"0s"` disables timeouts.

To retrieve sourcemaps from disk instead of the network, specify one or more [`location` blocks][location].
When `location` blocks are provided, they are checked first for sourcemaps before falling back to downloading.

### location block

The `location` block declares a location where sourcemaps are stored on the filesystem.
The `location` block can be specified multiple times to declare multiple locations where sourcemaps are stored.

Name                   | Type     | Description                                         | Default | Required
-----------------------|----------|-----------------------------------------------------|---------|---------
`path`                 | `string` | The path on disk where sourcemaps are stored.       |         | yes
`minified_path_prefix` | `string` | The prefix of the minified path sent from browsers. |         | yes

The `minified_path_prefix` argument determines the prefix of paths to Javascript files, such as `http://example.com/`.
The `path` argument then determines where to find the sourcemap for the file.

For example, given the following location block:

```
location {
    path                 = "/var/my-app/build"
    minified_path_prefix = "http://example.com/"
}
```

To look up the sourcemaps for a file hosted at `http://example.com/foo.js`, the `faro.receiver` component will:

1. Remove the minified path prefix to extract the path to the file (`foo.js`).
2. Search for that file path with a `.map` extension (`foo.js.map`) in `path` (`/var/my-app/build/foo.js.map`).

Optionally, the value for the `path` argument may contain `{{ .Release }}` as a template value, such as `/var/my-app/{{ .Release }}/build`.
The template value will be replaced with the release value provided by the [Faro Web App SDK][faro-sdk].

### output block

The `output` block specifies where to forward collected logs and traces.

Name     | Type                     | Description                                          | Default | Required
---------|--------------------------|------------------------------------------------------|---------|---------
`logs`   | `list(LogsReceiver)`     | A list of `loki` components to forward logs to.      | `[]`    | no
`traces` | `list(otelcol.Consumer)` | A list of `otelcol` components to forward traces to. | `[]`    | no

## Exported fields

`faro.receiver` does not export any fields.

## Component health

`faro.receiver` is reported as unhealthy when the integrated server fails to
start.

## Debug information

`faro.receiver` does not expose any component-specific debug information.

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
        listen_address = "NETWORK_ADDRESS"
    }

    sourcemaps {
        location {
            path                 = "PATH_TO_SOURCEMAPS"
            minified_path_prefix = "WEB_APP_PREFIX"
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
        endpoint = "OTLP_ADDRESS"
    }
}
```

Replace the following:

* `NETWORK_ADDRESS`: IP address of the network interface to listen to traffic on.
  This IP address must be reachable by browsers using the web application to instrument.

* `PATH_TO_SOURCEMAPS`: Path on disk where sourcemaps are located.

* `WEB_APP_PREFIX`: Prefix of the web application being instrumented.

* `LOKI_ADDRESS`: Address of the Loki server to send logs to.

  * If authentication is required to send logs to the Loki server, refer to the
    documentation of [loki.write][] for more information.

* `OTLP_ADDRESS`: The address of the OTLP-compatible server to send traces to.

  * If authentication is required to send logs to the Loki server, refer to the
    documentation of [otelcol.exporter.otlp][] for more information.

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
