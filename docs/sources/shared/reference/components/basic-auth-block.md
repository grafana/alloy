---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/basic-auth-block/
description: Shared content, basic auth block
headless: true
---

| Name            | Type     | Description                                                                                             | Default | Required |
| --------------- | -------- | ------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `password_file` | `string` | File containing the basic auth password. Read on every outgoing request that uses basic authentication. |         | no       |
| `password`      | `secret` | Basic auth password.                                                                                    |         | no       |
| `username`      | `string` | Basic auth username.                                                                                    |         | no       |

`password` and `password_file` are mutually exclusive, and only one can be provided inside a `basic_auth` block.

This shared `basic_auth` block has the same arguments and the same `password_file` reload behavior in every component that embeds it, unless a component page documents an exception.

{{< admonition type="warning" >}}
Using `password_file` causes the file to be read on every outgoing request. The file isn't limited to a one-time read at startup, so password rotation is picked up automatically, but high-frequency scrapes or writes re-read the file often.

Prefer the `local.file` component with the `password` attribute when you want fewer filesystem reads. `local.file` watches the file for changes and exports the latest content:

```alloy
local.file "basic_auth_password" {
  filename  = "/path/to/password"
  is_secret = true
}

// Inside a basic_auth block:
// password = local.file.basic_auth_password.content
```
{{< /admonition >}}
