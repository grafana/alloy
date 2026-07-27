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

`password` and `password_file` are mutually exclusive, and only one can be provided inside a `basic_auth` block.

If you set `password_file`, {{< param "PRODUCT_NAME" >}} reads the file on every outgoing request that uses basic authentication.
{{< param "PRODUCT_NAME" >}} reads the file on each request, so it picks up password rotation automatically.

{{< admonition type="caution" >}}
If you use `password_file`, {{< param "PRODUCT_NAME" >}} reads the file on every outgoing request.
Use the `local.file` component with the `password` attribute instead to avoid unnecessary reads.

High scrape or write rates can trigger repeated file reads.
The `local.file` component watches the file for changes and exports the latest content.
{{< /admonition >}}
