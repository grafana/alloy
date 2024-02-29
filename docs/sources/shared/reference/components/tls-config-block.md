---
aliases:
- /docs/agent/shared/flow/reference/components/tls-config-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/tls-config-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/tls-config-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/tls-config-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/tls-config-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/tls-config-block/
description: Shared content, tls config block
headless: true
---

Name                   | Type     | Description                                              | Default | Required
-----------------------|----------|----------------------------------------------------------|---------|---------
`ca_pem`               | `string` | CA PEM-encoded text to validate the server with.         |         | no
`ca_file`              | `string` | CA certificate to validate the server with.              |         | no
`cert_pem`             | `string` | Certificate PEM-encoded text for client authentication.  |         | no
`cert_file`            | `string` | Certificate file for client authentication.              |         | no
`insecure_skip_verify` | `bool`   | Disables validation of the server certificate.           |         | no
`key_file`             | `string` | Key file for client authentication.                      |         | no
`key_pem`              | `secret` | Key PEM-encoded text for client authentication.          |         | no
`min_version`          | `string` | Minimum acceptable TLS version.                          |         | no
`server_name`          | `string` | ServerName extension to indicate the name of the server. |         | no

The following pairs of arguments are mutually exclusive and can't both be set simultaneously:

* `ca_pem` and `ca_file`
* `cert_pem` and `cert_file`
* `key_pem` and `key_file`

When configuring client authentication, both the client certificate (using
`cert_pem` or `cert_file`) and the client key (using `key_pem` or `key_file`)
must be provided.

When `min_version` is not provided, the minimum acceptable TLS version is
inherited from Go's default minimum version, TLS 1.2. If `min_version` is
provided, it must be set to one of the following strings:

* `"TLS10"` (TLS 1.0)
* `"TLS11"` (TLS 1.1)
* `"TLS12"` (TLS 1.2)
* `"TLS13"` (TLS 1.3)
