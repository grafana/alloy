---
aliases:
- ../../configuration-language/standard-library/json_path/
- /docs/grafana-cloud/agent/flow/reference/stdlib/json_path/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/json_path/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/json_path/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/json_path/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/json_path/
description: Learn about json_path
title: json_path
---

# json_path

The `json_path` function lookup values using [jsonpath](https://goessner.net/articles/JsonPath/) syntax.

The function expects two strings. The first string is the JSON string used look up values. The second string is the jsonpath expression.

`json_path` always returns a list of values. If the jsonpath expression does not match any values, an empty list is returned.

A common use case of `json_path` is to decode and filter the output of a [`local.file`][] or [`remote.http`][] component to a River value.

> Remember to escape double quotes when passing JSON string literals to `json_path`.
>
> For example, the JSON value `{"key": "value"}` is properly represented by the
> string `"{\"key\": \"value\"}"`.

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

[`local.file`]: {{< relref "../components/local.file.md" >}}
[`remote.http`]: {{< relref "../components/remote.http.md" >}}
