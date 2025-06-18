---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/http/
description: Learn about the http configuration block
labels:
  stage: general-availability
  products:
    - oss
title: http
---

# `http`

`http` is an optional configuration block used to customize how the {{< param "PRODUCT_NAME" >}} HTTP server functions.
`http` is specified without a label and can only be provided once per configuration file.

## Usage

```alloy
http {

}
```

## Arguments

The `http` block doesn't support any arguments. You can configure this block with inner blocks.

## Blocks

You can use the following blocks with `http`:

| Block                                                              | Description                                                   | Required |
| ------------------------------------------------------------------ | ------------------------------------------------------------- | -------- |
| [`auth`][auth]                                                     | Configure server authentication.                              | no       |
| `auth` > [`basic`][basic]                                          | Configure basic authentication.                               | no       |
| `auth` > [`filter`][filter]                                        | Configure authentication filter.                              | no       |
| [`tls`][tls]                                                       | Define TLS settings for the HTTP server.                      | no       |
| `tls` > [`windows_certificate_filter`][windows_certificate_filter] | Configure Windows certificate store for all certificates.     | no       |
| `tls` > `windows_certificate_filter` > [`client`][client]          | Configure client certificates for Windows certificate filter. | no       |
| `tls` > `windows_certificate_filter` > [`server`][server]          | Configure server certificates for Windows certificate filter. | no       |

The > symbol indicates deeper levels of nesting.
For example, `auth` > `basic` refers to an `basic` block defined inside an `auth` block.

[auth]: #auth
[basic]: #basic
[filter]: #filter
[tls]: #tls
[windows_certificate_filter]: #windows-certificate-filter
[server]: #server
[client]: #client

### `auth`

The auth block configures server authentication for the `http` block.
This can be used to enable basic authentication and to set authentication filters for specified API paths.

### `basic`

The `basic` block enables basic HTTP authentication by requiring both a username and password for access.

| Name       | Type     | Description                                   | Default | Required |
| ---------- | -------- | --------------------------------------------- | ------- | -------- |
| `password` | `secret` | The password to use for basic authentication. |         | yes      |
| `username` | `string` | The username to use for basic authentication. |         | yes      |

### `filter`

The `filter` block is used to configure which API paths should be protected by authentication.
It allows you to specify a list of paths, using prefix matching, that will require authentication.

| Name                          | Type           | Description                                                                                                           | Default | Required |
| ----------------------------- | -------------- | --------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `authenticate_matching_paths` | `bool`         | If `true`, authentication is required for all matching paths. If `false`, authentication is excluded for these paths. | `true`  | no       |
| `paths`                       | `list(string)` | List of API paths to be protected by authentication. The paths are matched using prefix matching.                     | `[]`    | no       |

### `tls`

The `tls` block configures TLS settings for the HTTP server.

{{< admonition type="warning" >}}
If you add the `tls` block and reload the configuration when {{< param "PRODUCT_NAME" >}} is running, existing connections continue communicating over plain text.
Similarly, if you remove the `tls` block and reload the configuration when {{< param "PRODUCT_NAME" >}} is running, existing connections continue communicating over TLS.

To ensure all connections use TLS, configure the `tls` block before you start {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

| Name                | Type           | Description                                                      | Default          | Required      |
| ------------------- | -------------- | ---------------------------------------------------------------- | ---------------- | ------------- |
| `cert_file`         | `string`       | Path to the server TLS certificate on disk.                      | `""`             | conditionally |
| `cert_pem`          | `string`       | PEM data of the server TLS certificate.                          | `""`             | conditionally |
| `key_file`          | `string`       | Path to the server TLS key on disk.                              | `""`             | conditionally |
| `key_pem`           | `string`       | PEM data of the server TLS key.                                  | `""`             | conditionally |
| `cipher_suites`     | `list(string)` | Set of cipher suites to use.                                     | `[]`             | no            |
| `client_auth_type`  | `string`       | Client authentication to use.                                    | `"NoClientCert"` | no            |
| `client_ca_file`    | `string`       | Path to the client CA file on disk to validate requests against. | `""`             | no            |
| `client_ca_pem`     | `string`       | PEM data of the client CA to validate requests against.          | `""`             | no            |
| `curve_preferences` | `list(string)` | Set of elliptic curves to use in a handshake.                    | `[]`             | no            |
| `max_version`       | `string`       | Newest TLS version to accept from clients.                       | `""`             | no            |
| `min_version`       | `string`       | Oldest TLS version to accept from clients.                       | `""`             | no            |

When the `tls` block is specified, arguments for the TLS certificate (using `cert_pem` or `cert_file`) and for the TLS key (using `key_pem` or `key_file`) are required.

The following pairs of arguments are mutually exclusive, and only one may be configured at a time:

* `cert_pem` and `cert_file`
* `key_pem` and `key_file`
* `client_ca_pem` and `client_ca_file`

The `client_auth_type` argument determines whether to validate client certificates.
The default value, `NoClientCert`, indicates that the client certificate isn't validated.
The `client_ca_pem` and `client_ca_file` arguments may only be configured when `client_auth_type` is not `NoClientCert`.

The following values are accepted for `client_auth_type`:

* `NoClientCert`: client certificates are neither requested nor validated.
* `RequestClientCert`: requests clients to send an optional certificate. Certificates provided by clients aren't validated.
* `RequireAnyClientCert`: requires at least one certificate from clients. Certificates provided by clients aren't validated.
* `VerifyClientCertIfGiven`: requests clients to send an optional certificate. If a certificate is sent, it must be valid.
* `RequireAndVerifyClientCert`: requires clients to send a valid certificate.

The `client_ca_pem` or `client_ca_file` arguments may be used to perform client certificate validation.
These arguments may only be provided when `client_auth_type` isn't set to `NoClientCert`.

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

* `TLS13` for TLS 1.3
* `TLS12` for TLS 1.2
* `TLS11` for TLS 1.1
* `TLS10` for TLS 1.0

### `windows certificate filter`

The `windows_certificate_filter` block is used to configure retrieving certificates from the built-in Windows certificate store.
When you use the `windows_certificate_filter` block the following TLS settings are overridden and cause an error if defined.

* `cert_pem`
* `cert_file`
* `key_pem`
* `key_file`
* `client_ca`
* `client_ca_file`

{{< admonition type="warning" >}}
This feature is only available on Windows.

TLS min and max may not be compatible with the certificate stored in the Windows certificate store.
The `windows_certificate_filter` serves the certificate even if it isn't compatible with the specified TLS version.
{{< /admonition >}}

### `client`

The `client` block is used to check the certificate presented to the server.

| Name                  | Type           | Description                                                       | Default | Required |
| --------------------- | -------------- | ----------------------------------------------------------------- | ------- | -------- |
| `issuer_common_names` | `list(string)` | Issuer common names to check against.                             |         | no       |
| `subject_regex`       | `string`       | Regular expression to match Subject name.                         | `""`    | no       |
| `template_id`         | `string`       | Client Template ID to match in ASN1 format, for example, "1.2.3". | `""`    | no       |

### `server`

The `server` block is used to find the certificate to check the signer.
If multiple certificates are found, the `windows_certificate_filter` chooses the certificate with the expiration farthest in the future.

| Name                  | Type           | Description                                                                                                | Default | Required |
| --------------------- | -------------- | ---------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `store`               | `string`       | Name of the store to look for the server Certificate. For example, `MY` or `CA`.                           | `""`    | yes      |
| `system_store`        | `string`       | Name of the system store to look for the server Certificate. For example, `LocalMachine` or `CurrentUser`. | `""`    | yes      |
| `issuer_common_names` | `list(string)` | Issuer common names to check against.                                                                      |         | no       |
| `refresh_interval`    | `string`       | How often to check for a new server certificate.                                                           | `"5m"`  | no       |
| `template_id`         | `string`       | Server Template ID to match in ASN1 format, for example, "1.2.3".                                          | `""`    | no       |

## Examples

Example of enforcing authentication on `/metrics` and every endpoint that has `/v1` as prefix:

```alloy
http {
  auth {
    basic {
      username = sys.env("BASIC_AUTH_USERNAME")
      password = sys.env("BASIC_AUTH_PASSWORD")
    }

    filter {
      paths                       = ["/metrics", "/v1"]
      authenticate_matching_paths = true
    }
  }
}
```

Example enforcing authentication on all endpoints except `/metrics`:

```alloy
http {
  auth {
    basic {
      username = sys.env("BASIC_AUTH_USERNAME")
      password = sys.env("BASIC_AUTH_PASSWORD")
    }

    filter {
      paths                       = ["/metrics"]
      authenticate_matching_paths = false
    }
  }
}
```
