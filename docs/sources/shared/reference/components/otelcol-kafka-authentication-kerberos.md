---
description: Shared content, otelcol Kafka Kerberos authentication
headless: true
---

The `kerberos` block configures Kerberos authentication against the Kafka broker.

The following arguments are supported:

| Name                       | Type     | Description                                                     | Default | Required |
| -------------------------- | -------- | --------------------------------------------------------------- | ------- | -------- |
| `config_file`              | `string` | Path to Kerberos location, for example, `/etc/krb5.conf`.       |         | no       |
| `disable_fast_negotiation` | `bool`   | Disable PA-FX-FAST negotiation.                                 | `false` | no       |
| `keytab_file`              | `string` | Path to keytab file, for example, `/etc/security/kafka.keytab`. |         | no       |
| `password`                 | `secret` | Kerberos password to authenticate with.                         |         | no       |
| `realm`                    | `string` | Kerberos realm.                                                 |         | no       |
| `service_name`             | `string` | Kerberos service name.                                          |         | no       |
| `use_keytab`               | `string` | Enables using keytab instead of password.                       |         | no       |
| `username`                 | `string` | Kerberos username to authenticate as.                           |         | yes      |

When `use_keytab` is `false`, the `password` argument is required.
When `use_keytab` is `true`, the file pointed to by the `keytab_file` argument is used for authentication instead.
At most one of `password` or `keytab_file` must be provided.

`disable_fast_negotiation` is useful for Kerberos implementations which don't support PA-FX-FAST (Pre-Authentication Framework - Fast) negotiation.
