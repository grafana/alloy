---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/http/
description: Learn about the http configuration block
menuTitle: http
title: http block
---

# http block

`http` is an optional configuration block used to customize how the {{< param "PRODUCT_NAME" >}} HTTP server functions.
`http` is specified without a label and can only be provided once per configuration file.

## Example

```alloy
http {
  tls {
    cert_file = sys.env("TLS_CERT_FILE_PATH")
    key_file  = sys.env("TLS_KEY_FILE_PATH")
  }
}
```

## Arguments

The `http` block supports no arguments and is configured completely through inner blocks.

## Blocks

The following blocks are supported inside the definition of `http`:

| Hierarchy                                 | Block                          | Description                                                   | Required |
| ----------------------------------------- | ------------------------------ | ------------------------------------------------------------- | -------- |
| tls                                       | [tls][]                        | Define TLS settings for the HTTP server.                      | no       |
| tls > windows_certificate_filter          | [windows_certificate_filter][] | Configure Windows certificate store for all certificates.     | no       |
| tls > windows_certificate_filter > client | [client][]                     | Configure client certificates for Windows certificate filter. | no       |
| tls > windows_certificate_filter > server | [server][]                     | Configure server certificates for Windows certificate filter. | no       |

### tls block

The `tls` block configures TLS settings for the HTTP server.

{{< admonition type="warning" >}}
If you add the `tls` block and reload the configuration when {{< param "PRODUCT_NAME" >}} is running, existing connections will continue communicating over plaintext.
Similarly, if you remove the `tls` block and reload the configuration when {{< param "PRODUCT_NAME" >}} is running, existing connections will continue communicating over TLS.

To ensure all connections use TLS, configure the `tls` block before you start {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

| Name                | Type           | Description                                                      | Default          | Required      |
| ------------------- | -------------- | ---------------------------------------------------------------- | ---------------- | ------------- |
| `cert_pem`          | `string`       | PEM data of the server TLS certificate.                          | `""`             | conditionally |
| `cert_file`         | `string`       | Path to the server TLS certificate on disk.                      | `""`             | conditionally |
| `key_pem`           | `string`       | PEM data of the server TLS key.                                  | `""`             | conditionally |
| `key_file`          | `string`       | Path to the server TLS key on disk.                              | `""`             | conditionally |
| `client_ca_pem`     | `string`       | PEM data of the client CA to validate requests against.          | `""`             | no            |
| `client_ca_file`    | `string`       | Path to the client CA file on disk to validate requests against. | `""`             | no            |
| `client_auth_type`  | `string`       | Client authentication to use.                                    | `"NoClientCert"` | no            |
| `cipher_suites`     | `list(string)` | Set of cipher suites to use.                                     | `[]`             | no            |
| `curve_preferences` | `list(string)` | Set of elliptic curves to use in a handshake.                    | `[]`             | no            |
| `min_version`       | `string`       | Oldest TLS version to accept from clients.                       | `""`             | no            |
| `max_version`       | `string`       | Newest TLS version to accept from clients.                       | `""`             | no            |

When the `tls` block is specified, arguments for the TLS certificate (using `cert_pem` or `cert_file`) and for the TLS key (using `key_pem` or `key_file`) are required.

The following pairs of arguments are mutually exclusive, and only one may be configured at a time:

- `cert_pem` and `cert_file`
- `key_pem` and `key_file`
- `client_ca_pem` and `client_ca_file`

The `client_auth_type` argument determines whether to validate client certificates.
The default value, `NoClientCert`, indicates that the client certificate is not validated.
The `client_ca_pem` and `client_ca_file` arguments may only be configured when `client_auth_type` is not `NoClientCert`.

The following values are accepted for `client_auth_type`:

- `NoClientCert`: client certificates are neither requested nor validated.
- `RequestClientCert`: requests clients to send an optional certificate. Certificates provided by clients are not validated.
- `RequireAnyClientCert`: requires at least one certificate from clients. Certificates provided by clients are not validated.
- `VerifyClientCertIfGiven`: requests clients to send an optional certificate. If a certificate is sent, it must be valid.
- `RequireAndVerifyClientCert`: requires clients to send a valid certificate.

The `client_ca_pem` or `client_ca_file` arguments may be used to perform client certificate validation.
These arguments may only be provided when `client_auth_type` is not set to `NoClientCert`.

The `cipher_suites` argument determines what cipher suites to use.
If you don't provide cipher suite, a default list is used.
The set of cipher suites specified may be from the following:

| Cipher                                          | Allowed in BoringCrypto builds |
| ----------------------------------------------- | ------------------------------ |
| `TLS_RSA_WITH_AES_128_CBC_SHA`                  | no                             |
| `TLS_RSA_WITH_AES_256_CBC_SHA`                  | no                             |
| `TLS_RSA_WITH_AES_128_GCM_SHA256`               | yes                            |
| `TLS_RSA_WITH_AES_256_GCM_SHA384`               | yes                            |
| `TLS_AES_128_GCM_SHA256`                        | no                             |
| `TLS_AES_256_GCM_SHA384`                        | no                             |
| `TLS_CHACHA20_POLY1305_SHA256`                  | no                             |
| `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`          | no                             |
| `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`          | no                             |
| `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`            | no                             |
| `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`            | no                             |
| `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`       | yes                            |
| `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`       | yes                            |
| `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`         | yes                            |
| `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`         | yes                            |
| `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`   | no                             |
| `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256` | no                             |

The `curve_preferences` argument determines the set of elliptic curves to prefer during a handshake in preference order.
If not provided, a default list is used.
The set of elliptic curves specified may be from the following:

| Curve       | Allowed in BoringCrypto builds |
| ----------- | ------------------------------ |
| `CurveP256` | yes                            |
| `CurveP384` | yes                            |
| `CurveP521` | yes                            |
| `X25519`    | no                             |

The `min_version` and `max_version` arguments determine the oldest and newest TLS version that's acceptable from clients.
If you don't provide the min and max TLS version, a default value is used.

The following versions are recognized:

- `TLS13` for TLS 1.3
- `TLS12` for TLS 1.2
- `TLS11` for TLS 1.1
- `TLS10` for TLS 1.0

### windows certificate filter block

The `windows_certificate_filter` block is used to configure retrieving certificates from the built-in Windows certificate store.
When you use the `windows_certificate_filter` block the following TLS settings are overridden and cause an error if defined.

- `cert_pem`
- `cert_file`
- `key_pem`
- `key_file`
- `client_ca`
- `client_ca_file`

{{< admonition type="warning" >}}
This feature is only available on Windows.

TLS min and max may not be compatible with the certificate stored in the Windows certificate store.
The `windows_certificate_filter` serves the certificate even if it isn't compatible with the specified TLS version.
{{< /admonition >}}

### server block

The `server` block is used to find the certificate to check the signer.
If multiple certificates are found, the `windows_certificate_filter` chooses the certificate with the expiration farthest in the future.

| Name                  | Type           | Description                                                                                          | Default | Required |
| --------------------- | -------------- | ---------------------------------------------------------------------------------------------------- | ------- | -------- |
| `store`               | `string`       | Name of the system store to look for the server Certificate, for example, LocalMachine, CurrentUser. | `""`    | yes      |
| `system_store`        | `string`       | Name of the store to look for the server Certificate, for example, My, CA.                           | `""`    | yes      |
| `issuer_common_names` | `list(string)` | Issuer common names to check against.                                                                |         | no       |
| `template_id`         | `string`       | Server Template ID to match in ASN1 format, for example, "1.2.3".                                    | `""`    | no       |
| `refresh_interval`    | `string`       | How often to check for a new server certificate.                                                     | `"5m"`  | no       |

### client block

The `client` block is used to check the certificate presented to the server.

| Name                  | Type           | Description                                                       | Default | Required |
| --------------------- | -------------- | ----------------------------------------------------------------- | ------- | -------- |
| `issuer_common_names` | `list(string)` | Issuer common names to check against.                             |         | no       |
| `subject_regex`       | `string`       | Regular expression to match Subject name.                         | `""`    | no       |
| `template_id`         | `string`       | Client Template ID to match in ASN1 format, for example, "1.2.3". | `""`    | no       |

[tls]: #tls-block
[windows_certificate_filter]: #windows-certificate-filter-block
[server]: #server-block
[client]: #client-block
