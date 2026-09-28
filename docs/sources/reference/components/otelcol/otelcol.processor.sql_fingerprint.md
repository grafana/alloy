---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.sql_fingerprint/
description: Add SQL query fingerprints to trace spans for database query correlation
labels:
  stage: experimental
  products:
    - oss
title: otelcol.processor.sql_fingerprint
---

# `otelcol.processor.sql_fingerprint`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.sql_fingerprint` adds SQL query fingerprints to incoming trace spans.
Your database observability backend can fingerprint query text from database statistics with the same Go package and search traces for that fingerprint.
The component supports PostgreSQL, MySQL, and Microsoft SQL Server query shapes.
It doesn't connect to databases.

This is a custom Grafana Alloy component, unrelated to processors in the OpenTelemetry Collector.
You can define multiple instances with different labels.

## Usage

```alloy
otelcol.processor.sql_fingerprint "<LABEL>" {
  output {
    traces = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.processor.sql_fingerprint`:

| Name | Type | Description | Default | Required |
| --- | --- | --- | --- | --- |
| `max_query_bytes` | `int` | Maximum SQL input size in bytes. | `1048576` | no |
| `mysql_ansi_quotes` | `bool` | Interpret MySQL double quotes as identifier delimiters. | `false` | no |
| `mysql_no_backslash_escapes` | `bool` | Disable backslash escapes in MySQL string literals. | `false` | no |
| `postgresql_backslash_escapes` | `bool` | Interpret ordinary PostgreSQL strings with `standard_conforming_strings=off`. | `false` | no |
| `postgresql_version` | `int` | PostgreSQL major version for list normalization: `16`, `17`, or `18`. | `18` | no |
| `sql_server_quoted_identifier_off` | `bool` | Interpret SQL Server double quotes as string delimiters. | `false` | no |

Use the same options and library version in the trace pipeline and your backend.
Route sources with different SQL modes or PostgreSQL versions through separate component instances.
The component can't infer session modes from SQL text.

The component prefers nonempty string values in these span attributes:

| Attribute | Legacy fallback | Purpose |
| --- | --- | --- |
| `db.query.text` | `db.statement` | SQL query or batch text. |
| `db.system.name` | `db.system` | Database dialect: `postgresql`, `mysql`, or `microsoft.sql_server`. |

The legacy dialect values `postgres` and `mssql` are also accepted.
Resource attributes aren't used to select the dialect.

The output attribute `db.query.fingerprint` is always an array of unique strings, including for a single statement.
Values have the form `v5:<DIALECT>:<SHA256>`.
The component replaces this attribute on recognized SQL spans and removes stale values if recomputation produces no fingerprints.
It preserves SQL text, span timing, and other attributes.

Semicolon-separated batches produce one fingerprint per distinct supported statement.
Statements with detected parsing failures are skipped and counted.
Successful statements from the same batch are retained.
An unterminated quote, comment, or procedural context stops processing the remainder of the batch.
All spans continue downstream, including spans without fingerprints.

### Normalization coverage

The experimental normalizer handles literal and parameter substitution, quoted identifiers, comments, and native list markers for common data-manipulation statements.
PostgreSQL recognizes named Python `%(name)s` placeholders in expression value positions, including placeholders followed by casts such as `::INTEGER`.
PostgreSQL 16 and 17 retain constant list lengths; PostgreSQL 18 constant `IN` and `ARRAY` lists and MySQL digest lists are collapsed.
SQL Server list lengths are retained.
Explicit schema qualification remains significant.

Simple MySQL `INSERT [IGNORE] [INTO] table (columns) VALUES ...` statements match regardless of column order when every row reduces to a constant-value marker, including the native `(...)` marker.
Column names, table qualification, and the distinction between single and repeated rows remain significant.
INSERTs with expressions, `DEFAULT`, `SELECT`, row aliases, or trailing clauses retain column order.

This implementation is a structural normalizer, not a complete replica of each database parser.
It doesn't support stored procedures, procedural or transaction blocks, DDL, execution wrappers, optimizer hints, executable comments, Unicode escape identifiers, client batch directives, or batches without semicolon separators.
Full compatibility across database versions hasn't been established by live-server conformance tests.

Supply complete SQL rather than truncated UI text or query summaries.
The component rejects detected truncation and input exceeding its limits instead of hashing a prefix.
Limits also include 65,536 tokens per statement, 128 nested delimiters, and 256 statements per batch.
A text fingerprint doesn't resolve database objects, search paths, or collations.
Scope your trace search to the database instance, database name, and relevant time range.

## Blocks

You can use the following blocks with `otelcol.processor.sql_fingerprint`:

{{< docs/alloy-config >}}

| Block | Description | Required |
| --- | --- | --- |
| [`output`][output] | Configures where to send processed traces. | yes |

[output]: #output

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-traces.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name | Type | Description |
| --- | --- | --- |
| `input` | `otelcol.Consumer` | Receives OTLP-formatted traces. |

## Component health

`otelcol.processor.sql_fingerprint` is only reported as unhealthy if given an invalid configuration.
Individual query failures are reported through counters.

## Debug information

`otelcol.processor.sql_fingerprint` doesn't expose any component-specific debug information.
You can inspect processed traces with live debugging.

## Debug metrics

The following Prometheus metrics are exposed:

| Name | Type | Description |
| --- | --- | --- |
| `otelcol_processor_sql_fingerprint_spans_total` | `counter` | Spans containing SQL text, labeled by `db_system` and `outcome` (`fingerprinted` or `skipped`). |
| `otelcol_processor_sql_fingerprint_failures_total` | `counter` | Failures labeled by `db_system` and `reason`. |

Failure reasons are `invalid_sql`, `unsupported`, `truncated`, `limit`, and `unsupported_db_system`.
An unsupported or missing database system is labeled `unknown`.
One span can have several statement failures and still produce fingerprints.
Counters don't contain SQL text, database names, or fingerprint values.

## Example

This pipeline accepts application traces, adds fingerprints, and forwards batches to a local Tempo OTLP listener on port 4319.
Run Alloy with `--stability.level=experimental`.
Adjust the exporter endpoint to match your Tempo deployment.

```alloy
otelcol.receiver.otlp "applications" {
  grpc {
    endpoint = "127.0.0.1:4317"
  }
  http {
    endpoint = "127.0.0.1:4318"
  }
  output {
    traces = [otelcol.processor.sql_fingerprint.queries.input]
  }
}

otelcol.processor.sql_fingerprint "queries" {
  postgresql_version = 18
  output {
    traces = [otelcol.processor.batch.traces.input]
  }
}

otelcol.processor.batch "traces" {
  output {
    traces = [otelcol.exporter.otlp.tempo.input]
  }
}

otelcol.exporter.otlp "tempo" {
  client {
    endpoint = "127.0.0.1:4319"
    tls {
      insecure = true
    }
  }
}
```

Your backend can import `github.com/grafana/alloy/sqlfingerprint` and call `Fingerprint` on the selected database statistics text.
For each returned value, search Tempo with an array-element equality query:

```traceql
{ span.db.query.fingerprint = "v5:postgresql:<HASH>" }
```

Replace _`<HASH>`_ with the returned SHA-256 value.
See [Tempo array queries](https://grafana.com/docs/tempo/latest/traceql/construct-traceql-queries/#find-traces-with-arrays) for storage-format requirements.
