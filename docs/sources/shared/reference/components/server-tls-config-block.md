---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/server-tls-config-block/
description: Shared content, tls config block
headless: true
---

| Name               | Type     | Description                                                      | Default          | Required |
| ------------------ | -------- | ---------------------------------------------------------------- | ---------------- | -------- |
| `cert_pem`         | `string` | PEM data of the server TLS certificate.                          | `""`             | no       |
| `cert_file`        | `string` | Path to the server TLS certificate on disk.                      | `""`             | no       |
| `client_auth_type` | `string` | Client authentication to use.                                    | `"NoClientCert"` | no       |
| `client_ca_file`   | `string` | Path to the client CA file on disk to validate requests against. | `""`             | no       |
| `client_ca_pem`    | `string` | PEM data of the client CA to validate requests against.          | `""`             | no       |
| `key_file`         | `string` | Path to the server TLS key on disk.                              | `""`             | no       |
| `key_pem`          | `secret` | PEM data of the server TLS key.                                  | `""`             | no       |

The following pairs of arguments are mutually exclusive and can't both be set simultaneously:

* `cert_pem` and `cert_file`
* `key_pem` and `key_file`

When configuring client authentication, both the client certificate (using `cert_pem` or `cert_file`) and the client key (using `key_pem` or `key_file`) must be provided.
