---
description: Shared content, otelcol Kafka SASL authentication
headless: true
---

The `sasl` block configures SASL authentication against Kafka brokers.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | Username to use for SASL authentication. | | yes
`password` | `secret` | Password to use for SASL authentication. | | yes
`mechanism` | `string` | SASL mechanism to use when authenticating. | | yes
`version` | `number` | Version of the SASL Protocol to use when authenticating. | `0` | no

The `mechanism` argument can be set to one of the following strings:

* `"PLAIN"`
* `"AWS_MSK_IAM"`
* `"SCRAM-SHA-256"`
* `"SCRAM-SHA-512"`

When `mechanism` is set to `"AWS_MSK_IAM"`, the `aws_msk` child block must also be provided.

The `version` argument can be set to either `0` or `1`.
