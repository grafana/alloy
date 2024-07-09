---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.span/
aliases:
  - ../otelcol.processor.span/ # /docs/alloy/latest/reference/otelcol.processor.span/
description: Learn about otelcol.processor.span
title: otelcol.processor.span
---

# otelcol.processor.span

`otelcol.processor.span` accepts traces telemetry data from other `otelcol` components and modifies the names and attributes of the spans.
It also supports the ability to filter input data to determine if it should be included or excluded from this processor.

{{< admonition type="note" >}}
`otelcol.processor.span` is a wrapper over the upstream OpenTelemetry Collector `span` processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.processor.span` components by giving them different labels.

## Usage

```alloy
otelcol.processor.span "LABEL" {
  output {
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.span` doesn't support any arguments and is configured fully through inner blocks.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.span`:

Hierarchy            | Block             | Description                                                  | Required
---------------------|-------------------|--------------------------------------------------------------|---------
output               | [output][]        | Configures where to send received telemetry data.            | yes
name                 | [name][]          | Configures how to rename a span and add attributes.          | no
name > to_attributes | [to-attributes][] | Configuration to create attributes from a span name.         | no
status               | [status][]        | Specifies a status which should be set for this span.        | no
include              | [include][]       | Filter for data included in this processor's actions.        | no
include > regexp     | [regexp][]        | Regex cache settings.                                        | no
include > attribute  | [attribute][]     | A list of attributes to match against.                       | no
include > resource   | [resource][]      | A list of items to match the resources against.              | no
include > library    | [library][]       | A list of items to match the implementation library against. | no
exclude              | [exclude][]       | Filter for data excluded from this processor's actions       | no
exclude > regexp     | [regexp][]        | Regex cache settings.                                        | no
exclude > attribute  | [attribute][]     | A list of attributes to match against.                       | no
exclude > resource   | [resource][]      | A list of items to match the resources against.              | no
exclude > library    | [library][]       | A list of items to match the implementation library against. | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. For example, `include > attribute`
refers to an `attribute` block defined inside an `include` block.

If both an `include` block and an `exclude`block are specified, the `include` properties are checked before the `exclude` properties.

[name]: #name-block
[to-attributes]: #to-attributes-block
[status]: #status-block
[output]: #output-block
[include]: #include-block
[exclude]: #exclude-block
[regexp]: #regexp-block
[attribute]: #attribute-block
[resource]: #resource-block
[library]: #library-block
[debug_metrics]: #debug_metrics-block

### name block

The `name` block configures how to rename a span and add attributes.

The following attributes are supported:

Name              | Type           | Description                                                      | Default | Required
------------------|----------------|------------------------------------------------------------------|---------|---------
`from_attributes` | `list(string)` | Attribute keys to pull values from, to generate a new span name. | `[]`    | no
`separator` | `string` | Separates attributes values in the new span name. | `""` | no

Firstly `from_attributes` rules are applied, then [to-attributes][] are applied.
At least one of these 2 fields must be set.

`from_attributes` represents the attribute keys to pull the values from to
generate the new span name:
* All attribute keys are required in the span to rename a span.
If any attribute is missing from the span, no rename will occur.
* The new span name is constructed in order of the `from_attributes`
specified in the configuration.

`separator` is the string used to separate attributes values in the new
span name. If no value is set, no separator is used between attribute
values. `separator` is used with `from_attributes` only;
it is not used with [to-attributes][].

### to_attributes block

The `to_attributes` block configures how to create attributes from a span name.

The following attributes are supported:

Name                | Type           | Description                                                          | Default | Required
--------------------|----------------|----------------------------------------------------------------------|---------|---------
`rules`             | `list(string)` | A list of regex rules to extract attribute values from span name.    |         | yes
`break_after_match` | `bool`         | Configures if processing of rules should stop after the first match. | `false` | no

Each rule in the `rules` list is a regex pattern string.
1. The span name is checked against each regex in the list.
2. If it matches, then all named subexpressions of the regex are extracted as attributes and are added to the span.
3. Each subexpression name becomes an attribute name and the subexpression matched portion becomes the attribute value.
4. The matched portion in the span name is replaced by extracted attribute name.
5. If the attributes already exist in the span then they will be overwritten.
6. The process is repeated for all rules in the order they are specified.
7. Each subsequent rule works on the span name that is the output after processing the previous rule.

`break_after_match` specifies if processing of rules should stop after the first
match. If it is `false`, rule processing will continue to be performed over the
modified span name.

### status block

The `status` block specifies a status which should be set for this span.

The following attributes are supported:

Name          | Type     | Description                                       | Default | Required
--------------|----------|---------------------------------------------------|---------|---------
`code`        | `string` | A status code.                                    |         | yes
`description` | `string` | An optional field documenting Error status codes. | `""`    | no

The supported values for `code` are:
* `Ok`
* `Error`
* `Unset`

`description` should only be specified if `code` is set to `Error`.

### include block

The `include` block provides an option to include data being fed into the
[name][] and [status][] blocks based on the properties of a span.

The following arguments are supported:

Name         | Type           | Description                                          | Default | Required
-------------|----------------|------------------------------------------------------|---------|---------
`match_type` | `string`       | Controls how items to match against are interpreted. |         | yes
`services`   | `list(string)` | A list of items to match the service name against.   | `[]`    | no
`span_names` | `list(string)` | A list of items to match the span name against.      | `[]`    | no
`span_kinds` | `list(string)` | A list of items to match the span kind against.      | `[]`    | no

`match_type` is required and must be set to either `"regexp"` or `"strict"`.

A match occurs if at least one item in the lists matches.

One of `services`, `span_names`, `span_kinds`, [attribute][], [resource][], or [library][] must be specified
with a non-empty value for a valid configuration.

### exclude block

The `exclude` block provides an option to exclude data from being fed into the
[name][] and [status][] blocks based on the properties of a span.

The following arguments are supported:

Name         | Type           | Description                                          | Default | Required
-------------|----------------|------------------------------------------------------|---------|---------
`match_type` | `string`       | Controls how items to match against are interpreted. |         | yes
`services`   | `list(string)` | A list of items to match the service name against.   | `[]`    | no
`span_names` | `list(string)` | A list of items to match the span name against.      | `[]`    | no
`span_kinds` | `list(string)` | A list of items to match the span kind against.      | `[]`    | no

`match_type` is required and must be set to either `"regexp"` or `"strict"`.

A match occurs if at least one item in the lists matches.

One of `services`, `span_names`, `span_kinds`, [attribute][], [resource][], or [library][] must be specified
with a non-empty value for a valid configuration.

### regexp block

{{< docs/shared lookup="reference/components/otelcol-filter-regexp-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### attribute block

{{< docs/shared lookup="reference/components/otelcol-filter-attribute-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### resource block

{{< docs/shared lookup="reference/components/otelcol-filter-resource-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### library block

{{< docs/shared lookup="reference/components/otelcol-filter-library-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block-traces.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` OTLP-formatted data for traces telemetry signals.
Logs and metrics are not supported.

## Component health

`otelcol.processor.attributes` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.attributes` does not expose any component-specific debug
information.

## Examples

### Creating a new span name from attribute values

This example creates a new span name from the values of attributes `db.svc`,
`operation`, and `id`, in that order, separated by the value `::`.
All attribute keys need to be specified in the span for the processor to rename it.

```alloy
otelcol.processor.span "default" {
  name {
    separator        = "::"
    from_attributes  = ["db.svc", "operation", "id"]
  }

  output {
      traces = [otelcol.exporter.otlp.default.input]
  }
}
```

For a span with the following attributes key/value pairs, the above
configuration will change the span name to `"location::get::1234"`:
```json
{
  "db.svc": "location",
  "operation": "get",
  "id": "1234"
}
```

For a span with the following attributes key/value pairs, the above
configuration will not change the span name.
This is because the attribute key `operation` isn't set:
```json
{
  "db.svc": "location",
  "id": "1234"
}
```

### Creating a new span name from attribute values (no separator)

```alloy
otelcol.processor.span "default" {
  name {
    from_attributes = ["db.svc", "operation", "id"]
  }

  output {
      traces = [otelcol.exporter.otlp.default.input]
  }
}
```

For a span with the following attributes key/value pairs, the above
configuration will change the span name to `"locationget1234"`:
```json
{
  "db.svc": "location",
  "operation": "get",
  "id": "1234"
}
```

### Renaming a span name and adding attributes

Example input and output using the configuration below:
1. Let's assume input span name is `/api/v1/document/12345678/update`
2. The span name will be changed to `/api/v1/document/{documentId}/update`
3. A new attribute `"documentId"="12345678"` will be added to the span.

```alloy
otelcol.processor.span "default" {
  name {
    to_attributes {
      rules = ["^\\/api\\/v1\\/document\\/(?P<documentId>.*)\\/update$"]
    }
  }

  output {
      traces = [otelcol.exporter.otlp.default.input]
  }
}
```

### Filtering, renaming a span name and adding attributes

This example renames the span name to `{operation_website}`
and adds the attribute `{Key: operation_website, Value: <old span name> }`
if the span has the following properties:
- Service name contains the word `banks`.
- The span name contains `/` anywhere in the string.
- The span name is not `donot/change`.

```alloy
otelcol.processor.span "default" {
  include {
    match_type = "regexp"
    services   = ["banks"]
    span_names = ["^(.*?)/(.*?)$"]
  }
  exclude {
    match_type = "strict"
    span_names = ["donot/change"]
  }
  name {
    to_attributes {
      rules = ["(?P<operation_website>.*?)$"]
    }
  }

  output {
      traces = [otelcol.exporter.otlp.default.input]
  }
}
```

### Setting a status

This example changes the status of a span to "Error" and sets an error description.

```alloy
otelcol.processor.span "default" {
  status {
    code        = "Error"
    description = "some additional error description"
  }

  output {
      traces = [otelcol.exporter.otlp.default.input]
  }
}
```

### Setting a status depending on an attribute value

This example sets the status to success only when attribute `http.status_code`
is equal to `400`.

```alloy
otelcol.processor.span "default" {
  include {
    match_type = "strict"
    attribute {
      key   = "http.status_code"
      value = 400
    }
  }
  status {
    code = "Ok"
  }

  output {
      traces = [otelcol.exporter.otlp.default.input]
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.span` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.span` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
