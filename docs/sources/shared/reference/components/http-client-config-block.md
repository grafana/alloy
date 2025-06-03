---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/http-client-config-block/
description: Shared content, http client config block
headless: true
---

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |

`bearer_token`, `bearer_token_file`, `basic_auth`, `authorization`, and `oauth2` are mutually exclusive, and only one can be provided inside of a `http_client_config` block.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}
