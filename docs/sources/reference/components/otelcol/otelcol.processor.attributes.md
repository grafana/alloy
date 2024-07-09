---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.attributes/
aliases:
  - ../otelcol.processor.attributes/ # /docs/alloy/latest/reference/otelcol.processor.attributes/
description: Learn about otelcol.processor.attributes
title: otelcol.processor.attributes
---

# otelcol.processor.attributes

`otelcol.processor.attributes` accepts telemetry data from other `otelcol`
components and modifies attributes of a span, log, or metric.
It also supports the ability to filter and match input data to determine if it should be included or excluded for attribute modifications.

{{< admonition type="note" >}}
`otelcol.processor.attributes` is a wrapper over the upstream OpenTelemetry Collector `attributes` processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.processor.attributes` components by giving them different labels.

## Usage

```alloy
otelcol.processor.attributes "LABEL" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.attributes` doesn't support any arguments and is configured fully through inner blocks.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.attributes`:

Hierarchy              | Block            | Description                                                        | Required
-----------------------|------------------|--------------------------------------------------------------------|---------
output                 | [output][]       | Configures where to send received telemetry data.                  | yes
action                 | [action][]       | Actions to take on the attributes of incoming metrics/logs/traces. | no
include                | [include][]      | Filter for data included in this processor's actions.              | no
include > regexp       | [regexp][]       | Regex cache settings.                                              | no
include > attribute    | [attribute][]    | A list of attributes to match against.                             | no
include > resource     | [resource][]     | A list of items to match the resources against.                    | no
include > library      | [library][]      | A list of items to match the implementation library against.       | no
include > log_severity | [library][]      | How to match against a log record's SeverityNumber, if defined.    | no
exclude                | [exclude][]      | Filter for data excluded from this processor's actions             | no
exclude > regexp       | [regexp][]       | Regex cache settings.                                              | no
exclude > attribute    | [attribute][]    | A list of attributes to match against.                             | no
exclude > resource     | [resource][]     | A list of items to match the resources against.                    | no
exclude > library      | [library][]      | A list of items to match the implementation library against.       | no
exclude > log_severity | [log_severity][] | How to match against a log record's SeverityNumber, if defined.    | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. For example, `include > attribute`
refers to an `attribute` block defined inside an `include` block.

If both an `include` block and an `exclude`block are specified, the `include` properties are checked before the `exclude` properties.

[output]: #output-block
[action]: #action-block
[include]: #include-block
[exclude]: #exclude-block
[regexp]: #regexp-block
[attribute]: #attribute-block
[resource]: #resource-block
[library]: #library-block
[log_severity]: #log_severity-block
[debug_metrics]: #debug_metrics-block

### action block

The `action` block configures how to modify the span, log, or metric.

The following attributes are supported:

Name             | Type     | Description                                                             | Default | Required
-----------------|----------|-------------------------------------------------------------------------|---------|---------
`key`            | `string` | The attribute that the action relates to.                               |         | yes
`action`         | `string` | The type of action performed.                                           |         | yes
`value`          | `any`    | The value to populate for the key.                                      |         | no
`pattern`        | `string` | A regex pattern.                                                        | `""`    | no
`from_attribute` | `string` | The attribute from the input data used to populate the attribute value. | `""`    | no
`from_context`   | `string` | The context value used to populate the attribute value.                 | `""`    | no
`converted_type` | `string` | The type to convert the attribute value to.                             | `""`    | no

The `value` data type must be either a number, string, or boolean.

The supported values for `action` are:

* `insert`: Inserts a new attribute in input data where the key does not already exist.

    * The `key` attribute is required. It specifies the attribute to act upon.
    * One of the `value`, `from_attribute` or `from_context` attributes is required.

* `update`: Updates an attribute in input data where the key does exist.

    * The `key`attribute is required. It specifies the attribute to act upon.
    * One of the `value`, `from_attribute` or `from_context` attributes is required.

* `upsert`: Either inserts a new attribute in input data where the key does not already exist
   or updates an attribute in input data where the key does exist.

    * The `key`attribute is required. It specifies the attribute to act upon.
    * One of the `value`, `from_attribute` or `from_context`attributes is required:
        * `value` specifies the value to populate for the key.
        * `from_attribute` specifies the attribute from the input data to use to populate
        the value. If the attribute doesn't exist, no action is performed.
        * `from_context` specifies the context value used to populate the attribute value.
        If the key is prefixed with `metadata.`, the values are searched
        in the receiver's transport protocol for additional information like gRPC Metadata or HTTP Headers.
        If the key is prefixed with `auth.`, the values are searched
        in the authentication information set by the server authenticator.
        Refer to the server authenticator's documentation part of your pipeline
        for more information about which attributes are available.
        If the key doesn't exist, no action is performed.
        If the key has multiple values the values will be joined with a `;` separator.

* `hash`: Hashes (SHA1) an existing attribute value.

    * The `key` attribute and/or the `pattern` attributes is required.

* `extract`: Extracts values using a regular expression rule from the input key to target keys specified in the rule.
  If a target key already exists, it will be overridden. Note: It behaves similarly to the Span Processor `to_attributes`
  setting with the existing attribute as the source.

    * The `key` attribute is required. It specifies the attribute to extract values from. The value of `key` is NOT altered.
    * The `pattern` attribute is required. It is the regex pattern used to extract attributes from the value of `key`.
      The submatchers must be named. If attributes already exist, they will be overwritten.

* `convert`: Converts an existing attribute to a specified type.

    * The `key` attribute is required. It specifies the attribute to act upon.
    * The `converted_type` attribute is required and must be one of int, double or string.

* `delete`: Deletes an attribute from the input data.

    * The `key` attribute and/or the `pattern` attribute is required. It specifies the attribute to act upon.

### include block

The `include` block provides an option to include data being fed into the [action] blocks based on the properties of a span, log, or metric records.

{{< docs/shared lookup="reference/components/match-properties-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

One of the following is also required:
* For spans, one of `services`, `span_names`, `span_kinds`, [attribute][], [resource][], or [library][] must be specified
  with a non-empty value for a valid configuration. The `log_bodies`, `log_severity_texts`, `log_severity`, and `metric_names` attributes are invalid.
* For logs, one of `log_bodies`, `log_severity_texts`, `log_severity`, [attribute][], [resource][], or [library][] must be
  specified with a non-empty value for a valid configuration. The `span_names`, `span_kinds`, `metric_names`, and `services` attributes are invalid.
* For metrics, `metric_names` must be specified with a valid non-empty value for a valid configuration. The `span_names`,
  `span_kinds`, `log_bodies`, `log_severity_texts`, `log_severity`, `services`, [attribute][], [resource][], and [library][] attributes are invalid.

If the configuration includes filters which are specific to a particular signal type, it is best to include only that signal type in the component's output.
For example, adding a `span_names` filter could cause the component to error if logs are configured in the component's outputs.

### exclude block

The `exclude` block provides an option to exclude data from being fed into the [action] blocks based on the properties of a span, log, or metric records.

{{< admonition type="note" >}}
Signals excluded by the `exclude` block will still be propagated to downstream components as-is.
If you would like to not propagate certain signals to downstream components,
consider a processor such as [otelcol.processor.tail_sampling][].

[otelcol.processor.tail_sampling]: ../otelcol.processor.tail_sampling/
{{< /admonition >}}

{{< docs/shared lookup="reference/components/match-properties-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

One of the following is also required:
* For spans, one of `services`, `span_names`, `span_kinds`, [attribute][], [resource][], or [library][] must be specified 
  with a non-empty value for a valid configuration. The `log_bodies`, `log_severity_texts`, `log_severity`, and `metric_names` attributes are invalid.
* For logs, one of `log_bodies`, `log_severity_texts`, `log_severity`, [attribute][], [resource][], or [library][] must be 
  specified with a non-empty value for a valid configuration. The `span_names`, `span_kinds`, `metric_names`, and `services` attributes are invalid.
* For metrics, `metric_names` must be specified with a valid non-empty value for a valid configuration. The `span_names`, 
  `span_kinds`, `log_bodies`, `log_severity_texts`, `log_severity`, `services`, [attribute][], [resource][], and [library][] attributes are invalid.

If the configuration includes filters which are specific to a particular signal type, it is best to include only that signal type in the component's output.
For example, adding a `span_names` filter could cause the component to error if logs are configured in the component's outputs.

### regexp block

{{< docs/shared lookup="reference/components/otelcol-filter-regexp-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### attribute block

{{< docs/shared lookup="reference/components/otelcol-filter-attribute-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### resource block

{{< docs/shared lookup="reference/components/otelcol-filter-resource-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### library block

{{< docs/shared lookup="reference/components/otelcol-filter-library-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### log_severity block

{{< docs/shared lookup="reference/components/otelcol-filter-log-severity-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.processor.attributes` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.attributes` does not expose any component-specific debug
information.

## Examples

### Various uses of the "action" block

```alloy
otelcol.receiver.otlp "default" {
  http {}
  grpc {}

  output {
    metrics = [otelcol.processor.attributes.default.input]
    logs    = [otelcol.processor.attributes.default.input]
    traces  = [otelcol.processor.attributes.default.input]
  }
}

otelcol.processor.attributes "default" {
    // Inserts a new attribute "attribute1" to spans where
    // the key "attribute1" doesn't exist.
    // The type of `attribute1` is inferred by the configuration.
    // `123` is an integer and is stored as an integer in the attributes.
    action {
        key = "attribute1"
        value = 123
        action = "insert"
    }

    // Inserts a new attribute with a key of "string key" and
    // a string value of "anotherkey".
    action {
        key = "string key"
        value = "anotherkey"
        action = "insert"
    }

    // Setting an attribute on all spans.
    // Any spans that already had `region` now have value `planet-earth`.
    // This can be done to set properties for all traces without
    // requiring an instrumentation change.
    action {
        key = "region"
        value = "planet-earth"
        action = "upsert"
    }

    // The following demonstrates copying a value to a new key.
    // If a span doesn't contain `user_key`, no new attribute `new_user_key` is created.
    action {
        key = "new_user_key"
        from_attribute = "user_key"
        action = "upsert"
    }

    // Hashing existing attribute values.
    action {
        key = "user.email"
        action = "hash"
    }

    // Uses the value from key `example_user_key` to upsert attributes
    // to the target keys specified in the `pattern`.
    // (Insert attributes for target keys that do not exist and update keys that exist.)
    // Given example_user_key = /api/v1/document/12345678/update/v1
    // then the following attributes will be inserted:
    // new_example_user_key: 12345678
    // version: v1
    //
    // Note: Similar to the Span Processor, if a target key already exists,
    // it will be updated.
    //
    // Note: The regex pattern is enclosed in backticks instead of quotation marks.
    // This constitutes a raw {{< param "PRODUCT_NAME" >}} syntax string, and lets us avoid the need to escape backslash characters.
    action {
        key = "example_user_key"
        pattern = `\/api\/v1\/document\/(?P<new_user_key>.*)\/update\/(?P<version>.*)$`
        action = "extract"
    }

    // Converting the type of an existing attribute value.
    action {
        key = "http.status_code"
        converted_type = "int"
        action = "convert"
    }

    // Deleting keys from an attribute.
    action {
        key = "credit_card"
        action = "delete"
    }

    output {
        metrics = [otelcol.exporter.otlp.default.input]
        logs    = [otelcol.exporter.otlp.default.input]
        traces  = [otelcol.exporter.otlp.default.input]
    }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("OTLP_ENDPOINT")
  }
}
```

### Excluding spans based on attributes

For example, the following spans match the properties and won't be processed by the processor:
* Span1 Name: "svcB", Attributes: {env: "dev", test_request: 123, credit_card: 1234}
* Span2 Name: "svcA", Attributes: {env: "dev", test_request: false}

The following spans do not match the properties and the processor actions are applied to it:
* Span3 Name: "svcB", Attributes: {env: 1, test_request: "dev", credit_card: 1234}
* Span4 Name: "svcC", Attributes: {env: "dev", test_request: false}

Note that due to the presence of the `services` attribute, this configuration works only for
trace signals. This is why only traces are configured in the `output` block.

```alloy
otelcol.processor.attributes "default" {
    exclude {
        match_type = "strict"
        services = ["svcA", "svcB"]
        attribute {
            key = "env"
            value = "dev"
        }
        attribute {
            key = "test_request"
        }
    }
    action {
        key = "credit_card"
        action = "delete"
    }
    action {
        key = "duplicate_key"
        action = "delete"
    }
    output {
        traces  = [otelcol.exporter.otlp.default.input]
    }
}
```

### Excluding spans based on resources

A "strict" `match_type` means that we must strictly match the `resource` key/value pairs.

Note that the `resource` attribute is not used for metrics, which is why metrics are not configured
in the component output.

```alloy
otelcol.processor.attributes "default" {
    exclude {
        match_type = "strict"
        resource {
            key = "host.type"
            value = "n1-standard-1"
        }
    }
    action {
        key = "credit_card"
        action = "delete"
    }
    action {
        key = "duplicate_key"
        action = "delete"
    }
    output {
        logs    = [otelcol.exporter.otlp.default.input]
        traces  = [otelcol.exporter.otlp.default.input]
    }
}
```

### Excluding spans based on a specific library version

A "strict" `match_type` means that we must strictly match the `library` key/value pairs.

Note that the `library` attribute is not used for metrics, which is why metrics are not configured
in the component output.

```alloy
otelcol.processor.attributes "default" {
    exclude {
        match_type = "strict"
        library {
            name = "mongo-java-driver"
            version = "3.8.0"
        }
    }
    action {
        key = "credit_card"
        action = "delete"
    }
    action {
        key = "duplicate_key"
        action = "delete"
    }
    output {
        logs    = [otelcol.exporter.otlp.default.input]
        traces  = [otelcol.exporter.otlp.default.input]
    }
}
```

### Including and excluding spans based on regex and services

This processor will remove the "token" attribute and will obfuscate the "password" attribute 
in spans where the service name matches `"auth.*"` and where the span name does not match `"login.*"`.

Note that due to the presence of the `services` and `span_names` attributes, this configuration 
works only for trace signals. This is why only traces are configured in the `output` block.

```alloy
otelcol.processor.attributes "default" {
    // Specifies the span properties that must exist for the processor to be applied.
    include {
        // "match_type" defines that "services" is an array of regexp-es.
        match_type = "regexp"
        // The span service name must match "auth.*" pattern.
        services = ["auth.*"]
    }

    exclude {
        // "match_type" defines that "span_names" is an array of regexp-es.
        match_type = "regexp"
        // The span name must not match "login.*" pattern.
        span_names = ["login.*"]
    }

    action {
        key = "password"
        action = "update"
        value = "obfuscated"
    }

    action {
        key = "token"
        action = "delete"
    }

    output {
        traces  = [otelcol.exporter.otlp.default.input]
    }
}
```

### Including spans based on regex and attributes

The following demonstrates how to process spans with attributes that match a regexp pattern.
This processor will obfuscate the "db.statement" attribute in spans where the "db.statement" attribute
matches a regex pattern.

```alloy
otelcol.processor.attributes "default" {
    include {
        // "match_type" of "regexp" defines that the "value" attributes 
        // in the "attribute" blocks are regexp-es.
        match_type = "regexp"

        // This attribute ('db.statement') must exist in the span and match 
        // the regex ('SELECT \* FROM USERS.*') for a match.
        attribute {
            key = "db.statement"
            value = "SELECT \* FROM USERS.*"
        }
    }

    action {
        key = "db.statement"
        action = "update"
        value = "SELECT * FROM USERS [obfuscated]"
    }

    output {
        metrics = [otelcol.exporter.otlp.default.input]
        logs    = [otelcol.exporter.otlp.default.input]
        traces  = [otelcol.exporter.otlp.default.input]
    }
}
```

### Including spans based on regex of log body

This processor will remove the "token" attribute and will obfuscate the "password"
attribute in spans where the log body matches "AUTH.*".

Note that due to the presence of the `log_bodies` attribute, this configuration works only for
log signals. This is why only logs are configured in the `output` block.

```alloy
otelcol.processor.attributes "default" {
    include {
        match_type = "regexp"
        log_bodies = ["AUTH.*"]
    }
    action {
        key = "password"
        action = "update"
        value = "obfuscated"
    }
    action {
        key = "token"
        action = "delete"
    }

    output {
        logs    = [otelcol.exporter.otlp.default.input]
    }
}
```

### Including spans based on regex of log severity

The following demonstrates how to process logs that have a severity level which is equal to or higher than
the level specified in the `log_severity` block. This processor will remove the "token" attribute and will 
obfuscate the "password" attribute in logs where the severity is at least "INFO".

Note that due to the presence of the `log_severity` attribute, this configuration works only for
log signals. This is why only logs are configured in the `output` block.

```alloy
otelcol.processor.attributes "default" {
    include {
        match_type = "regexp"
		log_severity {
			min = "INFO"
			match_undefined = true
		}
    }
    action {
        key = "password"
        action = "update"
        value = "obfuscated"
    }
    action {
        key = "token"
        action = "delete"
    }

    output {
        logs    = [otelcol.exporter.otlp.default.input]
    }
}
```

### Including spans based on regex of log severity text

The following demonstrates how to process logs that have a severity text that match regexp
patterns. This processor will remove the "token" attribute and will obfuscate the "password"
attribute in logs where severity matches "info".

Note that due to the presence of the `log_severity_texts` attribute, this configuration works only for
log signals. This is why only logs are configured in the `output` block.

```alloy
otelcol.processor.attributes "default" {
    include {
        match_type = "regexp"
        log_severity_texts = ["info.*"]
    }
    action {
        key = "password"
        action = "update"
        value = "obfuscated"
    }
    action {
        key = "token"
        action = "delete"
    }

    output {
        logs    = [otelcol.exporter.otlp.default.input]
    }
}
```

### Including metrics based on metric names

The following demonstrates how to process metrics that have a name starting with "counter".
This processor will add a label called "important_label" with a value of "label_val" to the metric.
If the label already exists, its value will be updated.

Note that due to the presence of the `metric_names` attribute, this configuration works only for
metric signals. This is why only metrics are configured in the `output` block.

```alloy
otelcol.processor.attributes "default" {
	include {
		match_type = "regexp"
		metric_names = ["counter.*"]
	}
	action {
		key = "important_label"
		action = "upsert"
		value = "label_val"
	}

    output {
        metrics = [otelcol.exporter.otlp.default.input]
    }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.attributes` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.attributes` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->