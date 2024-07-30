---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.probabilistic_sampler/
aliases:
  - ../otelcol.processor.probabilistic_sampler/ # /docs/alloy/latest/reference/otelcol.processor.probabilistic_sampler/
description: Learn about telcol.processor.probabilistic_sampler
title: otelcol.processor.probabilistic_sampler
---

# otelcol.processor.probabilistic_sampler

`otelcol.processor.probabilistic_sampler` accepts logs and traces data from other otelcol components and applies probabilistic sampling based on configuration options.

<!-- 
The next few paragraphs were copied from the OTel docs:
https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/probabilisticsamplerprocessor/README.md
-->

The probabilistic sampler processor supports several modes of sampling for spans and log records.  
Sampling is performed on a per-request basis, considering individual items statelessly. 
For whole trace sampling, see `otelcol.processor.tail_sampling`.

For trace spans, this sampler supports probabilistic sampling based on a configured sampling percentage applied to the TraceID.
In addition, the sampler recognizes a `sampling.priority` annotation, which can force the sampler to apply 0% or 100% sampling.

For log records, this sampler can be configured to use the embedded TraceID and follow the same logic as applied to spans.  
When the TraceID is not defined, the sampler can be configured to apply hashing to a selected log record attribute.  
This sampler also supports sampling priority.

{{< admonition type="note" >}}
`otelcol.processor.probabilistic_sampler` is a wrapper over the upstream OpenTelemetry Collector Contrib `probabilistic_sampler` processor.
If necessary, bug reports or feature requests will be redirected to the upstream repository.
{{< /admonition >}}

You can specify multiple `otelcol.processor.probabilistic_sampler` components by giving them different labels.

## Usage

```alloy
otelcol.processor.probabilistic_sampler "LABEL" {
  output {
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.probabilistic_sampler` supports the following arguments:

Name                  | Type      | Description                                                                                                          | Default          | Required
----------------------|-----------|----------------------------------------------------------------------------------------------------------------------|------------------|---------
`mode`                | `string`  | Sampling mode.                                                                                                       | `"proportional"` | no
`hash_seed`           | `uint32`  | An integer used to compute the hash algorithm.                                                                       | `0`              | no
`sampling_percentage` | `float32` | Percentage of traces or logs sampled.                                                                                | `0`              | no
`sampling_precision`  | `int`     | The number of hexadecimal digits used to encode the sampling threshold.                                              | `4`              | no
`fail_closed`         | `bool`    | Whether to reject items with sampling-related errors.                                                                | `true`           | no
`attribute_source`    | `string`  | Defines where to look for the attribute in `from_attribute`.                                                         | `"traceID"`      | no
`from_attribute`      | `string`  | The name of a log record attribute used for sampling purposes.                                                       | `""`             | no
`sampling_priority`   | `string`  | The name of a log record attribute used to set a different sampling priority from the `sampling_percentage` setting. | `""`             | no

You can set `mode` to `"proportional"`, `"equalizing"`, or `"hash_seed"`.
The default is `"proportional"` unless either `hash_seed` is configured or `attribute_source` is set to `record`.
For more information on modes, refer to the [Sampling Modes](#sampling-modes) section.

`hash_seed` determines an integer to compute the hash algorithm. This argument could be used for both traces and logs.
When used for logs, it computes the hash of a log record.
For hashing to work, all collectors for a given tier, for example, behind the same load balancer, must have the same `hash_seed`.
It is also possible to leverage a different `hash_seed` at different collector tiers to support additional sampling requirements.

`sampling_percentage` determines the percentage at which traces or logs are sampled. All traces or logs are sampled if you set this argument to a value greater than or equal to 100.

`attribute_source` (logs only) determines where to look for the attribute in `from_attribute`. The allowed values are `traceID` or `record`.

`from_attribute` (logs only) determines the name of a log record attribute used for sampling purposes, such as a unique log record ID. The value of the attribute is only used if the trace ID is absent or if `attribute_source` is set to `record`.

`sampling_priority` (logs only) determines the name of a log record attribute used to set a different sampling priority from the `sampling_percentage` setting. 0 means to never sample the log record, and greater than or equal to 100 means to always sample the log record.

The `probabilistic_sampler` supports two types of sampling for traces:
1. `sampling.priority` [semantic convention](https://github.com/opentracing/specification/blob/master/semantic_conventions.md#span-tags-table) as defined by OpenTracing.
2. Trace ID hashing.

The `sampling.priority` semantic convention takes priority over trace ID hashing.
Trace ID hashing samples based on hash values determined by trace IDs.

The `probabilistic_sampler` supports sampling logs according to their trace ID, or by a specific log record attribute.

`sampling_precision` must be a value between 1 and 14 (inclusive).

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.probabilistic_sampler`:

Hierarchy | Block      | Description                          | Required
----------|------------|--------------------------------------|---------
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[debug_metrics]: #debug_metrics-block

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` OTLP-formatted data for any telemetry signal of these types:
* logs
* traces

<!-- 
The next few sections were copied from the OTel docs:
https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/probabilisticsamplerprocessor/README.md
-->

## Consistency guarantee

A consistent probability sampler is a Sampler that supports
independent sampling decisions for each span or log record in a group
(e.g. by TraceID), while maximizing the potential for completeness as
follows.

Consistent probability sampling requires that for any span in a given
trace, if a Sampler with lesser sampling probability selects the span
for sampling, then the span would also be selected by a Sampler
configured with greater sampling probability.

## Completeness property

A trace is complete when all of its members are sampled.  A
"sub-trace" is complete when all of its descendents are sampled.

Ordinarily, Trace and Logging OpenTelemetry SDKs configure parent-based samplers
which decide to sample based on the Context, because it leads to
completeness.

When non-root spans or logs make independent sampling decisions
instead of using the parent-based approach (e.g., using the
`TraceIDRatioBased` sampler for a non-root span), incompleteness may
result, and when spans and log records are independently sampled in a
processor, as by this component, the same potential for completeness
arises.  The consistency guarantee helps minimimize this issue.

Consistent probability samplers can be safely used with a mixture of
probabilities and preserve sub-trace completeness, provided that child
spans and log records are sampled with probability greater than or
equal to the parent context.

Using 1%, 10% and 50% probabilities for example, in a consistent
probability scheme the 50% sampler must sample when the 10% sampler
does, and the 10% sampler must sample when the 1% sampler does.  A
three-tier system could be configured with 1% sampling in the first
tier, 10% sampling in the second tier, and 50% sampling in the bottom
tier.  In this configuration, 1% of traces will be complete, 10% of
traces will be sub-trace complete at the second tier, and 50% of
traces will be sub-trace complete at the third tier thanks to the
consistency property.

These guidelines should be considered when deploying multiple
collectors with different sampling probabilities in a system.  For
example, a collector serving frontend servers can be configured with
smaller sampling probability than a collector serving backend servers,
without breaking sub-trace completeness.

## Sampling randomness

To achieve consistency, sampling randomness is taken from a
deterministic aspect of the input data.  For traces pipelines, the
source of randomness is always the TraceID.  For logs pipelines, the
source of randomness can be the TraceID or another log record
attribute, if configured.

For log records, the `attribute_source` and `from_attribute` fields determine the
source of randomness used for log records.  When `attribute_source` is
set to `traceID`, the TraceID will be used.  When `attribute_source`
is set to `record` or the TraceID field is absent, the value of
`from_attribute` is taken as the source of randomness (if configured).

## Sampling priority

The sampling priority mechanism is an override, which takes precedence
over the probabilistic decision in all modes.

{{< admonition type="note" >}}
Logs and Traces have different behavior.
{{< /admonition >}}

In traces pipelines, when the priority attribute has value 0, the
configured probability will be modified to 0% and the item will not
pass the sampler.  When the priority attribute is non-zero, the
configured probability will be set to 100%.  The sampling priority
attribute is not configurable, and is called `sampling.priority`.

In logs pipelines, when the priority attribute has value 0, the
configured probability will be modified to 0%, and the item will not
pass the sampler.  Otherwise, the logs sampling priority attribute is
interpreted as a percentage, with values >= 100 equal to 100%
sampling.  The logs sampling priority attribute is configured via
`sampling_priority`.

## Sampling modes

There are three sampling modes available.  All modes are consistent.

### Hash seed

The hash seed method uses the FNV hash function applied to either a
Trace ID (spans, log records), or to the value of a specified
attribute (only logs).  The hashed value, presumed to be random, is
compared against a threshold value that corresponds with the sampling
percentage.

This mode requires configuring the `hash_seed` field.  This mode is
enabled when the `hash_seed` field is not zero, or when log records
are sampled with `attribute_source` is set to `record`.

In order for hashing to be consistent, all collectors for a given tier
(e.g. behind the same load balancer) must have the same
`hash_seed`. It is also possible to leverage a different `hash_seed`
at different collector tiers to support additional sampling
requirements.

This mode uses 14 bits of information in its sampling decision; the
default `sampling_precision`, which is 4 hexadecimal digits, exactly
encodes this information.

This mode is selected by default.

#### Hash seed: Use-cases

The hash seed mode is most useful in logs sampling, because it can be
applied to units of telemetry other than TraceID.  For example, a
deployment consisting of 100 pods can be sampled according to the
`service.instance.id` resource attribute.  In this case, 10% sampling
implies collecting log records from an expected value of 10 pods.

### Proportional

OpenTelemetry specifies a consistent sampling mechanism using 56 bits
of randomness, which may be obtained from the Trace ID according to
the W3C Trace Context Level 2 specification.  Randomness can also be
explicly encoding in the OpenTelemetry `tracestate` field, where it is
known as the R-value.

This mode is named because it reduces the number of items transmitted
proportionally, according to the sampling probability.  In this mode,
items are selected for sampling without considering how much they were
already sampled by preceding samplers.

This mode uses 56 bits of information in its calculations.  The
default `sampling_precision` (4) will cause thresholds to be rounded
in some cases when they contain more than 16 significant bits.

#### Proportional: Use-cases

The proportional mode is generally applicable in trace sampling,
because it is based on OpenTelemetry and W3C specifications.  This
mode is selected by default, because it enforces a predictable
(probabilistic) ratio between incoming items and outgoing items of
telemetry.  No matter how SDKs and other sources of telemetry have
been configured with respect to sampling, a collector configured with
25% proportional sampling will output (an expected value of) 1 item
for every 4 items input.

### Equalizing

This mode uses the same randomness mechanism as the propotional
sampling mode, in this case considering how much each item was already
sampled by preceding samplers.  This mode can be used to lower
sampling probability to a minimum value across a whole pipeline, 
making it possible to conditionally adjust sampling probabilities.

This mode compares a 56 bit threshold against the configured sampling
probability and updates when the threshold is larger.  The default
`sampling_precision` (4) will cause updated thresholds to be rounded
in some cases when they contain more than 16 significant bits.

#### Equalizing: Use-cases

The equalizing mode is useful in collector deployments where client
SDKs have mixed sampling configuration and the user wants to apply a
uniform sampling probability across the system.  For example, a user's
system consists of mostly components developed in-house, but also some
third-party software.  Seeking to lower the overall cost of tracing,
the configures 10% sampling in the samplers for all of their in-house
components.  This leaves third-party software components unsampled,
making the savings less than desired.  In this case, the user could
configure a 10% equalizing probabilistic sampler.  Already-sampled
items of telemetry from the in-house components will pass-through one
for one in this scenario, while items of telemetry from third-party
software will be sampled by the intended amount.

## Sampling threshold information

In all modes, information about the effective sampling probability is
added into the item of telemetry.  The random variable that was used
may also be recorded, in case it was not derived from the TraceID
using a standard algorithm.

For traces, threshold and optional randomness information are encoded
in the W3C Trace Context `tracestate` fields.  The tracestate is
divided into sections according to a two-character vendor code;
OpenTelemetry uses "ot" as its section designator.  Within the
OpenTelemetry section, the sampling threshold is encoded using "th"
and the optional random variable is encoded using "rv".

For example, 25% sampling is encoded in a tracing Span as:

```
tracestate: ot=th:c
```

Users can randomness values in this way, independently, making it
possible to apply consistent sampling across traces for example.  If
the Trace was initialized with pre-determined randomness value
`9b8233f7e3a151` and 100% sampling, it would read:

```
tracestate: ot=th:0;rv:9b8233f7e3a151
```

This component, using either proportional or equalizing modes, could
apply 50% sampling the Span.  This span with randomness value
`9b8233f7e3a151` is consistently sampled at 50% because the threshold,
when zero padded (i.e., `80000000000000`), is less than the randomess
value.  The resulting span will have the following tracestate:

```
tracestate: ot=th:8;rv:9b8233f7e3a151
```

For log records, threshold and randomness information are encoded in
the log record itself, using attributes.  For example, 25% sampling
with an explicit randomness value is encoded as:

```
sampling.threshold: c
sampling.randomness: e05a99c8df8d32
```

### Sampling precision

When encoding sampling probability in the form of a threshold,
variable precision is permitted making it possible for the user to
restrict sampling probabilities to rounded numbers of fixed width.

Because the threshold is encoded using hexadecimal digits, each digit
contributes 4 bits of information.  One digit of sampling precision
can express exact sampling probabilities 1/16, 2/16, ... through
16/16.  Two digits of sampling precision can express exact sampling
probabilities 1/256, 2/256, ... through 256/256.  With N digits of
sampling precision, there are exactly `(2^N)-1` exactly representable
probabilities.

Depending on the mode, there are different maximum reasonable settings
for this parameter.

- The `hash_seed` mode uses a 14-bit hash function, therefore
  precision 4 completely captures the available information.
- The `equalizing` mode configures a sampling probability after
  parsing a `float32` value, which contains 20 bits of precision,
  therefore precision 5 completely captures the available information.
- The `proportional` mode configures its ratio using a `float32`
  value, however it carries out the arithmetic using 56-bits of
  precision.  In this mode, increasing precision has the effect
  of preserving precision applied by preceding samplers.

In cases where larger precision is configured than is actually
available, the added precision has no effect because trailing zeros
are eliminated by the encoding.

### Error handling

This processor considers it an error when the arriving data has no
randomness.  This includes conditions where the TraceID field is
invalid (16 zero bytes) and where the log record attribute source has
zero bytes of information.

By default, when there are errors determining sampling-related
information from an item of telemetry, the data will be refused.  This
behavior can be changed by setting the `fail_closed` property to
false, in which case erroneous data will pass through the processor.

## Component health

`otelcol.processor.probabilistic_sampler` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.probabilistic_sampler` does not expose any component-specific debug
information.

## Examples

### Basic usage

```alloy
otelcol.processor.probabilistic_sampler "default" {
  hash_seed           = 123
  sampling_percentage = 15.3

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sample 15% of the logs

```alloy
otelcol.processor.probabilistic_sampler "default" {
  sampling_percentage = 15

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sample logs according to their "logID" attribute

```alloy
otelcol.processor.probabilistic_sampler "default" {
  sampling_percentage = 15
  attribute_source    = "record"
  from_attribute      = "logID"

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sample logs according to a "priority" attribute

```alloy
otelcol.processor.probabilistic_sampler "default" {
  sampling_percentage = 15
  sampling_priority   = "priority"

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.probabilistic_sampler` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.probabilistic_sampler` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
