| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `url` | `string` | Full URL to send logs to. |  | yes |
| `batch_size` | `string` | Maximum batch size of logs to accumulate before sending. | `"1MiB"` | no |
| `batch_wait` | `duration` | Maximum amount of time to wait before sending a batch. | `"1s"` | no |
| `bearer_token` | `secret` | Bearer token to authenticate with. |  | no |
| `bearer_token_file` | `string` | File containing a bearer token to authenticate with. |  | no |
| `enable_http2` | `bool` | Whether HTTP2 is supported for requests. | `true` | no |
| `follow_redirects` | `bool` | Whether redirects returned by the server should be followed. | `true` | no |
| `headers` | `map(string)` | Extra headers to deliver with the request. |  | no |
| `http_headers` | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name. |  | no |
| `max_backoff_period` | `duration` | Maximum backoff time between retries. | `"5m"` | no |
| `max_backoff_retries` | `int` | Maximum number of retries. | `10` | no |
| `min_backoff_period` | `duration` | Initial backoff time between retries. | `"500ms"` | no |
| `name` | `string` | Optional name to identify this endpoint with. |  | no |
| `no_proxy` | `string` | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |  | no |
| `proxy_connect_header` | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests. |  | no |
| `proxy_from_environment` | `bool` | Use the proxy URL indicated by environment variables. |  | no |
| `proxy_url` | `string` | HTTP proxy to send requests through. |  | no |
| `remote_timeout` | `duration` | Timeout for requests made to the URL. | `"10s"` | no |
| `retry_on_http_429` | `bool` | Retry when an HTTP 429 status code is received. | `true` | no |
| `tenant_id` | `string` | The tenant ID used by default to push logs. |  | no |
