---
description: Shared content, otelcol Kafka SASL AWS_MSK authentication
headless: true
---

The `aws_msk` block configures extra parameters for SASL authentication when
using the `AWS_MSK_IAM` mechanism.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`region` | `string` | AWS region the MSK cluster is based in. | | yes
`broker_addr` | `string` | MSK address to connect to for authentication. | | yes
