---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.redaction/
description: Learn about otelcol.processor.redaction
labels:
  stage: experimental
  products:
    - oss
title: otelcol.processor.redaction
---

# `otelcol.processor.redaction`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.redaction` removes span, log, and metric attributes that don't match an allowlist.
It masks attribute values that match blocklist expressions and sanitizes high-cardinality URLs and database queries into stable, low-cardinality forms.
It also removes disallowed keys from map-shaped log bodies and masks or sanitizes values in log bodies.

{{< admonition type="note" >}}
`otelcol.processor.redaction` is a wrapper over the upstream OpenTelemetry Collector [`redaction`][] processor.
If necessary, bug reports or feature requests will be redirected to the upstream repository.

[`redaction`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/redactionprocessor
{{< /admonition >}}

You can specify multiple `otelcol.processor.redaction` components by giving them different labels.

## Usage

```alloy
otelcol.processor.redaction "<LABEL>" {
  allowed_keys  = ["<KEY>", ...]
  blocked_values = ["<REGEX>", ...]

  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.processor.redaction`:

| Name                   | Type           | Description                                                                                                                                        | Default | Required |
| ---------------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `allow_all_keys`       | `bool`         | Allow all attribute keys. Disables the `allowed_keys` list.                                                                                        | `false` | no       |
| `allowed_keys`         | `list(string)` | Allowed attribute keys. Keys not on the list are removed.                                                                                          | `[]`    | no       |
| `allowed_values`       | `list(string)` | Regular expressions for values that the processor leaves unchanged, even when they also match blocked value patterns.                              | `[]`    | no       |
| `blocked_key_patterns` | `list(string)` | Regular expressions for attribute keys whose values are masked.                                                                                    | `[]`    | no       |
| `blocked_values`       | `list(string)` | Regular expressions for value substrings that the processor masks after key filtering.                                                             | `[]`    | no       |
| `hash_function`        | `string`       | Function used to hash redacted values instead of masking them with a fixed string.                                                                 | `""`    | no       |
| `hmac_key`             | `secret`       | Secret key used for HMAC hashing when `hash_function` is an HMAC variant.                                                                          | `""`    | no       |
| `ignored_key_patterns` | `list(string)` | Regular expressions for attribute keys that pass through unchanged.                                                                                | `[]`    | no       |
| `ignored_keys`         | `list(string)` | Attribute keys that pass through unchanged.                                                                                                        | `[]`    | no       |
| `redact_all_types`     | `bool`         | Redact non-string attributes as well, by converting them to a string representation.                                                               | `false` | no       |
| `summary`              | `string`       | Controls diagnostic attributes that describe redaction activity. Use `debug` or `info` to send diagnostics. Use `silent` or `""` to suppress them. | `""`    | no       |

If `allow_all_keys` is `false`, only attributes whose keys appear in `allowed_keys` are kept.
The `allowed_keys` list fails closed: if it's empty and `allow_all_keys` is `false`, all attributes are removed.

The processor removes disallowed keys before it checks remaining values against `blocked_values`.
To mask values without removing keys, set `allow_all_keys = true` or include the keys in `allowed_keys`.

`hash_function` accepts one of `sha1`, `sha3`, `md5`, `hmac-sha256`, or `hmac-sha512`.
{{< param "PRODUCT_NAME" >}} validates `hash_function` during configuration parsing.
When you set `hash_function` to `hmac-sha256`, you must also set `hmac_key` to at least 32 bytes.
When you set `hash_function` to `hmac-sha512`, you must also set `hmac_key` to at least 64 bytes.

The processor matches each `blocked_values` expression against attribute values.
It replaces only the substring that matches the expression.
By default, the processor replaces matching text with `****`.
When you set `hash_function`, the processor replaces matching text with a hash of the matched substring.
Regular expressions use the [RE2][] syntax, which doesn't support lookarounds or backreferences.

[RE2]: https://github.com/google/re2/wiki/Syntax

## Blocks

You can use the following blocks with `otelcol.processor.redaction`:

{{< docs/alloy-config >}}

| Block                                    | Description                                                                | Required |
| ---------------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]                       | Configures where to send received telemetry data.                          | yes      |
| [`db_sanitizer`][db_sanitizer]           | Configures database query sanitization.                                    | no       |
| [`db_sanitizer` > `es`][db]              | Configures Elasticsearch query sanitization.                               | no       |
| [`db_sanitizer` > `memcached`][db]       | Configures Memcached command sanitization.                                 | no       |
| [`db_sanitizer` > `mongo`][db]           | Configures MongoDB query sanitization.                                     | no       |
| [`db_sanitizer` > `opensearch`][db]      | Configures OpenSearch query sanitization.                                  | no       |
| [`db_sanitizer` > `redis`][db]           | Configures Redis command sanitization.                                     | no       |
| [`db_sanitizer` > `sql`][db]             | Configures SQL query sanitization.                                         | no       |
| [`db_sanitizer` > `valkey`][db]          | Configures Valkey command sanitization.                                    | no       |
| [`debug_metrics`][debug_metrics]         | Configures the metrics that this component generates to monitor its state. | no       |
| [`url_sanitizer`][url_sanitizer]         | Configures URL sanitization.                                               | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `db_sanitizer` > `sql` refers to a `sql` block defined inside a `db_sanitizer` block.

[output]: #output
[db_sanitizer]: #db_sanitizer
[db]: #db-blocks
[debug_metrics]: #debug_metrics
[url_sanitizer]: #url_sanitizer

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `url_sanitizer`

The `url_sanitizer` block sanitizes high-cardinality URLs, such as routes with embedded IDs or query strings, into stable low-cardinality forms.
This helps prevent span-metrics cardinality explosions.
By default, it also sanitizes matching client and server span names.

| Name                 | Type           | Description                                              | Default | Required |
| -------------------- | -------------- | -------------------------------------------------------- | ------- | -------- |
| `enabled`            | `bool`         | Enable URL sanitization.                                 | `false` | no       |
| `attributes`         | `list(string)` | Attributes to sanitize.                                  | `[]`    | no       |
| `sanitize_span_name` | `bool`         | Whether span names should also be sanitized. Only applies when `enabled` is `true`. Set to `false` to opt out. | `true` | no |

### `db_sanitizer`

The `db_sanitizer` block sanitizes database queries and commands.
By default, it also sanitizes matching client, server, and internal span names.

| Name                 | Type   | Description                             | Default | Required |
| -------------------- | ------ | --------------------------------------- | ------- | -------- |
| `sanitize_span_name` | `bool` | Whether span names should also be sanitized. Only applies when at least one database sanitizer is enabled. Set to `false` to opt out. | `true` | no |

The `db_sanitizer` block contains the `es`, `memcached`, `mongo`, `opensearch`, `redis`, `sql`, and `valkey` blocks described in [`db` blocks](#db-blocks).

### `db` blocks

The `es`, `memcached`, `mongo`, `opensearch`, `redis`, `sql`, and `valkey` blocks each configure sanitization for one database technology.
All of them share the same arguments:

| Name         | Type           | Description                                                        | Default | Required |
| ------------ | -------------- | ----------------------------------------------------------------- | ------- | -------- |
| `enabled`    | `bool`         | Enable sanitization for this database technology.                 | `false` | no       |
| `attributes` | `list(string)` | Attribute keys to apply sanitization to. If empty, all string values are sanitized. | `[]` | no |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                   |
| ------- | ------------------ | ------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | Accepts `otelcol.Consumer` data for metrics, logs, or traces. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.processor.redaction` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.redaction` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.processor.redaction` doesn't expose any component-specific debug metrics.

## Examples

### Keep selected keys and mask credit card numbers

This example keeps only an explicit set of attribute keys and masks any value that looks like a credit card number.

```alloy
otelcol.processor.redaction "default" {
  allowed_keys   = ["description", "group", "id", "name"]
  blocked_values = ["4[0-9]{12}(?:[0-9]{3})?"]

  output {
    traces = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sanitize high-cardinality URLs

This example collapses high-cardinality URLs and span names into stable low-cardinality forms.

```alloy
otelcol.processor.redaction "default" {
  allow_all_keys = true

  url_sanitizer {
    enabled            = true
    attributes         = ["http.url", "url.full"]
    sanitize_span_name = true
  }

  output {
    traces = [otelcol.exporter.otlp.default.input]
  }
}
```

### Redact IP addresses

This example masks IPv4 and IPv6 addresses found in any attribute value.
Each regular expression matches a full address, so the whole address is replaced with `****`.
The regular expressions are written as [raw strings][raw-string] using backticks, so backslashes don't need to be escaped.

```alloy
otelcol.processor.redaction "default" {
  allow_all_keys = true

  blocked_values = [
    // IPv4, for example 203.0.113.42
    `\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`,
    // IPv6, including the "::" compressed forms, for example 2001:db8::1
    `(?i)(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,2}(?::[0-9a-f]{1,4}){1,5}|(?:[0-9a-f]{1,4}:){1,3}(?::[0-9a-f]{1,4}){1,4}|(?:[0-9a-f]{1,4}:){1,4}(?::[0-9a-f]{1,4}){1,3}|(?:[0-9a-f]{1,4}:){1,5}(?::[0-9a-f]{1,4}){1,2}|(?:[0-9a-f]{1,4}:){1,6}:[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,7}:|:(?::[0-9a-f]{1,4}){1,7}|::`,
  ]

  output {
    traces = [otelcol.exporter.otlp.default.input]
  }
}
```

### Redact email addresses

This example masks any email address found in an attribute value, so `john.doe@example.com` becomes `****`.

```alloy
otelcol.processor.redaction "default" {
  allow_all_keys = true

  blocked_values = [
    `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
  ]

  output {
    traces = [otelcol.exporter.otlp.default.input]
  }
}
```

{{< admonition type="note" >}}
The processor replaces the entire matched substring, and RE2 doesn't support lookarounds, so you can't keep only the domain part and produce `****@example.com`.
If you want to hide the local part and keep the domain, match the local part together with `@`, for example `` `[a-zA-Z0-9._%+\-]+@` ``.
That pattern turns `john.doe@example.com` into `****example.com` because the match includes `@`.
{{< /admonition >}}

### Hash matched values

Setting `hash_function` replaces each matched value with a hash instead of a fixed `****` string.
Identical inputs produce identical hashes, so you can still correlate telemetry by a value, such as a client IP, without exposing it.
This example replaces IPv4 addresses with a keyed HMAC-SHA256 digest.

```alloy
otelcol.processor.redaction "default" {
  allow_all_keys = true

  hash_function = "hmac-sha256"
  hmac_key      = sys.env("REDACTION_HMAC_KEY") // must be at least 32 bytes

  blocked_values = [
    `\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`,
  ]

  output {
    traces = [otelcol.exporter.otlp.default.input]
  }
}
```

[raw-string]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#strings

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.redaction` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.redaction` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
