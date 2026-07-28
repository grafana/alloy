---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/basic-auth-block/
description: Shared content, basic auth block
headless: true
---

| Name            | Type     | Description                                           | Default | Required |
| --------------- | -------- | ----------------------------------------------------- | ------- | -------- |
| `password_file` | `string` | Path to a file that contains the basic auth password. |         | no       |
| `password`      | `secret` | Basic auth password.                                  |         | no       |
| `username`      | `string` | Basic auth username.                                  |         | no       |

When you use `password_file`, the file is read on every outgoing request that uses basic authentication.

`password` and `password_file` are mutually exclusive, and only one can be provided inside a `basic_auth` block.

{{< admonition type="note" >}}
High scrape or write rates create repeated file reads when you use `password_file`.
You can use the [`local.file`][local.file] component to read the password file and provide the content to the `password` attribute.
This avoids repeated file reads because `local.file` monitors the file and reads when it changes.

[local.file]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/local/local.file/
{{< /admonition >}}
