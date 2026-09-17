---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/oauth2-block/
description: Shared content, oauth2 block
headless: true
---

| Name                          | Type                | Description                                                                                                 | Default                | Required |
|-------------------------------|---------------------|-------------------------------------------------------------------------------------------------------------|------------------------|----------|
| `token_url`                   | `string`            | URL to fetch the token from.                                                                                |                        | yes      |
| `grant_type`                  | `string`            | OAuth2 grant type. Valid values: `"client_credentials"` or `"urn:ietf:params:oauth:grant-type:jwt-bearer"`. | `"client_credentials"` | no       |
| `client_id`                   | `string`            | OAuth2 client ID.                                                                                           |                        | no       |
| `client_secret`               | `secret`            | OAuth2 client secret. Used when grant_type `"client_credentials"`.                                          |                        | no       |
| `client_secret_file`          | `string`            | File containing the OAuth2 client secret. Used when grant_type `"client_credentials"`.                      |                        | no       |
| `client_certificate_key`      | `secret`            | JWT bearer private key.                                                                                     | `""`                   | no       |
| `client_certificate_key_file` | `string`            | Path to a file containing the JWT bearer private key.                                                       | `""`                   | no       |
| `client_certificate_key_id`   | `string`            | Key ID included in JWT bearer grant requests.                                                               | `""`                   | no       |
| `iss`                         | `string`            | JWT issuer claim for JWT bearer grant. Defaults to `client_id` when empty.                                  | `""`                   | no       |
| `audience`                    | `string`            | JWT audience claim for JWT bearer grant. Defaults to `token_url` when empty.                                | `""`                   | no       |
| `claims`                      | `map(any)`          | Additional JWT claims for JWT bearer grant.                                                                 | `{}`                   | no       |
| `scopes`                      | `list(string)`      | List of scopes to authenticate with.                                                                        |                        | no       |
| `endpoint_params`             | `map(string)`       | Optional parameters to append to the token URL.                                                             |                        | no       |
| `signature_algorithm`         | `string`            | JWT signing algorithm for JWT bearer grant. Valid values: `RS256`, `RS384`, `RS512`.                        | `"RS256"`              | no       |
| `proxy_url`                   | `string`            | HTTP proxy to send requests through.                                                                        |                        | no       |
| `proxy_connect_header`        | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                               |                        | no       |
| `no_proxy`                    | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying.            |                        | no       |
| `proxy_from_environment`      | `bool`              | Use the proxy URL indicated by environment variables.                                                       | `false`                | no       |


Value of `grant_type` decides which variables will be used inside the `oauth2` block.

`client_secret` and `client_secret_file` are mutually exclusive, and only one can be provided inside an `oauth2` block.

`client_certificate_key` and `client_certificate_key_file` are mutually exclusive, and only one can be provided inside an `oauth2` block.

{{< admonition type="warning" >}}
Using `client_secret_file`/`client_certificate_key_file` causes the file to be read on every outgoing request.
Use the `local.file` component to read the file value once with the `client_secret`/`client_certificate_key_file` attribute instead to avoid unnecessary reads.
{{< /admonition >}}

The `oauth2` block may also contain a separate `tls_config` sub-block.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}
