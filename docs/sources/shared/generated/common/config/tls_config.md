| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `ca_file` | `string` | Path to the CA certificate file. |  | no |
| `ca_pem` | `string` | CA certificate in PEM format. |  | no |
| `cert_file` | `string` | Path to the client certificate file. |  | no |
| `cert_pem` | `string` | Client certificate in PEM format. |  | no |
| `insecure_skip_verify` | `bool` | Whether to skip TLS certificate verification. |  | no |
| `key_file` | `string` | Path to the client key file. |  | no |
| `key_pem` | `secret` | Client key in PEM format. |  | no |
| `min_version` | `string` | Minimum acceptable TLS version. |  | no |
| `server_name` | `string` | Server name used for TLS certificate verification. |  | no |
