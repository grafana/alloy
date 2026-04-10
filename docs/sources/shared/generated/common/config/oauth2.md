| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `client_id` | `string` | OAuth2 client ID. |  | no |
| `client_secret` | `secret` | OAuth2 client secret. |  | no |
| `client_secret_file` | `string` | File containing the OAuth2 client secret. |  | no |
| `endpoint_params` | `map(string)` | Optional parameters to append to the token URL. |  | no |
| `no_proxy` | `string` | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |  | no |
| `proxy_connect_header` | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests. |  | no |
| `proxy_from_environment` | `bool` | Use the proxy URL indicated by environment variables. |  | no |
| `proxy_url` | `string` | HTTP proxy to send token requests through. |  | no |
| `scopes` | `list(string)` | List of scopes to request during authentication. |  | no |
| `token_url` | `string` | URL to fetch the OAuth2 token from. |  | no |
