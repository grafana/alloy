---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/oauth2-block/
description: Shared content, oauth2 block
headless: true
---

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `client_id`              | `string`            | OAuth2 client ID.                                                                                |         | no       |
| `client_secret_file`     | `string`            | File containing the OAuth2 client secret.                                                        |         | no       |
| `client_secret`          | `secret`            | OAuth2 client secret.                                                                            |         | no       |
| `endpoint_params`        | `map(string)`       | Optional parameters to append to the token URL.                                                  |         | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `scopes`                 | `list(string)`      | List of scopes to authenticate with.                                                             |         | no       |
| `token_url`              | `string`            | URL to fetch the token from.                                                                     |         | no       |

`client_secret` and `client_secret_file` are mutually exclusive, and only one can be provided inside an `oauth2` block.

The `oauth2` block may also contain a separate `tls_config` sub-block.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}
