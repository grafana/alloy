---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/livedebugging/
description: Learn about the livedebugging configuration block
labels:
  stage: general-availability
  products:
    - oss
title: livedebugging
---

# `livedebugging`

`livedebugging` is an optional configuration block that enables the [live debugging feature][debug], that streams real-time data from your components directly to the {{< param "PRODUCT_NAME" >}} UI.

By default, [live debugging][debug] is disabled and must be explicitly enabled through this configuration block to make the debugging data visible in the {{< param "PRODUCT_NAME" >}} UI.

{{< admonition type="note" >}}
The live debugging feature uses the {{< param "PRODUCT_NAME" >}} UI to provide detailed insights into the data flowing through your pipelines.
To ensure that your data remains secure while live debugging is enabled, configure TLS in the [http block][].

[http block]: ../http/
{{< /admonition >}}

## Usage

```alloy
livedebugging {

}
```

## Arguments

You can use the following arguments with `livedebugging`:

| Name      | Type   | Description                         | Default | Required |
| --------- | ------ | ----------------------------------- | ------- | -------- |
| `enabled` | `bool` | Enables the live debugging feature. | `false` | no       |

## Example

The following example enables `livedebugging`:

```alloy
livedebugging {
  enabled = true
}
```

[debug]: ../../../troubleshoot/debug/
