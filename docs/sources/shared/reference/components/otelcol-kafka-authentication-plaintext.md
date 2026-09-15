---
description: Shared content, otelcol Kafka plaintext authentication
headless: true
---

{{< admonition type="caution" >}}
The `plaintext` block has been deprecated.
Use `sasl` with `mechanism` set to `PLAIN` instead.

The upstream OpenTelemetry Collector Kafka client removed native plaintext authentication support.
Alloy translates the `plaintext` block into a `sasl` configuration with `mechanism` set to `PLAIN` internally, so existing configurations continue to work.
{{< /admonition >}}

The `plaintext` block configures plain text authentication against Kafka brokers.

The following arguments are supported:

| Name       | Type     | Description                                    | Default | Required |
| ---------- | -------- | ---------------------------------------------- | ------- | -------- |
| `password` | `secret` | Password to use for plain text authentication. |         | yes      |
| `username` | `string` | Username to use for plain text authentication. |         | yes      |
