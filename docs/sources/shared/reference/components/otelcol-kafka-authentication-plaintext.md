---
description: Shared content, otelcol Kafka plaintext authentication
headless: true
---

The `plaintext` block configures plain text authentication against Kafka brokers.

The following arguments are supported:

| Name       | Type     | Description                                    | Default | Required |
| ---------- | -------- | ---------------------------------------------- | ------- | -------- |
| `password` | `secret` | Password to use for plain text authentication. |         | yes      |
| `username` | `string` | Username to use for plain text authentication. |         | yes      |
