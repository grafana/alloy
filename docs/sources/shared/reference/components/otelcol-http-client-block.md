---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-http-client-block/
description: Shared content, otelcol http client block
headless: true
---

The following arguments are supported:

| Name                      | Type                       | Description                                                                                                        | Default    | Required |
| ------------------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------- | -------- |
| `endpoint`                | `string`                   | The target URL to send telemetry data to.                                                                          |            | yes      |
| `auth`                    | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests.                                       |            | no       |
| `compression`             | `string`                   | Compression mechanism to use for requests.                                                                         | `"gzip"`   | no       |
| `disable_keep_alives`     | `bool`                     | Disable HTTP keep-alive.                                                                                           | `false`    | no       |
| `force_attempt_http2`     | `bool`                     | Force the HTTP client to try to use the HTTP/2 protocol.                                                           | `true`     | no       |
| `headers`                 | `map(string)`              | Additional headers to send with the request.                                                                       | `{}`       | no       |
| `http2_ping_timeout`      | `duration`                 | Timeout after which the connection will be closed if a response to Ping isn't received.                            | `"15s"`    | no       |
| `http2_read_idle_timeout` | `duration`                 | Timeout after which a health check using ping frame will be carried out if no frame is received on the connection. | `"0s"`     | no       |
| `idle_conn_timeout`       | `duration`                 | Time to wait before an idle connection closes itself.                                                              | `"90s"`    | no       |
| `max_conns_per_host`      | `int`                      | Limits the total (dialing,active, and idle) number of connections per host.                                        | `0`        | no       |
| `max_idle_conns_per_host` | `int`                      | Limits the number of idle HTTP connections the host can keep open.                                                 | `0`        | no       |
| `max_idle_conns`          | `int`                      | Limits the number of idle HTTP connections the client can keep open.                                               | `100`      | no       |
| `proxy_url`               | `string`                   | HTTP proxy to send requests through.                                                                               |            | no       |
| `read_buffer_size`        | `string`                   | Size of the read buffer the HTTP client uses for reading server responses.                                         | `0`        | no       |
| `timeout`                 | `duration`                 | Time to wait before marking a request as failed.                                                                   | `"30s"`    | no       |
| `write_buffer_size`       | `string`                   | Size of the write buffer the HTTP client uses for writing requests.                                                | `"512KiB"` | no       |

When setting `headers`, note that:

* Certain headers such as `Content-Length` and `Connection` are automatically written when needed and values in `headers` may be ignored.
* The `Host` header is automatically derived from the `endpoint` value. However, this automatic assignment can be overridden by explicitly setting a `Host` header in `headers`.

Setting `disable_keep_alives` to `true` will result in significant overhead establishing a new HTTP or HTTPS connection for every request.
Before enabling this option, consider whether changes to idle connection settings can achieve your goal.

If `http2_ping_timeout` is unset or set to `0s`, it will default to `15s`.

If `http2_read_idle_timeout` is unset or set to `0s`, then no health check will be performed.

Golang's default HTTP transport attempts HTTP/2 by default, however some settings (`max_conns_per_host`, `max_idle_conns_per_host`, `max_idle_conns`) are only relevant for HTTP/1.
The `force_attempt_http2` attribute allows a user to only attempt HTTP/1.

{{< docs/shared lookup="reference/components/otelcol-compression-field.md" source="alloy" version="<ALLOY_VERSION>" >}}
