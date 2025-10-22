---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-tls-server-block/
description: Shared content, otelcol tls server block
headless: true
---

The following arguments are supported:

| Name                           | Type           | Description                                                                                  | Default     | Required |
| ------------------------------ | -------------- | -------------------------------------------------------------------------------------------- | ----------- | -------- |
| `ca_file`                      | `string`       | Path to the CA file.                                                                         |             | no       |
| `ca_pem`                       | `string`       | CA PEM-encoded text to validate the server with.                                             |             | no       |
| `cert_file`                    | `string`       | Path to the TLS certificate.                                                                 |             | no       |
| `cert_pem`                     | `string`       | Certificate PEM-encoded text for client authentication.                                      |             | no       |
| `cipher_suites`                | `list(string)` | A list of TLS cipher suites that the TLS transport can use.                                  | `[]`        | no       |
| `client_ca_file`               | `string`       | Path to the TLS cert to use by the server to verify a client certificate.                    |             | no       |
| `curve_preferences`            | `list(string)` | Set of elliptic curves to use in a handshake.                                                | `[]`        | no       |
| `include_system_ca_certs_pool` | `boolean`      | Whether to load the system certificate authorities pool alongside the certificate authority. | `false`     | no       |
| `key_file`                     | `string`       | Path to the TLS certificate key.                                                             |             | no       |
| `key_pem`                      | `secret`       | Key PEM-encoded text for client authentication.                                              |             | no       |
| `max_version`                  | `string`       | Maximum acceptable TLS version for connections.                                              | `"TLS 1.3"` | no       |
| `min_version`                  | `string`       | Minimum acceptable TLS version for connections.                                              | `"TLS 1.2"` | no       |
| `reload_interval`              | `duration`     | The duration after which the certificate is reloaded.                                        | `"0s"`      | no       |

If `reload_interval` is set to `"0s"`, the certificate never reloaded.

The following pairs of arguments are mutually exclusive and can't both be set simultaneously:

* `ca_pem` and `ca_file`
* `cert_pem` and `cert_file`
* `key_pem` and `key_file`

If `cipher_suites` is left blank, a safe default list is used.
Refer to the [Go Cipher Suites documentation][golang-cipher-suites] for a list of supported cipher suites.

`client_ca_file` sets the `ClientCA` and `ClientAuth` to `RequireAndVerifyClientCert` in the `TLSConfig`. 
Refer to the [Go TLS documentation][golang-tls] for more information.

The `curve_preferences` argument determines the set of elliptic curves to prefer during a handshake in preference order.
If not provided, a default list is used.
The set of elliptic curves available are `X25519`, `P521`, `P256`, and `P384`.

[golang-tls]: https://godoc.org/crypto/tls#Config
[golang-cipher-suites]: https://go.dev/src/crypto/tls/cipher_suites.go
