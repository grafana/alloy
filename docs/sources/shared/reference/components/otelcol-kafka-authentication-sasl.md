---
description: Shared content, otelcol Kafka SASL authentication
headless: true
---

The `sasl` block configures SASL authentication against Kafka brokers.

The following arguments are supported:

| Name        | Type     | Description                                              | Default | Required |
| ----------- | -------- | -------------------------------------------------------- | ------- | -------- |
| `mechanism` | `string` | SASL mechanism to use when authenticating.               |         | yes      |
| `password`  | `secret` | Password to use for SASL authentication.                 |         | yes      |
| `username`  | `string` | Username to use for SASL authentication.                 |         | yes      |
| `version`   | `number` | (Deprecated) Version of the SASL Protocol to use when authenticating. | `0`     | no       |

You can set the `mechanism` argument to one of the following strings:

* `"PLAIN"`
* `"SCRAM-SHA-256"`
* `"SCRAM-SHA-512"`
* `"AWS_MSK_IAM_OAUTHBEARER"`

When `mechanism` is set to `"AWS_MSK_IAM_OAUTHBEARER"`, the `aws_msk` child block must also be provided.

`version` has been deprecated. The client negotiates the SASL handshake version automatically; setting `version` doesn't produce an error, but has no effect.
