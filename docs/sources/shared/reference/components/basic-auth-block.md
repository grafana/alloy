---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/basic-auth-block/
description: Shared content, basic auth block
headless: true
---

| Name            | Type     | Description                              | Default | Required |
| --------------- | -------- | ---------------------------------------- | ------- | -------- |
| `password_file` | `string` | File containing the basic auth password. |         | no       |
| `password`      | `secret` | Basic auth password.                     |         | no       |
| `username`      | `string` | Basic auth username.                     |         | no       |

`password` and `password_file` are mutually exclusive, and only one can be provided inside a `basic_auth` block.

{{< admonition type="warning" >}}
Using `password_file` causes the file to be read on every outgoing request.
Use the `local.file` component with the `password` attribute instead to avoid unnecessary reads.
{{< /admonition >}}
