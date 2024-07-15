---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/json_decode/
description: Learn about json_decode
title: json_decode
---

# json_decode

The `json_decode` function decodes a string representing JSON into an {{< param "PRODUCT_NAME" >}} value.
`json_decode` fails if the string argument provided can't be parsed as JSON.

A common use case of `json_decode` is to decode the output of a [`local.file`][] component to an {{< param "PRODUCT_NAME" >}} value.

> Remember to escape double quotes when passing JSON string literals to `json_decode`.
>
> For example, the JSON value `{"key": "value"}` is properly represented by the string `"{\"key\": \"value\"}"`.

## Examples

```
> json_decode("15")
15

> json_decode("[1, 2, 3]")
[1, 2, 3]

> json_decode("null")
null

> json_decode("{\"key\": \"value\"}")
{
  key = "value",
}

> json_decode(local.file.some_file.content)
"Hello, world!"
```

[`local.file`]: ../../components/local/local.file/
