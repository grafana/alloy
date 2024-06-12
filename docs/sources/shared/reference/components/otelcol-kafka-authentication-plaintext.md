---
description: Shared content, otelcol Kafka plaintext authentication
headless: true
---

The `plaintext` block configures plain text authentication against Kafka brokers.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | Username to use for plain text authentication. | | yes
`password` | `secret` | Password to use for plain text authentication. | | yes
