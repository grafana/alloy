---
description: Shared content, otelcol Kafka Kerberos authentication
headless: true
---

The `kerberos` block configures Kerberos authentication against the Kafka
broker.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`service_name` | `string` | Kerberos service name. | | no
`realm` | `string` | Kerberos realm. | | no
`use_keytab` | `string` | Enables using keytab instead of password. | | no
`username` | `string` | Kerberos username to authenticate as. | | yes
`password` | `secret` | Kerberos password to authenticate with. | | no
`config_file` | `string` | Path to Kerberos location (for example, `/etc/krb5.conf`). | | no
`keytab_file` | `string` | Path to keytab file (for example, `/etc/security/kafka.keytab`). | | no

When `use_keytab` is `false`, the `password` argument is required. When
`use_keytab` is `true`, the file pointed to by the `keytab_file` argument is
used for authentication instead. At most one of `password` or `keytab_file`
must be provided.
