---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.tail_sampling/
aliases:
  - ../otelcol.processor.tail_sampling/ # /docs/alloy/latest/reference/otelcol.processor.tail_sampling/
description: Learn about otelcol.processor.tail_sampling
title: otelcol.processor.tail_sampling
---

# otelcol.processor.tail_sampling

`otelcol.processor.tail_sampling` samples traces based on a set of defined
policies. All spans for a given trace *must* be received by the same collector
instance for effective sampling decisions.

The `tail_sampling` component uses both soft and hard limits, where the hard limit
is always equal or larger than the soft limit. When memory usage goes above the
soft limit, the processor component drops data and returns errors to the
preceding components in the pipeline. When usage exceeds the hard
limit, the processor forces a garbage collection in order to try and free
memory. When usage is below the soft limit, no data is dropped and no forced
garbage collection is performed.

> **Note**: `otelcol.processor.tail_sampling` is a wrapper over the upstream
> OpenTelemetry Collector Contrib `tail_sampling` processor. Bug reports or feature
> requests will be redirected to the upstream repository, if necessary.

Multiple `otelcol.processor.tail_sampling` components can be specified by
giving them different labels.

## Usage

```alloy
otelcol.processor.tail_sampling "LABEL" {
  policy {
    ...
  }
  ...

  output {
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.tail_sampling` supports the following arguments:

Name                          | Type       | Description                                                                  | Default | Required
------------------------------|------------|------------------------------------------------------------------------------|---------|---------
`decision_wait`               | `duration` | Wait time since the first span of a trace before making a sampling decision. | `"30s"` | no
`num_traces`                  | `int`      | Number of traces kept in memory.                                             | `50000` | no
`expected_new_traces_per_sec` | `int`      | Expected number of new traces (helps in allocating data structures).         | `0`     | no

`decision_wait` determines the number of batches to maintain on a channel. Its value must convert to a number of seconds greater than zero.

`num_traces` determines the buffer size of the trace delete channel which is composed of trace ids. Increasing the number will increase the memory usage of the component while decreasing the number will lower the maximum amount of traces kept in memory.

`expected_new_traces_per_sec` determines the initial slice sizing of the current batch. A larger number will use more memory but be more efficient when adding traces to the batch.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.tail_sampling`:

Hierarchy | Block | Description  | Required
--------- | ----- | -----------  | --------
decision_cache                                                | [decision_cache] [] | Configures amount of trace IDs to be kept in an LRU cache. | no
policy                                                        | [policy] [] | Policies used to make a sampling decision. | yes
policy > latency                                              | [latency] | The policy will sample based on the duration of the trace. | no
policy > numeric_attribute                                    | [numeric_attribute] | The policy will sample based on number attributes (resource and record). | no
policy > probabilistic                                        | [probabilistic] | The policy will sample a percentage of traces. | no
policy > status_code                                          | [status_code] | The policy will sample based upon the status code. | no
policy > string_attribute                                     | [string_attribute] | The policy will sample based on string attributes (resource and record) value matches. | no
policy > rate_limiting                                        | [rate_limiting] | The policy will sample based on rate. | no
policy > span_count                                           | [span_count] | The policy will sample based on the minimum number of spans within a batch. | no
policy > boolean_attribute                                    | [boolean_attribute] | The policy will sample based on a boolean attribute (resource and record). | no
policy > ottl_condition                                       | [ottl_condition] | The policy will sample based on a given boolean OTTL condition (span and span event).| no
policy > trace_state                                          | [trace_state] | The policy will sample based on TraceState value matches. | no
policy > and                                                  | [and] | The policy will sample based on multiple policies, creates an `and` policy. | no
policy > and > and_sub_policy                                 | [and_sub_policy] [] | A set of policies underneath an `and` policy type. | no
policy > and > and_sub_policy > latency                       | [latency] | The policy will sample based on the duration of the trace. | no
policy > and > and_sub_policy > numeric_attribute             | [numeric_attribute] | The policy will sample based on number attributes (resource and record). | no
policy > and > and_sub_policy > probabilistic                 | [probabilistic] | The policy will sample a percentage of traces. | no
policy > and > and_sub_policy > status_code                   | [status_code] | The policy will sample based upon the status code. | no
policy > and > and_sub_policy > string_attribute              | [string_attribute] | The policy will sample based on string attributes (resource and record) value matches. | no
policy > and > and_sub_policy > rate_limiting                 | [rate_limiting] | The policy will sample based on rate. | no
policy > and > and_sub_policy > span_count                    | [span_count] | The policy will sample based on the minimum number of spans within a batch. | no
policy > and > and_sub_policy > boolean_attribute             | [boolean_attribute] | The policy will sample based on a boolean attribute (resource and record). | no
policy > and > and_sub_policy > ottl_condition                | [ottl_condition] | The policy will sample based on a given boolean OTTL condition (span and span event). | no
policy > and > and_sub_policy > trace_state                   | [trace_state] | The policy will sample based on TraceState value matches. | no
policy > composite                                            | [composite] | This policy will sample based on a combination of above samplers, with ordering and rate allocation per sampler. | no
policy > composite > composite_sub_policy                     | [composite_sub_policy] [] | A set of policies underneath a `composite` policy type. | no
policy > composite > composite_sub_policy > latency           | [latency] | The policy will sample based on the duration of the trace. | no
policy > composite > composite_sub_policy > numeric_attribute | [numeric_attribute] | The policy will sample based on number attributes (resource and record). | no
policy > composite > composite_sub_policy > probabilistic     | [probabilistic] | The policy will sample a percentage of traces. | no
policy > composite > composite_sub_policy > status_code       | [status_code] | The policy will sample based upon the status code. | no
policy > composite > composite_sub_policy > string_attribute  | [string_attribute] | The policy will sample based on string attributes (resource and record) value matches. | no
policy > composite > composite_sub_policy > rate_limiting     | [rate_limiting] | The policy will sample based on rate. | no
policy > composite > composite_sub_policy > span_count        | [span_count] | The policy will sample based on the minimum number of spans within a batch. | no
policy > composite > composite_sub_policy > boolean_attribute | [boolean_attribute] | The policy will sample based on a boolean attribute (resource and record). | no
policy > composite > composite_sub_policy > ottl_condition    | [ottl_condition] | The policy will sample based on a given boolean OTTL condition (span and span event). | no
policy > composite > composite_sub_policy > trace_state       | [trace_state] | The policy will sample based on TraceState value matches. | no
output                                                        | [output] [] | Configures where to send received telemetry data. | yes
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[decision_cache]: #decision_cache-block
[policy]: #policy-block
[latency]: #latency-block
[numeric_attribute]: #numeric_attribute-block
[probabilistic]: #probabilistic-block
[status_code]: #status_code-block
[string_attribute]: #string_attribute-block
[rate_limiting]: #rate_limiting-block
[span_count]: #span_count-block
[boolean_attribute]: #boolean_attribute-block
[ottl_condition]: #ottl_condition-block
[trace_state]: #trace_state-block
[and]: #and-block
[and_sub_policy]: #and_sub_policy-block
[composite]: #composite-block
[composite_sub_policy]: #composite_sub_policy-block
[output]: #output-block
[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/
[debug_metrics]: #debug_metrics-block

### decision_cache block

The `decision_cache` block configures the number of trace IDs to be kept in an LRU cache, 
persisting the "keep" decisions for traces that may have already been released from memory. 

The following arguments are supported:

Name                 | Type     | Description                                 | Default | Required
---------------------|----------|---------------------------------------------|---------|---------
`sampled_cache_size` | `number` | Amount of trace IDs to keep in the cache.   | `0`     | no

When `sampled_cache_size` is set to `0`, the cache is inactive. 

When using `decision_cache`, configure `sampled_cache_size` much higher than `num_traces` so that decisions for trace IDs are kept longer than the span data for the trace.

### policy block

The `policy` block configures a sampling policy used by the component. At least one `policy` block is required.

The following arguments are supported:

Name   | Type     | Description                            | Default | Required
-------|----------|----------------------------------------|---------|---------
`name` | `string` | The custom name given to the policy.   |         | yes
`type` | `string` | The valid policy type for this policy. |         | yes

Each policy results in a decision, and the processor evaluates them to make a final decision:

- When there's an "inverted not sample" decision, the trace is not sampled.
- When there's a "sample" decision, the trace is sampled.
- When there's an "inverted sample" decision and no "not sample" decisions, the trace is sampled.
- In all other cases, the trace is *not* sampled.

An "inverted" decision is the one made based on the "invert_match" attribute, such as the one from the string tag policy.

### latency block

The `latency` block configures a policy of type `latency`. The policy samples based on the duration of the trace. The duration is determined by looking at the earliest start time and latest end time, without taking into consideration what happened in between.

The following arguments are supported:

Name                 | Type     | Description                                            | Default | Required
---------------------|----------|--------------------------------------------------------|---------|---------
`threshold_ms`       | `number` | Lower latency threshold for sampling, in milliseconds. |         | yes
`upper_threshold_ms` | `number` | Upper latency threshold for sampling, in milliseconds. | `0`     | no

For a trace to be sampled, its latency should be greater than `threshold_ms` and lower than or equal to `upper_threshold_ms`.

An `upper_threshold_ms` of `0` will result in a policy which samples anything greater than `threshold_ms`.

### numeric_attribute block

The `numeric_attribute` block configures a policy of type `numeric_attribute`. The policy samples based on number attributes (resource and record).

The following arguments are supported:

Name           | Type     | Description                                                    | Default | Required
---------------|----------|----------------------------------------------------------------|---------|---------
`key`          | `string` | Tag that the filter is matched against.                        |         | yes
`min_value`    | `number` | The minimum value of the attribute to be considered a match.   |         | yes
`max_value`    | `number` | The maximum value of the attribute to be considered a match.   |         | yes
`invert_match` | `bool`   | Indicates that values must not match against attribute values. | `false` | no

### probabilistic block

The `probabilistic` block configures a policy of type `probabilistic`. The policy samples a percentage of traces.

The following arguments are supported:

Name                  | Type     | Description                                      | Default | Required
----------------------|----------|--------------------------------------------------|---------|---------
`sampling_percentage` | `number` | The percentage rate at which traces are sampled. |         | yes
`hash_salt`           | `string` | See below.                                       |         | no

Use `hash_salt` to configure the hashing salts. This is important in scenarios where multiple layers of collectors
have different sampling rates. If multiple collectors use the same salt with different sampling rates, passing one
layer may pass the other even if the collectors have different sampling rates. Configuring different salts avoids that.

### status_code block

The `status_code` block configures a policy of type `status_code`. The policy samples based upon the status code.

The following arguments are supported:

Name           | Type           | Description                                                                               | Default | Required
---------------|----------------|-------------------------------------------------------------------------------------------|---------|---------
`status_codes` | `list(string)` | Holds the configurable settings to create a status code filter sampling policy evaluator. |         | yes

`status_codes` values must be "OK", "ERROR" or "UNSET".

### string_attribute block

The `string_attribute` block configures a policy of type `string_attribute`. The policy samples based on string attributes (resource and record) value matches. Both exact and regex value matches are supported.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`key`                    | `string`       | Tag that the filter is matched against. | | yes
`values`                 | `list(string)` | Set of values or regular expressions to use when matching against attribute values. | | yes
`enabled_regex_matching` | `bool`         | Determines whether to match attribute values by regexp string. | false | no
`cache_max_size`         | `string`       | The maximum number of attribute entries of Least Recently Used (LRU) Cache that stores the matched result from the regular expressions defined in `values.` | | no
`invert_match`           | `bool`         | Indicates that values or regular expressions must not match against attribute values. | false | no

### rate_limiting block

The `rate_limiting` block configures a policy of type `rate_limiting`. The policy samples based on rate.

The following arguments are supported:

Name               | Type     | Description                                                         | Default | Required
-------------------|----------|---------------------------------------------------------------------|---------|---------
`spans_per_second` | `number` | Sets the maximum number of spans that can be processed each second. |         | yes

### span_count block

The `span_count` block configures a policy of type `span_count`. The policy samples based on the minimum number of spans within a batch. If all traces within the batch have fewer spans than the threshold, the batch is not sampled.

The following arguments are supported:

Name        | Type     | Description                         | Default | Required
------------|----------|-------------------------------------|---------|---------
`min_spans` | `number` | Minimum number of spans in a trace. |         | yes
`max_spans` | `number` | Maximum number of spans in a trace. | `0`     | no

Set `max_spans` to `0`, if you do not want to limit the policy samples based on the maximum number of spans in a trace.

### boolean_attribute block

The `boolean_attribute` block configures a policy of type `boolean_attribute`.
The policy samples based on a boolean attribute (resource and record).

The following arguments are supported:

Name    | Type     | Description                                                                       | Default | Required
--------|----------|-----------------------------------------------------------------------------------|---------|---------
`key`   | `string` | Attribute key to match against.                                                   |         | yes
`value` | `bool`   | The bool value (`true` or `false`) to use when matching against attribute values. |         | yes

### ottl_condition block

The `ottl_condition` block configures a policy of type `ottl_condition`. The policy samples based on a given boolean
[OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl) condition (span and span event).

The following arguments are supported:

Name         | Type           | Description                                         | Default | Required
-------------|----------------|-----------------------------------------------------|---------|---------
`error_mode` | `string`       | Error handling if OTTL conditions fail to evaluate. |         | yes
`span`       | `list(string)` | OTTL conditions for spans.                          | `[]`    | no
`spanevent`  | `list(string)` | OTTL conditions for span events.                    | `[]`    | no

The supported values for `error_mode` are:
* `ignore`: Ignore errors returned by conditions, log them, and continue on to the next condition. This is the recommended mode.
* `silent`: Ignore errors returned by conditions, do not log them, and continue on to the next condition.
* `propagate`: Return the error up the pipeline. This will result in the payload being dropped from {{< param "PRODUCT_NAME" >}}.

At least one of `span` or `spanevent` should be specified. Both `span` and `spanevent` can also be specified.

### trace_state block

The `trace_state` block configures a policy of type `trace_state`. The policy samples based on TraceState value matches.

The following arguments are supported:

Name     | Type           | Description                                                    | Default | Required
---------|----------------|----------------------------------------------------------------|---------|---------
`key`    | `string`       | Tag that the filter is matched against.                        |         | yes
`values` | `list(string)` | Set of values to use when matching against trace_state values. |         | yes

### and block

The `and` block configures a policy of type `and`. The policy samples based on multiple policies by creating an `and` policy.

### and_sub_policy block

The `and_sub_policy` block configures a sampling policy used by the `and` block. At least one `and_sub_policy` block is required inside an `and` block.

The following arguments are supported:

Name   | Type     | Description                            | Default | Required
-------|----------|----------------------------------------|---------|---------
`name` | `string` | The custom name given to the policy.   |         | yes
`type` | `string` | The valid policy type for this policy. |         | yes

### composite block

The `composite` block configures a policy of type `composite`. This policy samples based on a combination of the above samplers, with ordering and rate allocation per sampler. Rate allocation allocates certain percentages of spans per policy order. For example, if `max_total_spans_per_second` is set to 100, then `rate_allocation` is set as follows:

1. test-composite-policy-1 = 50% of max_total_spans_per_second = 50 spans_per_second
2. test-composite-policy-2 = 25% of max_total_spans_per_second = 25 spans_per_second
3. To ensure remaining capacity is filled, use always_sample as one of the policies.

### composite_sub_policy block

The `composite_sub_policy` block configures a sampling policy used by the `composite` block. At least one`composite_sub_policy` block is required inside a `composite` block.

The following arguments are supported:

Name   | Type     | Description                            | Default | Required
-------|----------|----------------------------------------|---------|---------
`name` | `string` | The custom name given to the policy.   |         | yes
`type` | `string` | The valid policy type for this policy. |         | yes

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.processor.tail_sampling` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.tail_sampling` does not expose any component-specific debug
information.

## Example

This example batches trace data from {{< param "PRODUCT_NAME" >}} before sending it to
[otelcol.exporter.otlp][] for further processing. This example shows an impractical number of policies for the purpose of demonstrating how to set up each type.

```alloy
tracing {
  sampling_fraction = 1
  write_to          = [otelcol.processor.tail_sampling.default.input]
}

otelcol.processor.tail_sampling "default" {
  decision_wait               = "10s"
  num_traces                  = 100
  expected_new_traces_per_sec = 10

  policy {
    name = "test-policy-1"
    type = "always_sample"
  }

  policy {
    name = "test-policy-2"
    type = "latency"

    latency {
      threshold_ms = 5000
    }
  }

  policy {
    name = "test-policy-3"
    type = "numeric_attribute"

    numeric_attribute {
      key       = "key1"
      min_value = 50
      max_value = 100
    }
  }

  policy {
    name = "test-policy-4"
    type = "probabilistic"

    probabilistic {
      sampling_percentage = 10
    }
  }

  policy {
    name = "test-policy-5"
    type = "status_code"

    status_code {
      status_codes = ["ERROR", "UNSET"]
    }
  }

  policy {
    name = "test-policy-6"
    type = "string_attribute"

    string_attribute {
      key    = "key2"
      values = ["value1", "value2"]
    }
  }

  policy {
    name = "test-policy-7"
    type = "string_attribute"

    string_attribute {
      key                    = "key2"
      values                 = ["value1", "val*"]
      enabled_regex_matching = true
      cache_max_size         = 10
    }
  }

  policy {
    name = "test-policy-8"
    type = "rate_limiting"

    rate_limiting {
      spans_per_second = 35
    }
  }

  policy {
    name = "test-policy-9"
    type = "string_attribute"

    string_attribute {
      key                    = "http.url"
      values                 = ["/health", "/metrics"]
      enabled_regex_matching = true
      invert_match           = true
    }
  }

  policy {
    name = "test-policy-10"
    type = "span_count"

    span_count {
      min_spans = 2
    }
  }

  policy {
    name = "test-policy-11"
    type = "trace_state"

    trace_state {
      key    = "key3"
      values = ["value1", "value2"]
    }
  }

  policy {
    name = "test-policy-12"
    type = "ottl_condition"
    ottl_condition {
      error_mode = "ignore"
      span = [
        "attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"",
        "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\"",
      ]
      spanevent = [
        "name != \"test_span_event_name\"",
        "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\"",
      ]
    }
  }

  policy {
    name = "and-policy-1"
    type = "and"

    and {
      and_sub_policy {
        name = "test-and-policy-1"
        type = "numeric_attribute"

        numeric_attribute {
          key       = "key1"
          min_value = 50
          max_value = 100
        }
      }

      and_sub_policy {
        name = "test-and-policy-2"
        type = "string_attribute"

        string_attribute {
          key    = "key1"
          values = ["value1", "value2"]
        }
      }
    }
  }

  policy {
    name = "composite-policy-1"
    type = "composite"

    composite {
      max_total_spans_per_second = 1000
      policy_order               = ["test-composite-policy-1", "test-composite-policy-2", "test-composite-policy-3"]

      composite_sub_policy {
        name = "test-composite-policy-1"
        type = "numeric_attribute"

        numeric_attribute {
          key       = "key1"
          min_value = 50
          max_value = 100
        }
      }

      composite_sub_policy {
        name = "test-composite-policy-2"
        type = "string_attribute"

        string_attribute {
          key    = "key1"
          values = ["value1", "value2"]
        }
      }

      composite_sub_policy {
        name = "test-composite-policy-3"
        type = "always_sample"
      }

      rate_allocation {
        policy  = "test-composite-policy-1"
        percent = 50
      }

      rate_allocation {
        policy  = "test-composite-policy-2"
        percent = 50
      }
    }
  }

  output {
    traces = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = env("OTLP_SERVER_ENDPOINT")
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.tail_sampling` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.tail_sampling` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
