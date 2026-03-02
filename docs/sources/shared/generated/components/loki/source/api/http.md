| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `conn_limit` | `int` | Maximum number of simultaneous HTTP connections. Defaults to no limit. | `0` | no |
| `listen_address` | `string` | Network address on which the server listens for new connections. Defaults to accepting all incoming connections. | `""` | no |
| `listen_port` | `int` | Port number on which the server listens for new connections. | `8080` | no |
| `server_idle_timeout` | `duration` | Idle timeout for HTTP server. | `"120s"` | no |
| `server_read_timeout` | `duration` | Read timeout for HTTP server. | `"30s"` | no |
| `server_write_timeout` | `duration` | Write timeout for HTTP server. | `"30s"` | no |
