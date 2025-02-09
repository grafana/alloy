---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/encoding/
description: Learn about encoding functions
aliases:
  - ./base64_decode/ # /docs/alloy/latest/reference/stdlib/base64_decode/
  - ./json_decode/ # /docs/alloy/latest/reference/stdlib/json_decode/
  - ./yaml_decode/ # /docs/alloy/latest/reference/stdlib/yaml_decode/
menuTitle: encoding
title: encoding
---

# encoding

The `encoding` namespace contains encoding and decoding functions.

## encoding.from_base64

The `encoding.from_base64` function decodes a RFC4648-compliant Base64-encoded string into the original string.

`encoding.from_base64` fails if the provided string argument contains invalid Base64 data.

### Example

```text
> encoding.from_base64("dGFuZ2VyaW5l")
tangerine
```

## encoding.from_URLbase64

The `encoding.from_URLbase64` function decodes a RFC4648-compliant Base64 URL safe encoded string into the original string.

`encoding.from_URLbase64` fails if the provided string argument contains invalid Base64 data.

### Example

```
> encoding.from_URLbase64("c3RyaW5nMTIzIT8kKiYoKSctPUB-")
string123!?$*&()'-=@~
```

## encoding.to_base64

The `encoding.to_base64` function encodes the original string into RFC4648-compliant Base64 encoded string.

### Example

```
> encoding.to_base64("string123!?$*&()'-=@~")
c3RyaW5nMTIzIT8kKiYoKSctPUB+
```

## encoding.to_URLbase64

The `encoding.to_base64` function encodes the original string into RFC4648-compliant URL safe Base64 encoded string.

### Example

```
> encoding.to_URLbase64("string123!?$*&()'-=@~")
c3RyaW5nMTIzIT8kKiYoKSctPUB-
```

## encoding.from_json

The `encoding.from_json` function decodes a string representing JSON into an {{< param "PRODUCT_NAME" >}} value.
`encoding.from_json` fails if the string argument provided can't be parsed as JSON.

A common use case of `encoding.from_json` is to decode the output of a [`local.file`][] component to an {{< param "PRODUCT_NAME" >}} value.

{{< admonition type="note" >}}
Remember to escape double quotes when passing JSON string literals to `encoding.from_json`.

For example, the JSON value `{"key": "value"}` is properly represented by the string `"{\"key\": \"value\"}"`.
{{< /admonition >}}

### Examples

```alloy
> encoding.from_json("15")
15

> encoding.from_json("[1, 2, 3]")
[1, 2, 3]

> encoding.from_json("null")
null

> encoding.from_json("{\"key\": \"value\"}")
{
  key = "value",
}

> encoding.from_json(local.file.some_file.content)
"Hello, world!"
```

## encoding.from_yaml

The `encoding.from_yaml` function decodes a string representing YAML into an {{< param "PRODUCT_NAME" >}} value.
`encoding.from_yaml` fails if the string argument provided can't be parsed as YAML.

A common use case of `encoding.from_yaml` is to decode the output of a [`local.file`][] component to an {{< param "PRODUCT_NAME" >}} value.

{{< admonition type="note" >}}
 Remember to escape double quotes when passing YAML string literals to `encoding.from_yaml`.

For example, the YAML value `key: "value"` is properly represented by the string `"key: \"value\""`.
{{< /admonition >}}

### Examples

```alloy
> encoding.from_yaml("15")
15
> encoding.from_yaml("[1, 2, 3]")
[1, 2, 3]
> encoding.from_yaml("null")
null
> encoding.from_yaml("key: value")
{
  key = "value",
}
> encoding.from_yaml(local.file.some_file.content)
"Hello, world!"
```

[`local.file`]: ../../components/local/local.file/
