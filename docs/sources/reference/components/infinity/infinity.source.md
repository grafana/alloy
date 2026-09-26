---
canonical: https://grafana.com/docs/alloy/latest/reference/components/infinity/infinity.source/
description: Learn about infinity.source
labels:
  stage: experimental
  products:
    - oss
title: infinity.source
---

# `infinity.source`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`infinity.source` polls HTTP APIs or inline data, parses JSON, CSV, TSV, XML, HTML, or GraphQL responses with the Grafana Infinity backend parsers, and sends the result as current-state metrics or as log entries to other components.

You can specify multiple `infinity.source` components by giving them different labels.

## Usage

```alloy
infinity.source "<LABEL>" {
  query "<QUERY_LABEL>" {
    url = "<URL>"
  }

  forward_to {
    metrics = <RECEIVER_LIST>
  }
}
```

## Arguments

You can use the following arguments with `infinity.source`:

| Name                 | Type       | Description                                             | Default | Required |
| -------------------- | ---------- | --------------------------------------------------------| ------- | -------- |
| `interval`           | `duration` | How often `infinity.source` polls each query.           | `"60s"` | no       |
| `max_response_size`  | `string`   | Maximum size of a query's decompressed HTTP response.   | `"10MiB"` | no     |
| `timeout`            | `duration` | Timeout for one poll of one query.                       | `"10s"` | no       |

The `timeout` argument must be less than or equal to the `interval` argument.
`timeout` bounds the HTTP request, reading its response body, and parsing the response.
It doesn't bound sending the result to receivers.
When parsing takes longer than `timeout`, the poll fails with the `timeout` reason.
Refer to [Limitations](#limitations) for what happens to a parser expression that doesn't end.

`infinity.source` measures the `max_response_size` argument against the decompressed response body.
A compressed response can be smaller on the wire than `max_response_size` and still fail this limit once decompressed.

## Blocks

You can use the following blocks with `infinity.source`:

{{< docs/alloy-config >}}

| Block                                                              | Description                                                       | Required |
| ------------------------------------------------------------------- | ------------------------------------------------------------------ | -------- |
| [`query`][query]                                                     | One query to poll and parse. You can specify `query` multiple times. | yes    |
| `query` > [`column`][column]                                         | Selects and types one field from the response.                     | no       |
| `query` > [`computed_column`][computed_column]                       | Adds a field computed from an expression.                          | no       |
| `query` > [`csv_options`][csv_options]                               | Parsing options for `type = "csv"` or `type = "tsv"`.               | no       |
| `query` > [`logs`][logs]                                             | Settings for a query with `format = "logs"`.                       | no       |
| `query` > [`metrics`][metrics_block]                                 | Settings for a query with `format = "table"`.                      | no       |
| `query` > [`transform.limit`][transform.limit]                       | Keeps only the first rows of the frame.                             | no       |
| `query` > [`transform.filter`][transform.filter]                     | Keeps only the rows that match an expression.                      | no       |
| `query` > [`transform.summarize`][transform.summarize]               | Replaces the frame with a single summarized frame.                 | no       |
| `query` > [`transform.computed_column`][transform.computed_column]   | Adds a field computed from an expression.                          | no       |
| `query` > [`url_options`][url_options]                               | HTTP request options for a query with `source = "url"`.            | no       |
| [`client`][client]                                                   | HTTP client settings used for queries with `source = "url"`.       | no       |
| `client` > [`basic_auth`][basic_auth]                                | Configure `basic_auth` for authenticating to the endpoint.          | no       |
| `client` > [`authorization`][authorization]                          | Configure generic authorization to the endpoint.                    | no       |
| `client` > [`oauth2`][oauth2]                                        | Configure OAuth 2.0 for authenticating to the endpoint.             | no       |
| `client` > `oauth2` > [`tls_config`][tls_config]                     | Configure TLS settings for connecting to the endpoint via OAuth 2.0.| no       |
| `client` > [`tls_config`][tls_config]                                | Configure TLS settings for connecting to the endpoint.              | no       |
| [`clustering`][clustering]                                           | Distribute query polling across cluster nodes.                      | no       |
| [`forward_to`][forward_to]                                           | Alloy-native receivers for the results.                             | no       |
| [`output`][output]                                                   | OpenTelemetry consumers for the results.                            | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `query` > `url_options` refers to a `url_options` block defined inside a `query` block.

[query]: #query
[column]: #column
[computed_column]: #computed_column
[csv_options]: #csv_options
[logs]: #logs
[metrics_block]: #metrics
[transform.limit]: #transformlimit
[transform.filter]: #transformfilter
[transform.summarize]: #transformsummarize
[transform.computed_column]: #transformcomputed_column
[url_options]: #url_options
[client]: #client
[basic_auth]: #basic_auth
[authorization]: #authorization
[oauth2]: #oauth2
[tls_config]: #tls_config
[clustering]: #clustering
[forward_to]: #forward_to
[output]: #output

{{< /docs/alloy-config >}}

### `query`

The `query "<LABEL>"` block configures one query that `infinity.source` polls and parses.
Specify `query` one or more times, each with a unique label.
The label is used as the `instance` label on every sample and log entry the query produces.

| Name                    | Type       | Description                                                            | Default    | Required |
| ------------------------ | ---------- | ----------------------------------------------------------------------- | ---------- | -------- |
| `data`                   | `string`   | Inline response body. Required when `source` is `"inline"`.             |            | no       |
| `filter_expression`      | `string`   | Expression that keeps only the matching rows after parsing.             |            | no       |
| `format`                 | `string`   | `"table"` for current-state metrics, or `"logs"` for log entries.       | `"table"`  | no       |
| `parser`                 | `string`   | Parser used to build the frame.                                        | `"backend"`| no       |
| `root_selector`          | `string`   | Selects the part of the response to parse into rows.                    |            | no       |
| `source`                 | `string`   | `"url"` to fetch the response over HTTP, or `"inline"` to use `data`.   | `"url"`    | no       |
| `summarize_alias`        | `string`   | Name of the resulting column, when `summarize_expression` is set.       | `"summary"`| no       |
| `summarize_by`           | `string`   | Column to group rows by, when `summarize_expression` is set.            |            | no       |
| `summarize_expression`   | `string`   | Expression that summarizes the frame into one row.                      |            | no       |
| `type`                   | `string`   | Response format to parse.                                               | `"json"`   | no       |
| `url`                    | `string`   | URL to poll. Required when `source` is `"url"`.                         |            | no       |

The following strings are valid `type` values:

* `"json"`: Parse the response as JSON.
* `"csv"`: Parse the response as comma-separated values.
* `"tsv"`: Parse the response as tab-separated values.
* `"xml"`: Parse the response as XML.
* `"html"`: Parse the response with the XML parser.
  `infinity.source` doesn't extract HTML tables.
  Malformed markup can parse into one string field with an empty name, without an error.
* `"graphql"`: Parse a GraphQL JSON response. This implies `url_options.method = "POST"` and `url_options.body_type = "graphql"`.

The following strings are valid `parser` values:

* `"backend"`: Use the Grafana Infinity backend parser.
* `"jq-backend"`: Use the backend parser with a `jq` expression as `root_selector`.

The following strings are valid `format` values:

* `"table"`: Each row becomes a current-state metric sample. Refer to [Metrics](#metrics-1).
* `"logs"`: Each row becomes a log entry. Refer to [Logs](#logs-1).

The following strings are valid `source` values:

* `"url"`: Fetch the response with an HTTP request to `url`.
* `"inline"`: Use the `data` argument as the response body, without making an HTTP request.

When `source` is `"url"`, the `url` argument is required, and must be an absolute `http` or `https` URL.
When `source` is `"inline"`, the `data` argument is required.

Set `filter_expression` to drop rows after parsing.
Set `summarize_expression` to replace the frame with a single row that summarizes it; `summarize_by` groups rows before summarizing, and `summarize_alias` names the resulting column.

`infinity.source` post-processes each parsed frame in a fixed order: computed columns, then `filter_expression`, then the summary when `summarize_expression` is set, and finally each `transform` block, in the order the blocks appear in the configuration.

### `column`

The `column` block selects and types one field from the response.
Specify `column` zero or more times.

| Name               | Type     | Description                                       | Default        | Required |
| ------------------- | -------- | -------------------------------------------------- | -------------- | -------- |
| `selector`          | `string` | Selects the field to extract from the response.    |                | yes      |
| `text`              | `string` | Name of the resulting column.                      | from `selector`| no       |
| `timestamp_format`  | `string` | Format used to parse a `type = "timestamp"` value. |                | no       |
| `type`              | `string` | Type to convert the extracted value to.            | autodetected   | no       |

The following strings are valid `type` values: `"string"`, `"number"`, `"boolean"`, `"timestamp"`, `"timestamp_epoch"`, and `"timestamp_epoch_s"`.
The `timestamp_format` argument is valid only when `type` is `"timestamp"`.
When you don't set `type`, `infinity.source` lets the parser detect the column's type from the response.

### `computed_column`

The `computed_column` block adds a field computed from an expression.
Specify `computed_column` zero or more times.

| Name        | Type     | Description                             | Default | Required |
| ----------- | -------- | ---------------------------------------- | ------- | -------- |
| `selector`  | `string` | Expression that computes the value.      |         | yes      |
| `text`      | `string` | Name of the resulting column.            |         | yes      |

`infinity.source` doesn't apply a `type` to a computed column; the computed value keeps the type the expression produces.

### `csv_options`

The `csv_options` block configures parsing for `type = "csv"` and `type = "tsv"` queries.

| Name                      | Type     | Description                                                        | Default | Required |
| -------------------------- | -------- | -------------------------------------------------------------------- | ------- | -------- |
| `columns`                 | `string` | Header line to use instead of the response's own first line.       |         | no       |
| `comment`                 | `string` | Line prefix that marks a line as a comment to skip.                 |         | no       |
| `delimiter`               | `string` | Field delimiter.                                                     | `","`   | no       |
| `relax_column_count`      | `bool`   | Allow rows with a different number of columns than the header.      | `false` | no       |
| `skip_lines_with_error`   | `bool`   | Skip rows that fail to parse instead of failing the query.          | `false` | no       |

For `type = "tsv"` queries, `infinity.source` ignores `delimiter` and always splits fields on a tab.

Set `columns` to `"-"` or `"none"` to treat the response as headerless, so `infinity.source` generates column names automatically.
Any other non-empty value for `columns` is added before the response body as a header line.
`infinity.source` doesn't drop the response's own first line: it becomes a data row.

### `logs`

The `logs` block configures a query with `format = "logs"`.

| Name                           | Type            | Description                                    | Default  | Required |
| ------------------------------- | --------------- | ------------------------------------------------ | -------- | -------- |
| `entry_limit`                  | `number`        | Maximum number of log entries per poll.          | `0`      | no       |
| `label_columns`                | `list(string)`  | Columns to copy to log labels.                   | `[]`     | no       |
| `line_column`                  | `string`        | Column to use as the log line.                   | `"body"` | no       |
| `structured_metadata_columns`  | `list(string)`  | Columns to copy to structured metadata.          | `[]`     | no       |

A `logs` block requires `format = "logs"` on the enclosing `query` block.
A value of `0` for `entry_limit` disables the limit.
Refer to [Logs](#logs-1) for how `infinity.source` maps rows to log entries using these arguments.

### `metrics`

The `metrics` block configures a query with `format = "table"`.

| Name             | Type     | Description                                       | Default | Required |
| ----------------- | -------- | --------------------------------------------------- | ------- | -------- |
| `prefix`         | `string` | Prefix added to every metric name from this query. |         | no       |
| `series_limit`   | `number` | Maximum number of samples per poll.                | `0`     | no       |

A `metrics` block requires `format = "table"` on the enclosing `query` block.
The `prefix` argument must match `^[a-zA-Z_:][a-zA-Z0-9_:]*$`.
A value of `0` for `series_limit` disables the limit.
Refer to [Metrics](#metrics-1) for how `infinity.source` maps rows to samples using these arguments.

### `transform.limit`

The `transform.limit` inner block keeps only the first rows of the frame.
Specify `transform.limit` zero or more times.

| Name    | Type     | Description                      | Default | Required |
| ------- | -------- | ---------------------------------- | ------- | -------- |
| `limit` | `number` | Maximum number of rows to keep.    |         | yes      |

### `transform.filter`

The `transform.filter` inner block keeps only the rows that match an expression.
Specify `transform.filter` zero or more times.

| Name          | Type     | Description                                  | Default | Required |
| -------------- | -------- | ----------------------------------------------- | ------- | -------- |
| `expression`  | `string` | Expression a row must match to be kept.        |         | yes      |

### `transform.summarize`

The `transform.summarize` inner block replaces the frame with a single summarized frame.
Specify `transform.summarize` zero or more times.

| Name          | Type     | Description                                    | Default     | Required |
| -------------- | -------- | -------------------------------------------------| ----------- | -------- |
| `expression`  | `string` | Expression that summarizes the frame.           |             | yes      |
| `by`          | `string` | Column to group rows by before summarizing.     |             | no       |
| `alias`       | `string` | Name of the resulting summary column.           | `expression` | no      |

When you don't set `alias`, the summary column uses the `expression` text as its name.

### `transform.computed_column`

The `transform.computed_column` inner block adds a field computed from an expression.
Specify `transform.computed_column` zero or more times.

| Name          | Type     | Description                          | Default | Required |
| -------------- | -------- | --------------------------------------- | ------- | -------- |
| `expression`  | `string` | Expression that computes the value.    |         | yes      |
| `alias`       | `string` | Name of the resulting column.          |         | yes      |

`infinity.source` runs `transform.limit`, `transform.filter`, `transform.summarize`, and `transform.computed_column` blocks in the order they appear inside their `query` block, after computed columns, `filter_expression`, and the summary.

### `url_options`

The `url_options` block configures the HTTP request for a query with `source = "url"`.

| Name                       | Type          | Description                                                          | Default | Required |
| --------------------------- | ------------- | ----------------------------------------------------------------------| ------- | -------- |
| `body`                     | `string or secret` | Request body, used when `body_type` is `"raw"`.                | | no       |
| `body_content_type`        | `string`      | `Content-Type` header, used when `body_type` is `"raw"`.             |         | no       |
| `body_form`                | `map(secret)` | Form fields, used when `body_type` is `"form-data"` or `"x-www-form-urlencoded"`. | | no |
| `body_graphql_query`       | `string or secret` | GraphQL query, used when `body_type` is `"graphql"`.            |         | no       |
| `body_graphql_variables`   | `string`      | GraphQL variables as a JSON object, used when `body_type` is `"graphql"`. | | no  |
| `body_type`                | `string`      | Encoding of the request body.                                        | `"raw"` | no       |
| `headers`                  | `map(secret)` | Extra HTTP headers, added after the headers from `client`.           |         | no       |
| `method`                   | `string`      | HTTP method for the request.                                         | `"GET"` | no       |
| `params`                   | `map(secret)` | Extra URL query parameters.                                          |         | no       |

The `method` argument accepts `"GET"`, `"POST"`, `"PUT"`, `"PATCH"`, and `"DELETE"`, case-insensitively.

The following strings are valid `body_type` values:

* `"raw"`: Send `body` as is. Set `body_content_type` to set the request's `Content-Type` header.
* `"form-data"`: Encode `body_form` as `multipart/form-data`.
* `"x-www-form-urlencoded"`: Encode `body_form` as a URL-encoded form.
* `"graphql"`: Send `body_graphql_query` and `body_graphql_variables` as a JSON object, with a `Content-Type` of `application/json`.

A request body with `method = "GET"` is a configuration error.
A `type = "graphql"` query on the enclosing `query` block requires `method = "POST"` and `body_type = "graphql"`; setting either argument to a conflicting value is a configuration error.
The `body_graphql_variables` argument must be valid JSON when set.

`infinity.source` adds `headers` after the headers from the `client` block, and adds `params` to the request URL's existing query string.
A header name in both `headers` and `client`'s `http_headers` argument is a configuration error, and so is a parameter name in both `params` and the `url` argument's own query string.

### `client`

The `client` block configures the HTTP client `infinity.source` uses for every query with `source = "url"`.

{{< docs/shared lookup="reference/components/http-client-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

`infinity.source` sends {{< param "PRODUCT_NAME" >}}'s standard `User-Agent` header on every request.

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

| Name       | Type   | Description                                       | Default | Required |
| ----------- | ------ | ---------------------------------------------------- | ------- | -------- |
| `enabled`  | `bool` | Distribute query polling across cluster nodes.     |         | yes      |

When {{< param "PRODUCT_NAME" >}} is [using clustering][], and `enabled` is set to `true`, `infinity.source` shares ownership of its queries with its cluster peers.
A consistent hash of the component ID and the query name assigns each query to exactly one node.

If {{< param "PRODUCT_NAME" >}} isn't running in clustered mode, the `clustering` block is a no-op, and `infinity.source` polls every query on every node.

`infinity.source` checks ownership on every poll, so it doesn't implement `NotifyClusterChange`.
A change in ownership takes effect at the next poll, at most one `interval` after the cluster changes.

When a node loses ownership of a query, it clears that query's tracked series without sending stale markers, so the previous owner doesn't mark the new owner's series as stale.
Losing ownership also clears the query's health on that node, so the node's health no longer reflects a failure from before it lost ownership.

[using clustering]: ../../../../get-started/clustering/

### `forward_to`

The `forward_to` block configures the Alloy-native receivers for the results.

| Name       | Type                  | Description                                | Default | Required |
| ----------- | --------------------- | --------------------------------------------- | ------- | -------- |
| `logs`     | `list(LogsReceiver)`  | Loki components to send log entries to.     | `[]`    | no       |
| `metrics`  | `list(MetricsReceiver)` | Prometheus components to send samples to. | `[]`    | no       |

### `output`

The `output` block configures the OpenTelemetry consumers for the results.

| Name       | Type                     | Description                              | Default | Required |
| ----------- | ------------------------ | -------------------------------------------| ------- | -------- |
| `logs`     | `list(otelcol.Consumer)` | OpenTelemetry consumers to send logs to.  | `[]`    | no       |
| `metrics`  | `list(otelcol.Consumer)` | OpenTelemetry consumers to send metrics to.| `[]`    | no       |

Every query with `format = "table"` needs at least one receiver in `forward_to.metrics` or `output.metrics`.
Every query with `format = "logs"` needs at least one receiver in `forward_to.logs` or `output.logs`.
`infinity.source` fails to load its configuration when a query is missing the receiver its format needs.

## Metrics

`infinity.source` maps each row of a query's parsed frame to metrics as follows:

* Each string column with a non-nil value becomes a label. The label name is the column name, sanitized to a valid Prometheus label name.
* Each numeric column with a non-nil value becomes a gauge sample. The sample name is the `metrics` block's `prefix` argument plus the column name, sanitized to a valid Prometheus metric name.
* Each boolean column becomes a sample with a value of `1` or `0`.
* Time columns don't become samples or labels.
* CSV, TSV, XML, and HTML columns are strings unless a `column` block sets `type = "number"`.
  JSON string values that hold numbers, for example `"5"`, are also strings.
  An untyped string column becomes a label, so a query without a numeric column sends only `up`.
* Every sample from one poll shares the poll's start time.
* Every series has a `job` label set to the component's ID, and an `instance` label set to the query's label.
  A column named `job` or `instance` doesn't override these two labels; `infinity.source` logs a warning the first time this happens for a query.
* When two rows produce the same label set, `infinity.source` keeps the first row's sample and drops the rest.
  It counts the dropped samples in `infinity_source_duplicate_series_total` and logs a warning once for each poll.
* `up` is reserved for the synthetic sample described below. A column whose final metric name is `up`, after `prefix` and sanitizing, is dropped; `infinity.source` logs a warning the first time this happens for a query. Set `metrics.prefix` to avoid this collision.
* `up` is `1` after a successful poll and `0` after a failed poll.
* If a series from an earlier poll doesn't appear in the current poll, or if a poll fails, `infinity.source` sends a stale marker for that series.
* When {{< param "PRODUCT_NAME" >}} stops, or a configuration reload restarts a query's poll loop, `infinity.source` sends no stale markers for the poll in progress.
* When this node loses [clustering](#clustering) ownership of a query, `infinity.source` sends no stale markers for that query on this node; the series go stale by the metrics backend's lookback period instead. Losing ownership also clears the query's health on this node.

This example parses inline CSV data and sets `type = "number"` on the `count` column, so `count` becomes a sample and `name` becomes a label:

```alloy
infinity.source "inline_csv" {
  query "jobs" {
    type   = "csv"
    source = "inline"
    data   = "name,count\nbuild,3\ndeploy,5\n"

    column {
      selector = "name"
    }

    column {
      selector = "count"
      type     = "number"
    }
  }

  forward_to {
    metrics = [prometheus.remote_write.default.receiver]
  }
}
```

## Logs

`infinity.source` maps each row of a query's parsed frame to a log entry as follows:

* Timestamp: the first time column with a non-nil value in the row. If no time column has a value, `infinity.source` uses the poll's start time.
* Line: the value of the `logs` block's `line_column` column, converted to a string. If the frame has no column with that name, the line is a JSON object of every column in the row, in column order.
* Labels: `job`, `instance`, plus each column listed in `label_columns` that has a non-nil value in the row. `infinity.source` ignores a name in `label_columns` that isn't a column in the frame.
* Level: `infinity.source` sets the `level` label from a column named `severity` or `level`, unless `label_columns` already maps a column to a label named `level`. The `level` value also stays as an OpenTelemetry log attribute.
* Structured metadata: each column listed in `structured_metadata_columns` that has a non-nil value in the row.
* `job` and `instance` map to the OpenTelemetry resource attributes `service.name` and `service.instance.id`.
* If the entry count for a poll would exceed the `logs` block's `entry_limit`, the poll fails, and `infinity.source` sends no entries for it.

`infinity.source` doesn't deduplicate log entries across polls. Refer to [Limitations](#limitations).

## Limitations

`infinity.source` has the following limitations:

* `infinity.source` doesn't remove duplicate log entries across polls. If the API returns an overlapping time window, Loki receives the same entries again. Use API query parameters to request only new entries.
* Metrics show the current state only. `infinity.source` doesn't ingest historical points.
* Query parameters in `url` aren't secret. Put API keys in `url_options.params` or `url_options.headers`, which accept secrets.
* `infinity.source` doesn't support pagination, Azure Blob Storage, AWS SigV4, Google Sheets, or the UQL, GROQ, and simple parsers.
* `metrics.series_limit` and `logs.entry_limit` default to `0`, which means no limit. Set them for large or untrusted APIs.
* The parser libraries can't stop a `jq` or JSONata expression that doesn't end.
  After `timeout`, the poll fails, but the expression keeps using CPU until {{< param "PRODUCT_NAME" >}} restarts.
  A JSONata expression with unbounded recursion can crash {{< param "PRODUCT_NAME" >}} with a stack overflow, which Go can't recover from.
  Keep `root_selector` expressions simple.

## Translate a Grafana panel query

Use this table to translate a Grafana Infinity data source panel query that uses the `backend` or `jq-backend` parser into an `infinity.source` `query` block:

| Infinity query field                                   | `infinity.source` field                                              |
| --------------------------------------------------------| ------------------------------------------------------------------------ |
| `type`                                                  | `type`                                                                    |
| `parser`                                                | `parser`                                                                  |
| `format` (`table` or `logs` only)                       | `format`                                                                  |
| `source`                                                | `source`                                                                  |
| `url`                                                   | `url`                                                                     |
| `url_options.method`                                    | `url_options.method`                                                      |
| `url_options.params`, `url_options.headers` (key-value lists) | `url_options.params`, `url_options.headers` (maps)                  |
| `url_options.data`                                      | `url_options.body`                                                        |
| `root_selector`                                         | `root_selector`                                                           |
| `columns[]` (`selector`, `text`, `type`, `timestampFormat`) | `column` blocks (`selector`, `text`, `type`, `timestamp_format`)      |
| `computed_columns[]`                                     | `computed_column` blocks                                                  |
| `filterExpression`                                      | `filter_expression`                                                       |
| `summarizeExpression`, `summarizeBy`, `summarizeAlias`   | `summarize_expression`, `summarize_by`, `summarize_alias`                |
| the panel's transformations query                        | `transform.*` blocks inside each `query` block                          |

A panel's `transformations` query type in Grafana runs over the frames of every other query in the panel.
`infinity.source` has no equivalent shared query; copy the transformations query's steps into the `transform` blocks of each `query` block that needs them.

## Exported fields

`infinity.source` doesn't export any fields.

## Component health

`infinity.source` reports as healthy when every query succeeded on its last poll, or hasn't polled yet.

`infinity.source` reports as unhealthy when at least one query failed on its last poll.
The health message names the first failed query in label order, its failure reason, and how many other queries also failed, for example:

```text
query "orders" failed: status 503 (and 2 other queries)
```

Health messages never show a URL's query string; `infinity.source` replaces every query parameter value with `REDACTED`.

## Debug information

`infinity.source` doesn't expose any component-specific debug information.

## Debug metrics

The following Prometheus metrics are exposed:

| Name                                        | Type        | Description                                                          |
| --------------------------------------------- | ----------- | ------------------------------------------------------------------------ |
| `infinity_source_poll_duration_seconds`     | `histogram` | Duration of one poll of a query.                                     |
| `infinity_source_poll_failures_total`       | `counter`   | Total number of failed polls, by `reason`.                           |
| `infinity_source_polls_overrun_total`       | `counter`   | Total number of polls that took longer than `interval`.              |
| `infinity_source_samples_sent_total`        | `counter`   | Total number of samples sent, including `up`.                        |
| `infinity_source_entries_sent_total`        | `counter`   | Total number of log entries sent.                                    |
| `infinity_source_duplicate_series_total`    | `counter`   | Total number of samples dropped because an earlier row had the same labels. |

Every metric carries a `query` label with the query's label.
`infinity.source` deletes a query's label values from these metrics when you remove the query from the configuration.
`infinity_source_poll_failures_total` also carries a `reason` label, one of `request`, `timeout`, `status`, `too_large`, `parse`, `postprocess`, `series_limit`, `entry_limit`, or `emit`.

## Example

This example polls the GitHub API for the core rate limit and sends it as metrics, and polls the Grafana organization's public events feed and sends it as logs:

```alloy
infinity.source "github" {
  interval = "5m"

  client {
    bearer_token = sys.env("GITHUB_TOKEN")
  }

  query "rate_limit" {
    url           = "https://api.github.com/rate_limit"
    root_selector = "resources.core"
  }

  query "events" {
    url    = "https://api.github.com/orgs/grafana/events"
    format = "logs"

    column {
      selector = "type"
    }

    column {
      selector = "actor.login"
      text     = "actor"
    }

    column {
      selector = "repo.name"
      text     = "repo"
    }

    column {
      selector         = "created_at"
      type             = "timestamp"
      timestamp_format = "2006-01-02T15:04:05Z"
    }

    logs {
      label_columns               = ["type"]
      structured_metadata_columns = ["actor", "repo"]
    }
  }

  forward_to {
    metrics = [prometheus.remote_write.default.receiver]
    logs    = [loki.write.default.receiver]
  }
}

prometheus.remote_write "default" {
  endpoint {
    url = sys.env("PROMETHEUS_URL")
  }
}

loki.write "default" {
  endpoint {
    url = sys.env("LOKI_URL")
  }
}
```

Replace the following:

* `GITHUB_TOKEN`: A GitHub personal access token with permission to read the organization's events.
* `PROMETHEUS_URL`: The URL of the Prometheus remote-write-compatible server to send metrics to.
* `LOKI_URL`: The URL of the Loki server to send logs to.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`infinity.source` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)
- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)
- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
