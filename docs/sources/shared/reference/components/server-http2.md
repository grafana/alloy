---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/server-http2/
description: Shared content, server HTTP/2
headless: true
---

The `http2` block configures unencrypted HTTP/2 on the HTTP listener.
Set the `enabled` argument to `true` to accept HTTP/2 connections alongside HTTP/1.
Clients can start HTTP/2 directly with prior knowledge or request an HTTP/1.1 upgrade using `Upgrade: h2c`.

HTTP/2 over TLS is available independently of this block.
The settings in this block apply only to unencrypted HTTP/2 connections.

You can use the following arguments with the `http2` block:

| Name | Type | Description | Default | Required |
| ---- | ---- | ----------- | ------- | -------- |
| `enabled` | `bool` | Accept unencrypted HTTP/2 connections. | `false` | no |
| `idle_timeout` | `duration` | Time before closing an idle HTTP/2 connection. | `"0s"` | no |
| `max_concurrent_streams` | `number` | Maximum concurrent streams per client connection. | `100` | no |
| `max_decoder_header_table_size` | `number` | Maximum header compression table size for decoding, in bytes. | `4096` | no |
| `max_encoder_header_table_size` | `number` | Maximum header compression table size for encoding, in bytes. | `4096` | no |
| `max_handlers` | `number` | Accepted for compatibility, but has no effect. | `0` | no |
| `max_read_frame_size` | `number` | Largest HTTP/2 frame the server accepts, in bytes. | `0` | no |
| `max_upload_buffer_per_connection` | `number` | Initial receive flow-control window per connection, in bytes. | `0` | no |
| `max_upload_buffer_per_stream` | `number` | Initial receive flow-control window per stream, in bytes. | `0` | no |
| `permit_prohibited_ciphers` | `bool` | Permit cipher suites prohibited by HTTP/2. Has no effect on unencrypted connections. | `false` | no |
| `ping_timeout` | `duration` | Time to wait for a health-check ping response before closing the connection. | `"15s"` | no |
| `read_idle_timeout` | `duration` | Time without a received frame before sending a health-check ping. | `"0s"` | no |
| `write_byte_timeout` | `duration` | Time without write progress before closing a connection with pending data. | `"0s"` | no |

The `idle_timeout` argument is independent of `server_idle_timeout` in the parent `http` block.
A zero or negative value disables the HTTP/2 idle timeout.
Ping frames don't count as activity for this timeout.

The `read_idle_timeout` argument disables health checks when set to zero.
The `ping_timeout` argument uses a default of `"15s"` when set to zero.
The `write_byte_timeout` argument disables its timeout when set to zero or a negative value.

For `max_concurrent_streams`, zero selects the HTTP/2 implementation's default limit.
For either header table size, zero selects a default of `4096` bytes.
For `max_read_frame_size` and the upload buffer sizes, zero selects the implementation's default.
