---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/yaml_decode/
description: Learn about yaml_decode
title: yaml_decode
---

# yaml_decode

The `yaml_decode` function decodes a string representing YAML into an {{< param "PRODUCT_NAME" >}}
value. `yaml_decode` fails if the string argument provided cannot be parsed as
YAML.

A common use case of `yaml_decode` is to decode the output of a
[`local.file`][] component to an {{< param "PRODUCT_NAME" >}} value.

{{< admonition type="note" >}}
 Remember to escape double quotes when passing YAML string literals to `yaml_decode`.

For example, the YAML value `key: "value"` is properly represented by the string `"key: \"value\""`.
{{< /admonition >}}

## Examples

```
> yaml_decode("15")
15
> yaml_decode("[1, 2, 3]")
[1, 2, 3]
> yaml_decode("null")
null
> yaml_decode("key: value")
{
  key = "value",
}
> yaml_decode(local.file.some_file.content)
"Hello, world!"
```

[`local.file`]: ../../components/local/local.file/