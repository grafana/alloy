---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/server-http/
description: Shared content, server http
headless: true
---

The `http` block configures the HTTP server.

You can use the following arguments to configure the `http` block. Any omitted fields take their default values.

| Name                   | Type       | Description                                                                                                      | Default  | Required |
| ---------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------- | -------- | -------- |
| `conn_limit`           | `int`      | Maximum number of simultaneous HTTP connections. Defaults to no limit.                                           | `0`      | no       |
| `listen_address`       | `string`   | Network address on which the server listens for new connections. Defaults to accepting all incoming connections. | `""`     | no       |
| `listen_port`          | `int`      | Port number on which the server listens for new connections.                                                     | `8080`   | no       |
| `server_idle_timeout`  | `duration` | Idle timeout for HTTP server.                                                                                    | `"120s"` | no       |
| `server_read_timeout`  | `duration` | Read timeout for HTTP server.                                                                                    | `"30s"`  | no       |
| `server_write_timeout` | `duration` | Write timeout for HTTP server.                                                                                   | `"30s"`  | no       |

{{< admonition type="caution" >}}
When upgrading from the legacy HTTP/2 handler to the native Go HTTP/2 server, review these changes if you already use an `http2` block with `enabled = true`:

- Clients that require the HTTP/1.1 `Upgrade: h2c` handshake must switch to HTTP/2 with prior knowledge or HTTP/2 over TLS. Clients that support HTTP/1.1 fallback can continue using HTTP/1.1.
- HTTP/1 and HTTP/2 share an idle timeout. A nonzero `http2.idle_timeout` overrides `server_idle_timeout` for both protocols and logs a deprecation warning. Move this setting to `server_idle_timeout`. A zero `http2.idle_timeout` now inherits the server idle timeout instead of disabling the HTTP/2 idle timeout.
- Existing HTTP/2 settings also apply to TLS connections. Review any settings you previously used only for unencrypted connections.

The `max_handlers` setting remains accepted but has no effect. Remove it to avoid a deprecation warning when HTTP/2 is enabled.
{{< /admonition >}}
