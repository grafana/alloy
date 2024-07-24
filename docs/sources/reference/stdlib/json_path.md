---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/json_path/
description: Learn about json_path
title: json_path
---

# json_path

The `json_path` function lookup values using [jsonpath][] syntax.

The function expects two strings. The first string is the JSON string used look up values. The second string is the JSONPath expression.

`json_path` always returns a list of values. If the JSONPath expression doesn't match any values, an empty list is returned.

A common use case of `json_path` is to decode and filter the output of a [`local.file`][] or [`remote.http`][] component to an {{< param "PRODUCT_NAME" >}} syntax value.

> Remember to escape double quotes when passing JSON string literals to `json_path`.
>
> For example, the JSON value `{"key": "value"}` is properly represented by the string `"{\"key\": \"value\"}"`.

## Examples

```
> json_path("{\"key\": \"value\"}", ".key")
["value"]


> json_path("[{\"name\": \"Department\",\"value\": \"IT\"},{\"name\":\"TestStatus\",\"value\":\"Pending\"}]", "[?(@.name == \"Department\")].value")
["IT"]

> json_path("{\"key\": \"value\"}", ".nonexists")
[]

> json_path("{\"key\": \"value\"}", ".key")[0]
value

```

[jsonpath]: https://goessner.net/articles/JsonPath/
[`local.file`]: ../../components/local/local.file/
[`remote.http`]: ../../components/remote/remote.http/
