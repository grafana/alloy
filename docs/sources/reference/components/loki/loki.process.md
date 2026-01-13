---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.process/
aliases:
  - ../loki.process/ # /docs/alloy/latest/reference/components/loki.process/
description: Learn about loki.process
labels:
  stage: general-availability
  products:
    - oss
title: loki.process
---

# `loki.process`

`loki.process` receives log entries from other loki components, applies one or more processing _stages_, and forwards the results to the list of receivers in the component's arguments.

A stage is a multi-purpose tool that can parse, transform, and filter log entries before they're passed to a downstream component.
These stages are applied to each log entry in order of their appearance in the configuration file.
All stages within a `loki.process` block have access to the log entry's label set, the log line, the log timestamp, as well as a shared map of 'extracted' values so that the results of one stage can be used in a subsequent one.

You can specify multiple `loki.process` components by giving them different labels.

## Usage

```alloy
loki.process "<LABEL>" {
  forward_to = <RECEIVER_LIST>

  stage.<STAGENAME> {
    ...
  }
  ...
}
```

## Arguments

You can use the following argument with `loki.process`:

| Name         | Type                 | Description                                    | Default | Required |
| ------------ | -------------------- | ---------------------------------------------- | ------- | -------- |
| `forward_to` | `list(LogsReceiver)` | Where to forward log entries after processing. |         | yes      |

## Blocks

You can use the following blocks with `loki.process`:

| Block                                                              | Description                                                    | Required |
|--------------------------------------------------------------------|----------------------------------------------------------------|----------|
| [`stage.cri`][stage.cri]                                           | Configures a pre-defined CRI-format pipeline.                  | no       |
| [`stage.decolorize`][stage.decolorize]                             | Strips ANSI color codes from log lines.                        | no       |
| [`stage.docker`][stage.docker]                                     | Configures a pre-defined Docker log format pipeline.           | no       |
| [`stage.drop`][stage.drop]                                         | Configures a `drop` processing stage.                          | no       |
| [`stage.eventlogmessage`][stage.eventlogmessage]                   | Extracts data from the Message field in the Windows Event Log. | no       |
| [`stage.geoip`][stage.geoip]                                       | Configures a `geoip` processing stage.                         | no       |
| [`stage.json`][stage.json]                                         | Configures a JSON processing stage.                            | no       |
| [`stage.label_drop`][stage.label_drop]                             | Configures a `label_drop` processing stage.                    | no       |
| [`stage.label_keep`][stage.label_keep]                             | Configures a `label_keep` processing stage.                    | no       |
| [`stage.labels`][stage.labels]                                     | Configures a `labels` processing stage.                        | no       |
| [`stage.limit`][stage.limit]                                       | Configures a `limit` processing stage.                         | no       |
| [`stage.logfmt`][stage.logfmt]                                     | Configures a `logfmt` processing stage.                        | no       |
| [`stage.luhn`][stage.luhn]                                         | Configures a `luhn` processing stage.                          | no       |
| [`stage.match`][stage.match]                                       | Configures a `match` processing stage.                         | no       |
| [`stage.metrics`][stage.metrics]                                   | Configures a `metrics` stage.                                  | no       |
| [`stage.multiline`][stage.multiline]                               | Configures a `multiline` processing stage.                     | no       |
| [`stage.output`][stage.output]                                     | Configures an `output` processing stage.                       | no       |
| [`stage.pack`][stage.pack]                                         | Configures a `pack` processing stage.                          | no       |
| [`stage.pattern`][stage.pattern]                                   | Configures a `pattern` processing stage.                       | no       |
| [`stage.regex`][stage.regex]                                       | Configures a `regex` processing stage.                         | no       |
| [`stage.replace`][stage.replace]                                   | Configures a `replace` processing stage.                       | no       |
| [`stage.sampling`][stage.sampling]                                 | Configures a `sampling` processing stage.                      | no       |
| [`stage.static_labels`][stage.static_labels]                       | Configures a `static_labels` processing stage.                 | no       |
| [`stage.structured_metadata`][stage.structured_metadata]           | Configures a structured metadata processing stage.             | no       |
| [`stage.structured_metadata_drop`][stage.structured_metadata_drop] | Configures a `structured_metadata_drop` processing stage.      | no       |
| [`stage.template`][stage.template]                                 | Configures a `template` processing stage.                      | no       |
| [`stage.tenant`][stage.tenant]                                     | Configures a `tenant` processing stage.                        | no       |
| [`stage.timestamp`][stage.timestamp]                               | Configures a `timestamp` processing stage.                     | no       |
| [`stage.truncate`][stage.truncate]                                 | Configures a `truncate` processing stage.                      | no       |
| [`stage.windowsevent`][stage.windowsevent]                         | Configures a `windowsevent` processing stage.                  | no       |

You can provide any number of these stage blocks nested inside `loki.process`. These blocks run in order of appearance in the configuration file.

[stage.cri]: #stagecri
[stage.decolorize]: #stagedecolorize
[stage.docker]: #stagedocker
[stage.drop]: #stagedrop
[stage.eventlogmessage]: #stageeventlogmessage
[stage.geoip]: #stagegeoip
[stage.json]: #stagejson
[stage.label_drop]: #stagelabel_drop
[stage.label_keep]: #stagelabel_keep
[stage.labels]: #stagelabels
[stage.limit]: #stagelimit
[stage.logfmt]: #stagelogfmt
[stage.luhn]: #stageluhn
[stage.match]: #stagematch
[stage.metrics]: #stagemetrics
[stage.multiline]: #stagemultiline
[stage.output]: #stageoutput
[stage.pack]: #stagepack
[stage.pattern]: #stagepattern
[stage.regex]: #stageregex
[stage.replace]: #stagereplace
[stage.sampling]: #stagesampling
[stage.static_labels]: #stagestatic_labels
[stage.structured_metadata]: #stagestructured_metadata
[stage.structured_metadata_drop]: #stagestructured_metadata_drop
[stage.template]: #stagetemplate
[stage.tenant]: #stagetenant
[stage.truncate]: #stagetruncate
[stage.timestamp]: #stagetimestamp
[stage.windowsevent]: #stagewindowsevent

### `stage.cri`

The `stage.cri` inner block enables a predefined pipeline which reads log lines using the CRI logging format.

The following arguments are supported:

| Name                             | Type     | Description                                                          | Default | Required |
| -------------------------------- | -------- | -------------------------------------------------------------------- | ------- | -------- |
| `max_partial_line_size_truncate` | `bool`   | Truncate partial lines that are longer than `max_partial_line_size`. | `false` | no       |
| `max_partial_line_size`          | `number` | Maximum number of characters which a partial line can have.          | `0`     | no       |
| `max_partial_lines`              | `number` | Maximum number of partial lines to hold in memory.                   | `100`   | no       |

`max_partial_line_size` is only taken into account if `max_partial_line_size_truncate` is set to `true`.

```alloy
stage.cri {}
```

CRI specifies log lines as single space-delimited values with the following components:

* `time`: The timestamp string of the log.
* `stream`: Either `stdout` or `stderr`.
* `flags`: CRI flags including `F` or `P`.
* `log`: The contents of the log line.

Given the following log line, the subsequent key-value pairs are created in the shared map of extracted data:

```text
"2019-04-30T02:12:41.8443515Z stdout F message"

content: message
stream: stdout
timestamp: 2019-04-30T02:12:41.8443515
```

### `stage.decolorize`

The `stage.decolorize` strips ANSI color codes from the log lines, making it easier to parse logs.

The `stage.decolorize` block doesn't support any arguments or inner blocks, so it's always empty.

```alloy
stage.decolorize {}
```

`stage.decolorize` turns each line having a color code into a non-colored one.
For example:

```text
[2022-11-04 22:17:57.811] \033[0;32http\033[0m: GET /_health (0 ms) 204
```

is turned into

```text
[2022-11-04 22:17:57.811] http: GET /_health (0 ms) 204
```

### `stage.docker`

The `stage.docker` inner block enables a predefined pipeline which reads log lines in the standard format of Docker log files.

The `stage.docker` block doesn't support any arguments or inner blocks, so it's always empty.

```alloy
stage.docker {}
```

Docker log entries are formatted as JSON with the following keys:

* `log`: The content of log line.
* `stream`: Either `stdout` or `stderr`.
* `time`: The timestamp string of the log line.

Given the following log line, the subsequent key-value pairs are created in the shared map of extracted data:

```text
{"log":"log message\n","stream":"stderr","time":"2019-04-30T02:12:41.8443515Z"}

output: log message\n
stream: stderr
timestamp: 2019-04-30T02:12:41.8443515
```

### `stage.drop`

The `stage.drop` inner block configures a filtering stage that drops log entries based on several options.
If multiple options are provided, they're treated as AND clauses and must _all_ be true for the log entry to be dropped.
To drop entries with an OR clause, specify multiple `drop` blocks in sequence.

The following arguments are supported:

| Name                  | Type       | Description                                                                                                            | Default        | Required |
| --------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------------- | -------------- | -------- |
| `drop_counter_reason` | `string`   | A custom reason to report for dropped lines.                                                                           | `"drop_stage"` | no       |
| `expression`          | `string`   | A valid RE2 regular expression.                                                                                        | `""`           | no       |
| `longer_than`         | `string`   | If specified, the stage drops lines whose size exceeds the configured value.                                           | `""`           | no       |
| `older_than`          | `duration` | If specified, the stage drops lines whose timestamp is older than the current time minus this duration.                | `""`           | no       |
| `separator`           | `string`   | When `source` is a comma-separated list of names, this separator is placed between concatenated extracted data values. | `";"`          | no       |
| `source`              | `string`   | Name or comma-separated list of names from extracted data to match. If empty or not defined, it uses the log message.  | `""`           | no       |
| `value`               | `string`   | If both `source` and `value` are specified, the stage drops lines where `value` exactly matches the source content.    | `""`           | no       |

The `expression` field must be a RE2 regular expression string.

* If `source` is empty or not provided, the regular expression attempts to match the log line itself.
* If `source` is a single name, the regular expression attempts to match the corresponding value from the extracted map.
* If `source` is a comma-separated list of names, the corresponding values from the extracted map are concatenated using `separator` and the regular expression attempts to match the concatenated string.

The `value` field can only work with values from the extracted map, and must be specified together with `source`.

* If `source` is a single name, the entries are dropped when there is an exact match between the corresponding value from the extracted map and the `value`.
* If `source` is a comma-separated list of names, the entries are dropped when the `value` matches the `source` values from extracted data, concatenated using the `separator`.

Whenever an entry is dropped, the metric `loki_process_dropped_lines_total` is incremented.
By default, the reason label is `"drop_stage"`, but you can provide a custom label using the `drop_counter_reason` argument.

The following stage drops log entries that contain the word `debug` _and_ are longer than 1KB.

```alloy
stage.drop {
    expression  = ".*debug.*"
    longer_than = "1KB"
}
```

The following example defines multiple `drop` blocks so `loki.process` drops entries that are either 24h or older, are longer than 8KB, _or_ the extracted value of `app` is equal to `example`.

```alloy
stage.drop {
    older_than          = "24h"
    drop_counter_reason = "too old"
}

stage.drop {
    longer_than         = "8KB"
    drop_counter_reason = "too long"
}

stage.drop {
    source = "app"
    value  = "example"
}
```

### `stage.eventlogmessage`

Deprecated in favor of the [`stage.windowsevent`][stage.windowsevent] block.

The `eventlogmessage` stage extracts data from the Message string that appears in the Windows Event Log.

The following arguments are supported:

| Name                  | Type     | Description                                           | Default     | Required |
| --------------------- | -------- | ----------------------------------------------------- | ----------- | -------- |
| `drop_invalid_labels` | `bool`   | Whether to drop fields that aren't valid label names. | `false`     | no       |
| `overwrite_existing`  | `bool`   | Whether to overwrite existing extracted data fields.  | `false`     | no       |
| `source`              | `string` | Name of the field in the extracted data to parse.     | `"message"` | no       |

When `drop_invalid_labels` is set to `true`, the stage drops fields that aren't valid label names.
If set to `false`, the stage automatically converts them into valid labels replacing invalid characters with underscores.

When `overwrite_existing` is set to `true`, the stage overwrites existing extracted data fields with the same name.
If set to `false`, the `_extracted` suffix is appended to an existing field name.

The `eventlogmessage` stage only extracts lines with the key:value format.
All non-alpha characters in the key are replaced with underscores.
For example, `\tSecurity ID` is extracted as `_Security_ID`.

#### Example combined with `stage.json`

```alloy
stage.json {
    expressions = {
        message = "",
        Overwritten = "",
    }
}

stage.eventlogmessage {
    source = "message"
    overwrite_existing = true
}
```

Given the following log line:

```text
{"event_id": 1, "Overwritten": "old", "message": "Message type:\r\nOverwritten: new\r\nImage: C:\\Users\\User\\alloy.exe"}
```

The first stage would create the following key-value pairs in the set of extracted data:

* `message`: `Message type:\r\nOverwritten: new\r\nImage: C:\Users\User\alloy.exe`
* `Overwritten`: `old`

The second stage parses the value of `message` from the extracted data and appends or overwrites the following key-value pairs to the set of extracted data:

* `Image`: `C:\\Users\\User\\alloy.exe`
* `Message_type`: (empty string)
* `Overwritten`: `new`

### `stage.geoip`

The `stage.geoip` inner block configures a processing stage that reads an IP address and populates the shared map with `geoip` fields. The Maxmind GeoIP2 database is used for the lookup.

The following arguments are supported:

| Name             | Type          | Description                                                         | Default | Required |
| ---------------- | ------------- | ------------------------------------------------------------------- | ------- | -------- |
| `db`             | `string`      | Path to the Maxmind DB file.                                        |         | yes      |
| `source`         | `string`      | IP from extracted data to parse.                                    |         | yes      |
| `custom_lookups` | `map(string)` | Key-value pairs of JMESPath expressions.                            |         | no       |
| `db_type`        | `string`      | Maxmind DB type. Allowed values are `"city"`, `"asn"`, `"country"`. |         | no       |

#### GeoIP with City database example

```alloy
{"log":"log message","client_ip":"34.120.177.193"}

loki.process "example" {
    stage.json {
        expressions = {ip = "client_ip"}
    }

    stage.geoip {
        source  = "ip"
        db      = "/path/to/db/GeoLite2-City.mmdb"
        db_type = "city"
    }

    stage.labels {
        values = {
            geoip_city_name          = "",
            geoip_country_name       = "",
            geoip_country_code       = "",
            geoip_continent_name     = "",
            geoip_continent_code     = "",
            geoip_location_latitude  = "",
            geoip_location_longitude = "",
            geoip_postal_code        = "",
            geoip_timezone           = "",
            geoip_subdivision_name   = "",
            geoip_subdivision_code   = "",
        }
    }
}
```

The `json` stage extracts the IP address from the `client_ip` key in the log line.
Then the extracted `ip` value is given as source to `geoip` stage.
The `geoip` stage performs a lookup on the IP and populates the following fields in the shared map which are added as labels using the `labels` stage.

The extracted data from the IP used in this example:

```text
geoip_city_name: Kansas City
geoip_country_name: United States
geoip_country_code: US
geoip_continent_name: North America
geoip_continent_code: NA
geoip_location_latitude: 39.1027
geoip_location_longitude: -94.5778
geoip_postal_code: 64184
geoip_timezone: America/Chicago
geoip_subdivision_name: Missouri
geoip_subdivision_code: MO
```

#### GeoIP with ASN (Autonomous System Number) database example

```alloy
loki.process "example" {
    stage.json {
        expressions = {ip = "client_ip"}
    }

    stage.geoip {
        source  = "ip"
        db      = "/path/to/db/GeoIP2-ASN.mmdb"
        db_type = "asn"
    }

    stage.labels {
        values = {
            geoip_autonomous_system_number       = "",
            geoip_autonomous_system_organization = "",
        }
    }
}
```

The `json` stage extracts the IP address from the `client_ip` key in the log line.
Then the extracted `ip` value is given as source to `geoip` stage. The `geoip` stage performs a lookup on the IP and populates the shared map.

The extracted data from the IP used in this example:

```text
geoip_autonomous_system_number: 396982
geoip_autonomous_system_organization: GOOGLE-CLOUD-PLATFORM
```

#### GeoIP with Country database example

```alloy
{"log":"log message","client_ip":"34.120.177.193"}

loki.process "example" {
    stage.json {
        expressions = {ip = "client_ip"}
    }

    stage.geoip {
        source  = "ip"
        db      = "/path/to/db/GeoLite2-Country.mmdb"
        db_type = "country"
    }

    stage.labels {
        values = {
            geoip_country_name       = "",
            geoip_country_code       = "",
            geoip_continent_name     = "",
            geoip_continent_code     = "",
        }
    }
}
```

The `json` stage extracts the IP address from the `client_ip` key in the log line.
Then the extracted `ip` value is given as source to `geoip` stage.
The `geoip` stage performs a lookup on the IP and populates the following fields in the shared map which are added as labels using the `labels` stage.

The extracted data from the IP used in this example:

```text
geoip_country_name: United States
geoip_country_code: US
geoip_continent_name: North America
geoip_continent_code: NA
```

#### GeoIP with custom fields example

If the MMDB file used is enriched with custom data, for example, private IP addresses as explained in [the Maxmind blog post](https://github.com/maxmind/mmdb-from-go-blogpost), then it can be extracted from the record using the `custom_lookups` attribute.

```alloy
loki.process "example" {
    stage.json {
        expressions = {ip = "client_ip"}
    }

    stage.geoip {
        source         = "ip"
        db             = "/path/to/db/GeoIP2-Enriched.mmdb"
        db_type        = "city"
        custom_lookups = {
            "department"  = "MyCompany.DeptName",
            "parent_vnet" = "MyCompany.ParentVNet",
            "subnet"      = "MyCompany.Subnet",
        }
    }

    stage.labels {
        values = {
            department  = "",
            parent_vnet = "",
            subnet      = "",
        }
    }
}
```

The `json` stage extracts the IP address from the `client_ip` key in the log line.
Then the extracted `ip` value is given as source to `geoip` stage. The `geoip` stage performs a lookup on the IP and populates the shared map with the data from the city database results in addition to the custom lookups. Lastly, the custom lookup fields from the shared map are added as labels.

### `stage.json`

The `stage.json` inner block configures a JSON processing stage that parses incoming log lines or previously extracted values as JSON and uses [JMESPath expressions][] to extract new values from them.

[JMESPath expressions]: https://jmespath.site/main/#tutorial

The following arguments are supported:

| Name             | Type          | Description                                           | Default | Required |
| ---------------- | ------------- | ----------------------------------------------------- | ------- | -------- |
| `expressions`    | `map(string)` | Key-value pairs of JMESPath expressions.              |         | yes      |
| `drop_malformed` | `bool`        | Drop lines whose input can't be parsed as valid JSON. | `false` | no       |
| `source`         | `string`      | Source of the data to parse as JSON.                  | `""`    | no       |

The `expressions` field is the set of key-value pairs of JMESPath expressions to run.
The map key defines the name with which the data is extracted, while the map value is the expression used to populate the value.

When configuring a JSON stage, the `source` field defines the source of data to parse as JSON.
By default, this is the log line itself, but it can also be a previously extracted value.

The following example shows a given log line and two JSON stages.

```alloy
{"log":"log message\n","extra":"{\"user\":\"alloy\"}"}

loki.process "username" {
  stage.json {
      expressions = {output = "log", extra = ""}
  }

  stage.json {
      source      = "extra"
      expressions = {username = "user"}
  }
}
```

In this example, the first stage uses the log line as the source and populates these values in the shared map.
An empty expression means using the same value as the key, for example `extra="extra"`.

```text
output: log message\n
extra: {"user": "alloy"}
```

The second stage uses the value in `extra` as the input and appends the following key-value pair to the set of extracted data.

```text
username: alloy
```

{{< admonition type="note" >}}
Due to a limitation of the upstream `jmespath` library, you must wrap any string that contains a hyphen `-` in quotes so that it's not considered a numerical expression.

If you don't use quotes to wrap a string that contains a hyphen, you will get errors like: `Unexpected token at the end of the expression: tNumber`

You can use one of two options to circumvent this issue:

1. An escaped double quote. For example: `http_user_agent = "\"request_User-Agent\""`
1. A backtick quote. For example: ``http_user_agent = `"request_User-Agent"` ``
{{< /admonition >}}

### `stage.label_drop`

The `stage.label_drop` inner block configures a processing stage that drops labels from incoming log entries.

The following arguments are supported:

| Name     | Type           | Description                                 | Default | Required |
| -------- | -------------- | ------------------------------------------- | ------- | -------- |
| `values` | `list(string)` | Configures a `label_drop` processing stage. | `{}`    | no       |

```alloy
stage.label_drop {
    values = [ "kubernetes_node_name", "kubernetes_namespace" ]
}
```

### `stage.label_keep`

The `stage.label_keep` inner block configures a processing stage that filters the label set of an incoming log entry down to a subset.

The following arguments are supported:

| Name     | Type           | Description                                 | Default | Required |
| -------- | -------------- | ------------------------------------------- | ------- | -------- |
| `values` | `list(string)` | Configures a `label_keep` processing stage. | `{}`    | no       |

```alloy
stage.label_keep {
    values = [ "kubernetes_pod_name", "kubernetes_pod_container_name" ]
}
```

### `stage.labels`

The `stage.labels` inner block configures a labels processing stage that can read data from the extracted values map and set new labels on incoming log entries.

For labels that are static, refer to [`stage.static_labels`][stage.static_labels]

The following arguments are supported:

| Name     | Type          | Description                             | Default | Required |
| -------- | ------------- | --------------------------------------- | ------- | -------- |
| `values` | `map(string)` | Configures a `labels` processing stage. | `{}`    | no       |

In a labels stage, the map's keys define the label to set and the values are how to look them up.
If the value is empty, it's inferred to be the same as the key.

```alloy
stage.labels {
    values = {
      env  = "",         // Sets up an 'env' label, based on the 'env' extracted value.
      user = "username", // Sets up a 'user' label, based on the 'username' extracted value.
    }
}
```

### `stage.limit`

The `stage.limit` inner block configures a rate-limiting stage that throttles logs based on several options.

The following arguments are supported:

| Name                  | Type     | Description                                                                      | Default | Required |
| --------------------- | -------- | -------------------------------------------------------------------------------- | ------- | -------- |
| `burst`               | `number` | The maximum number of burst lines that the stage forwards.                       |         | yes      |
| `rate`                | `number` | The maximum rate of lines per second that the stage forwards.                    |         | yes      |
| `by_label_name`       | `string` | The label to use when rate-limiting on a label name.                             | `""`    | no       |
| `drop`                | `bool`   | Whether to discard or backpressure lines that exceed the rate limit.             | `false` | no       |
| `max_distinct_labels` | `number` | The number of unique values to keep track of when rate-limiting `by_label_name`. | `10000` | no       |

The rate limiting is a "token bucket" of size `burst`.
It's initially full and refilled at `rate` tokens per second.
Each received log entry consumes one token from the bucket.
When `drop` is set to true, incoming entries that exceed the rate-limit are dropped, otherwise they're queued until more tokens are available.

```alloy
stage.limit {
    rate  = 5
    burst = 10
}
```

If `by_label_name` is set, then `drop` must be set to `true`.
This enables the stage to rate-limit not by the number of lines but by the number of labels.

The following example rate-limits entries from each unique `namespace` value independently.
Any entries without the `namespace` label aren't rate-limited.
The stage keeps track of up to `max_distinct_labels` unique values, defaulting at 10000.

```alloy
stage.limit {
    rate  = 10
    burst = 10
    drop  = true

    by_label_name = "namespace"
}
```

### `stage.logfmt`

The `stage.logfmt` inner block configures a processing stage that reads incoming log lines as logfmt and extracts values from them.

The following arguments are supported:

| Name      | Type          | Description                                    | Default | Required |
| --------- | ------------- | ---------------------------------------------- | ------- | -------- |
| `mapping` | `map(string)` | Key-value pairs of `logmft` fields to extract. |         | yes      |
| `source`  | `string`      | Source of the data to parse as `logfmt`.       | `""`    | no       |

The `source` field defines the source of data to parse as `logfmt`.
When `source` is missing or empty, the stage parses the log line itself, but it can also be used to parse a previously extracted value.

This stage uses the [go-logfmt][] unmarshaler, so that numeric or boolean types are unmarshalled into their correct form.
The stage doesn't perform any other type conversions.
If the extracted value is a complex type, it's treated as a string.

[go-logfmt]: https://github.com/go-logfmt/logfmt

The following log line and stages demonstrates how this works.

```alloy
time=2012-11-01T22:08:41+00:00 app=loki level=WARN duration=125 message="this is a log line" extra="user=example_name"

stage.logfmt {
    mapping = { "extra" = "" }
}

stage.logfmt {
    mapping = { "username" = "user" }
    source  = "extra"
}
```

The first stage parses the log line itself and inserts the `extra` key in the set of extracted data, with the value of `user=example_name`.

The second stage parses the contents of `extra` and appends the `username: example_name` key-value pair to the set of extracted data.

### `stage.luhn`

The `stage.luhn` inner block configures a processing stage that reads incoming log lines and redacts strings that match a Luhn algorithm.

The [Luhn algorithm][] is a simple checksum formula used to validate various identification numbers, such as credit card numbers, IMEI numbers, National Provider Identifier numbers in the USA, and Canadian Social Insurance Numbers.
Many Payment Card Industry environments require these numbers to be redacted.

[Luhn algorithm]: https://en.wikipedia.org/wiki/Luhn_algorithm

The following arguments are supported:

| Name          | Type     | Description                                                    | Default          | Required |
| ------------- | -------- | -------------------------------------------------------------- | ---------------- | -------- |
| `delimiters`  | `string` | A list containing delimiters to accept as part of the number.  | `""`             | no       |
| `min_length`  | `int`    | Minimum length of digits to consider                           | `13`             | no       |
| `replacement` | `string` | String to substitute the matched patterns with.                | `"**REDACTED**"` | no       |
| `source`      | `string` | Source of the data to parse.                                   | `""`             | no       |

The `source` field defines the source of data to search.
When `source` is missing or empty, the stage parses the log line itself, but it can also be used to parse a previously extracted value.

If you want the Luhn algorithm to identify numbers with delimiters, for example `4032-0325-1354-8443`, you can configure the `delimiters` field with the expected delimiters.

#### Example

The following example log line contains an approved credit card number.

```alloy
time=2012-11-01T22:08:41+00:00 app=loki level=WARN duration=125 message="credit card approved 4032032513548443" extra="user=example_name"

stage.luhn {
    replacement = "**DELETED**"
}
```

The stage parses the log line, redacts the credit card number, and produces the following updated log line:

```text
time=2012-11-01T22:08:41+00:00 app=loki level=INFO duration=125 message="credit card approved **DELETED**" extra="user=example_name"
```

#### Example with `delimiters`

The following example log line contains an approved credit card number, represented with dash characters between each group of four digits.

```alloy
time=2012-11-01T22:08:41+00:00 app=loki level=WARN duration=125 message="credit card approved 4032-0325-1354-8443" extra="user=example_name"

stage.luhn {
    replacement = "**DELETED**"
    delimiters = "-"
}
```

The stage parses the log line, redacts the credit card number, and produces the following updated log line:

```text
time=2012-11-01T22:08:41+00:00 app=loki level=INFO duration=125 message="credit card approved **DELETED**" extra="user=example_name"
```

### `stage.match`

The `stage.match` inner block configures a filtering stage that can conditionally either apply a nested set of processing stages or drop an entry when a log entry matches a configurable LogQL stream selector and filter expressions.

The following arguments are supported:

| Name                  | Type     | Description                                                                                           | Default         | Required |
| --------------------- | -------- | ----------------------------------------------------------------------------------------------------- | --------------- | -------- |
| `selector`            | `string` | The LogQL stream selector and line filter expressions to use.                                         |                 | yes      |
| `action`              | `string` | The action to take when the selector matches the log line. Supported values are `"keep"` and `"drop"` | `"keep"`        | no       |
| `drop_counter_reason` | `string` | A custom reason to report for dropped lines.                                                          | `"match_stage"` | no       |
| `pipeline_name`       | `string` | A custom name to use for the nested pipeline.                                                         | `""`            | no       |

{{< admonition type="note" >}}
The filters don't include label filter expressions such as `| label == "examplelabel"`.
{{< /admonition >}}

The `stage.match` block supports a number of `stage.*` inner blocks, like the top-level block.
These are used to construct the nested set of stages to run if the selector matches the labels and content of the log entries.
It supports all the same `stage.NAME` blocks as the in the top level of the `loki.process` component.

If the specified action is `"drop"`, the metric `loki_process_dropped_lines_total` is incremented with every line dropped.
By default, the reason label is `"match_stage"`, but a custom reason can be provided by using the `drop_counter_reason` argument.

The following example log lines and stages show how this works.

```alloy
{ "time":"2023-01-18T17:08:41+00:00", "app":"example1", "component": ["parser","type"], "level" : "WARN", "message" : "app1 log line" }
{ "time":"2023-01-18T17:08:42+00:00", "app":"example2", "component": ["parser","type"], "level" : "ERROR", "message" : "example noisy error" }

stage.json {
    expressions = { "appname" = "app" }
}

stage.labels {
    values = { "applbl" = "appname" }
}

stage.match {
    selector = "{applbl=\"example1\"}"

    stage.json {
        expressions = { "msg" = "message" }
    }
}

stage.match {
    selector = "{applbl=\"qux\"}"
    stage.json {
        expressions = { "msg" = "msg" }
    }
}

stage.match {
    selector = "{applbl=\"example2\"} |~ \".*noisy error.*\""
    action   = "drop"

    drop_counter_reason = "discard_noisy_errors"
}

stage.output {
    source = "msg"
}
```

The first two stages parse the log lines as JSON, decode the `app` value into the shared extracted map as `appname`, and use its value as the `applbl` label.

The third stage uses the LogQL selector to only execute the nested stages on lines where the `applbl="example1"`.
So, for the first line, the nested JSON stage adds `msg="app1 log line"` into the extracted map.

The fourth stage uses the LogQL selector to only execute on lines where `applbl="qux"`. This means it won't match any of the input, and the nested JSON stage doesn't run.

The fifth stage drops entries from lines where `applbl` is set to 'example2' and the line contents matches the regex `.*noisy error.*`.
It also increments the `loki_process_dropped_lines_total` metric with a label `drop_counter_reason="discard_noisy_errors"`.

The final output stage changes the contents of the log line to be the value of `msg` from the extracted map. In this case, the first log entry's content is changed to `app1 log line`.

### `stage.metrics`

The `stage.metrics` inner block configures stage that allows you to define and update metrics based on values from the shared extracted map.
The created metrics are available at the {{< param "PRODUCT_NAME" >}} root `/metrics` endpoint.

The `stage.metrics` block doesn't support any arguments and is only configured via a number of nested inner `metric.*` blocks, one for each metric that should be generated.

The following blocks are supported inside the definition of `stage.metrics`:

| Block                                  | Description                   | Required |
| -------------------------------------- | ----------------------------- | -------- |
| [`metric.counter`][metric.counter]     | Defines a `counter` metric.   | no       |
| [`metric.gauge`][metric.gauge]         | Defines a `gauge` metric.     | no       |
| [`metric.histogram`][metric.histogram] | Defines a `histogram` metric. | no       |

{{< admonition type="note" >}}
The metrics will be reset if you reload the {{< param "PRODUCT_NAME" >}} configuration file.
{{< /admonition >}}

[metric.counter]: #metriccounter
[metric.gauge]: #metricgauge
[metric.histogram]: #metrichistogram

#### `metric.counter`

Defines a metric whose value only goes up.

The following arguments are supported:

| Name                | Type       | Description                                                                                               | Default                  | Required |
| ------------------- | ---------- | --------------------------------------------------------------------------------------------------------- | ------------------------ | -------- |
| `action`            | `string`   | The action to take. Valid actions are `inc` and `add`.                                                    |                          | yes      |
| `name`              | `string`   | The metric name.                                                                                          |                          | yes      |
| `count_entry_bytes` | `bool`     | If set to true, counts all log lines bytes.                                                               | `false`                  | no       |
| `description`       | `string`   | The metric's description and help text.                                                                   | `""`                     | no       |
| `match_all`         | `bool`     | If set to true, all log lines are counted, without attempting to match the `source` to the extracted map. | `false`                  | no       |
| `max_idle_duration` | `duration` | Maximum amount of time to wait until the metric is marked as 'stale' and removed.                         | `"5m"`                   | no       |
| `prefix`            | `string`   | The prefix to the metric name.                                                                            | `"loki_process_custom_"` | no       |
| `source`            | `string`   | Key from the extracted data map to use for the metric. Defaults to the metric name.                       | `""`                     | no       |
| `value`             | `string`   | If set, the metric only changes if `source` exactly matches the `value`.                                  | `""`                     | no       |

A counter can't set both `match_all` to true _and_ a `value`.
A counter can't set `count_entry_bytes` without also setting `match_all=true` _or_ `action=add`.
The valid `action` values are `inc` and `add`.
The `inc` action increases the metric value by 1 for each log line that passed the filter.
The `add` action converts the extracted value to a positive float and adds it to the metric.

#### `metric.gauge`

Defines a gauge metric whose value can go up or down.

The following arguments are supported:

| Name                | Type       | Description                                                                         | Default                  | Required |
| ------------------- | ---------- | ----------------------------------------------------------------------------------- | ------------------------ | -------- |
| `action`            | `string`   | The action to take. Valid actions are  `inc`, `dec`, `set`, `add`, or `sub`.        |                          | yes      |
| `name`              | `string`   | The metric name.                                                                    |                          | yes      |
| `description`       | `string`   | The metric's description and help text.                                             | `""`                     | no       |
| `max_idle_duration` | `duration` | Maximum amount of time to wait until the metric is marked as 'stale' and removed.   | `"5m"`                   | no       |
| `prefix`            | `string`   | The prefix to the metric name.                                                      | `"loki_process_custom_"` | no       |
| `source`            | `string`   | Key from the extracted data map to use for the metric. Defaults to the metric name. | `""`                     | no       |
| `value`             | `string`   | If set, the metric only changes if `source` exactly matches the `value`.            | `""`                     | no       |

The valid `action` values are `inc`, `dec`, `set`, `add`, or `sub`.
`inc` and `dec` increment and decrement the metric's value by 1 respectively.
If `set`, `add`, or `sub` is chosen, the extracted value must be convertible to a positive float and is set, added to, or subtracted from the metric's value.

#### `metric.histogram`

Defines a histogram metric whose values are recorded in predefined buckets.

The following arguments are supported:

| Name                | Type          | Description                                                                         | Default                  | Required |
| ------------------- | ------------- | ----------------------------------------------------------------------------------- | ------------------------ | -------- |
| `buckets`           | `list(float)` | Predefined buckets                                                                  |                          | yes      |
| `name`              | `string`      | The metric name.                                                                    |                          | yes      |
| `description`       | `string`      | The metric's description and help text.                                             | `""`                     | no       |
| `max_idle_duration` | `duration`    | Maximum amount of time to wait until the metric is marked as 'stale' and removed.   | `"5m"`                   | no       |
| `prefix`            | `string`      | The prefix to the metric name.                                                      | `"loki_process_custom_"` | no       |
| `source`            | `string`      | Key from the extracted data map to use for the metric. Defaults to the metric name. | `""`                     | no       |
| `value`             | `string`      | If set, the metric only changes if `source` exactly matches the `value`.            | `""`                     | no       |

#### `metrics` behavior

If `value` isn't present, all incoming log entries match.

Label values on created metrics can be dynamic, which can cause exported metrics to explode in cardinality or go stale, for example, when a stream stops receiving new logs.
To prevent unbounded growth of the `/metrics` endpoint, any metrics which haven't been updated within `max_idle_duration` are removed.
The `max_idle_duration` must be greater or equal to `"1s"`, and it defaults to `"5m"`.

The metric values extracted from the log data are internally converted to floats.
The supported values are the following:

* Integer
* Floating point number
* String - Two types of string format are supported:
  * Strings that represent floating point numbers, for example, "0.804" is converted to 0.804.
  * Duration format strings. Valid time units are `"ns"`, `"us"`, `"ms"`, `"s"`, `"m"`, `"h"`.
    A value in this format is converted to a floating point number of seconds, for example, `"0.5ms"` is converted to `0.0005`.
* Boolean:
  * `true` is converted to `1`.
  * `false` is converted to `0`.

The following pipeline creates a counter which increments every time any log line is received by using the `match_all` parameter.
The pipeline creates a second counter which adds the byte size of these log lines by using the `count_entry_bytes` parameter.

These two metrics disappear after 24 hours if no new entries are received, to avoid building up metrics which no longer serve any use.
These two metrics are a good starting point to track the volume of log streams in both the number of entries and their byte size, to identify sources of high-volume or high-cardinality data.

```alloy
stage.metrics {
    metric.counter {
        name        = "log_lines_total"
        description = "total number of log lines"
        prefix      = "my_custom_tracking_"

        match_all         = true
        action            = "inc"
        max_idle_duration = "24h"
    }
}
stage.metrics {
    metric.counter {
        name        = "log_bytes_total"
        description = "total bytes of log lines"
        prefix      = "my_custom_tracking_"

        match_all         = true
        count_entry_bytes = true
        action            = "add"
        max_idle_duration = "24h"
    }
}
```

Here, the first stage uses a regular expression to extract text in the format `order_status=<string>` in the log line.
The second stage, defines a counter which increments the `successful_orders_total` and `failed_orders_total` based on the previously extracted values.

```alloy
stage.regex {
    expression = "^.* order_status=(?P<order_status>.*?) .*$"
}
stage.metrics {
    metric.counter {
        name        = "successful_orders_total"
        description = "successful orders"
        source      = "order_status"
        value       = "success"
        action      = "inc"
    }
}
stage.metrics {
    metric.counter {
        name        = "failed_orders_total"
        description = "failed orders"
        source      = "order_status"
        value       = "fail"
        action      = "inc"
    }
}
```

In this example, the first stage extracts text in the format of `retries=<value>`, from the log line.
The second stage creates a gauge whose current metric value is increased by the number extracted from the retries field.

```alloy
stage.regex {
    expression = "^.* retries=(?P<retries>\\d+) .*$"
}
stage.metrics {
    metric.gauge {
        name        = "retries_total"
        description = "total_retries"
        source      = "retries"
        action      = "add"
    }
}
```

The following example shows a histogram that reads `response_time` from the extracted map and places it into a bucket, both increasing the count of the bucket and the sum for that particular bucket:

```alloy
stage.metrics {
    metric.histogram {
        name        = "http_response_time_seconds"
        description = "recorded response times"
        source      = "response_time"
        buckets     = [0.001,0.0025,0.005,0.010,0.025,0.050]
    }
}
```

### `stage.multiline`

The `stage.multiline` inner block merges multiple lines into a single block before passing it on to the next stage in the pipeline.

The following arguments are supported:

| Name            | Type       | Description                                                 | Default | Required |
|-----------------|------------|-------------------------------------------------------------|---------|----------|
| `firstline`     | `string`   | Name from extracted data to use for the log entry.          |         | yes      |
| `max_lines`     | `number`   | The maximum number of lines a block can have.               | `128`   | no       |
| `max_wait_time` | `duration` | The maximum time to wait for a multiline block.             | `"3s"`  | no       |
| `trim_newlines` | `bool`     | Whether to trim newlines from the end of each merged entry. | `true`  | no       |

A new block is identified by the RE2 regular expression passed in `firstline`.

Any line that does _not_ match the expression is considered to be part of the block of the previous match.
If no new logs arrive with `max_wait_time`, the block is sent on.
The `max_lines` field defines the maximum number of lines a block can have.
If this is exceeded, a new block is started.

The following example stage and stream of log entries from a Flask web service show how this works.

```text
stage.multiline {
    firstline     = "^\\[\\d{4}-\\d{2}-\\d{2} \\d{1,2}:\\d{2}:\\d{2}\\]"
    max_wait_time = "10s"
}

[2023-01-18 17:41:21] "GET /hello HTTP/1.1" 200 -
[2023-01-18 17:41:25] ERROR in app: Exception on /error [GET]
Traceback (most recent call last):
  File "/home/pallets/.pyenv/versions/3.8.5/lib/python3.8/site-packages/flask/app.py", line 2447, in wsgi_app
    response = self.full_dispatch_request()
  File "/home/pallets/.pyenv/versions/3.8.5/lib/python3.8/site-packages/flask/app.py", line 1952, in full_dispatch_request
    rv = self.handle_user_exception(e)
  File "/home/pallets/.pyenv/versions/3.8.5/lib/python3.8/site-packages/flask/app.py", line 1821, in handle_user_exception
    reraise(exc_type, exc_value, tb)
  File "/home/pallets/.pyenv/versions/3.8.5/lib/python3.8/site-packages/flask/_compat.py", line 39, in reraise
    raise value
  File "/home/pallets/.pyenv/versions/3.8.5/lib/python3.8/site-packages/flask/app.py", line 1950, in full_dispatch_request
    rv = self.dispatch_request()
  File "/home/pallets/.pyenv/versions/3.8.5/lib/python3.8/site-packages/flask/app.py", line 1936, in dispatch_request
    return self.view_functions[rule.endpoint](**req.view_args)
  File "/home/pallets/src/deployment_tools/hello.py", line 10, in error
    raise Exception("Sorry, this route always breaks")
Exception: Sorry, this route always breaks
[2023-01-18 17:42:24] "GET /error HTTP/1.1" 500 -
[2023-01-18 17:42:29] "GET /hello HTTP/1.1" 200 -
```

All 'blocks' that form log entries of separate web requests start with a timestamp in square brackets.
The stage detects this with the regular expression in `firstline` to collapse all lines of the traceback into a single block and thus a single Loki log entry.

### `stage.output`

The `stage.output` inner block configures a processing stage that reads from the extracted map and changes the content of the log entry that's forwarded to the next component.

The following arguments are supported:

| Name     | Type     | Description                                        | Default | Required |
| -------- | -------- | -------------------------------------------------- | ------- | -------- |
| `source` | `string` | Name from extracted data to use for the log entry. |         | yes      |

The following example log line and three-stage pipeline shows how this works.

```alloy
{"user": "John Doe", "message": "hello, world!"}

stage.json {
    expressions = { "user" = "user", "message" = "message" }
}

stage.labels {
    values = { "user" = "user" }
}

stage.output {
    source = "message"
}
```

The first stage extracts the following key-value pairs into the shared map:

```text
user: John Doe
message: hello, world!
```

Then, the second stage adds `user="John Doe"` to the label set of the log entry, and the final output stage changes the log line from the original JSON to `hello, world!`.

### `stage.pack`

The `stage.pack` inner block configures a transforming stage that replaces the log entry with a JSON object that embeds extracted values and labels with it.

The following arguments are supported:

| Name               | Type           | Description                                                                     | Default | Required |
| ------------------ | -------------- | ------------------------------------------------------------------------------- | ------- | -------- |
| `labels`           | `list(string)` | The values from the extracted data and labels to pack with the log entry.       |         | yes      |
| `ingest_timestamp` | `bool`         | Whether to replace the log entry timestamp with the time the `pack` stage runs. | `true`  | no       |

This stage lets you embed extracted values and labels together with the log line, by packing them into a JSON object.
The original message is stored under the `_entry` key, and all other keys retain their values.
This is useful in cases where you _do_ want to keep a certain label or metadata, but you don't want it to be indexed as a label due to high cardinality.

The querying capabilities of Loki make it easy to still access this data so it can be filtered and aggregated at query time.

For example, consider the following log entry:

```text
log_line: "something went wrong"
labels:   { "level" = "error", "env" = "dev", "user_id" = "f8fas0r" }
```

and this processing stage:

```alloy
stage.pack {
    labels = ["env", "user_id"]
}
```

The stage transforms the log entry into the following JSON object, where the two embedded labels are removed from the original log entry:

```json
{
  "_entry": "something went wrong",
  "env": "dev",
  "user_id": "f8fas0r"
}
```

At query time, the Loki [`unpack` parser][unpack parser] can be used to access these embedded labels and replace the log line with the original one stored in the `_entry` field automatically.

[unpack parser]: https://grafana.com/docs/loki/latest/logql/log_queries/#unpack

When combining several log streams to use with the `pack` stage, you can set `ingest_timestamp` to true to avoid interlaced timestamps and out-of-order ingestion issues.

### `stage.pattern`

The `stage.pattern` inner block configures a processing stage that parses log lines using
[LogQL pattern][logql pattern] expressions and uses named captures to add data into the shared extracted map of values.

The following arguments are supported:

| Name                 | Type     | Description                                                            | Default | Required |
| -------------------- | -------- | ---------------------------------------------------------------------- | ------- | -------- |
| `pattern`            | `string` | A valid LogQL pattern expression. At least one capture must be named.  |         | yes      |
| `source`             | `string` | Name from extracted data to parse. If empty, uses the log message.     | `""`    | no       |
| `labels_from_groups` | `bool`   | Whether to automatically add named capture groups as labels.           | `false` | no       |

The `pattern` field needs to be a [LogQL pattern][logql pattern] expression.
Every matched capture is added to the extracted map.
The name of the capture is used as the key in the extracted map for the matched value.

When `labels_from_groups` is set to true, any named captures from the pattern expression are automatically added as labels in addition to being added to the extracted map.
If a capture group name matches an existing label name, the existing label's value will be overridden by the extracted value.

If the `source` is empty or missing, then the stage parses the log line itself.
If the `source` set, the stage parses a previously extracted value with the same name.

Given the following log line and pattern stage, the extracted values are shown below:

```alloy
2019-01-01T01:00:00.000000001Z stderr P i'm a log message!

stage.pattern {
    pattern = "<time> <stream> <flags> <content>"
}

time: 2019-01-01T01:00:00.000000001Z,
stream: stderr,
flags: P,
content: i'm a log message
```

If the `source` value is set, then the pattern expression is applied to the value stored in the shared map under that name.

The following log line is put through this two-stage pipeline:

```alloy
{"timestamp":"2022-01-01T01:00:00.000000001Z"}

stage.json {
    expressions = { time = "timestamp" }
}
stage.pattern {
    pattern = "<year>-<month>-<day>T<hour>:<minute>:<second>.<nano>Z"
    source  = "time"
}
```

The first stage adds the following key-value pair into the extracted map:

```text
time: 2022-01-01T01:00:00.000000001Z
```

Then, the pattern stage parses the value for time from the shared values and appends the subsequent key-value pair back into the extracted values map:

```text
time: 2022-01-01T01:00:00.000000001Z
year: 2022
month: 01
day: 01
hour: 01
minute: 00
second: 00
nano: 000000001
```

The following example demonstrates how `labels_from_groups` can automatically add the matched groups as labels:

```alloy
{"timestamp":"2022-01-01T01:00:00.000000001Z"}

stage.json {
    expressions = { time = "timestamp" }
}
stage.pattern {
    pattern            = "<year>-<month>-<day>T<hour>:<minute>:<second>.<nano>Z"
    source             = "time"
    labels_from_groups = true   // Sets up a label for each field parsed from the pattern
}
```

This pipeline produces the same extracted values as before in addition to adding labels to the entry:

```text
time: 2022-01-01T01:00:00.000000001Z
year: 2022
month: 01
day: 01
hour: 01
minute: 00
second: 00
nano: 000000001
```

[logql pattern]: https://grafana.com/docs/loki/latest/query/log_queries/#pattern

### `stage.regex`

The `stage.regex` inner block configures a processing stage that parses log lines using regular expressions and uses named capture groups for adding data into the shared extracted map of values.

The following arguments are supported:

| Name                 | Type     | Description                                                        | Default | Required |
| -------------------- | -------- | ------------------------------------------------------------------ | ------- | -------- |
| `expression`         | `string` | A valid RE2 regular expression. Each capture group must be named.  |         | yes      |
| `source`             | `string` | Name from extracted data to parse. If empty, uses the log message. | `""`    | no       |
| `labels_from_groups` | `bool`   | Whether to automatically add named capture groups as labels.       | `false` | no       |

The `expression` field needs to be a RE2 regular expression string.
Every matched capture group is added to the extracted map, so it must be named like: `(?P<name>re)`.
The name of the capture group is then used as the key in the extracted map for the matched value.

Because of how {{< param "PRODUCT_NAME" >}} syntax strings work, any backslashes in `expression` must be escaped with a double backslash, for example, `"\\w"` or `"\\S+"`.

When `labels_from_groups` is set to true, any named capture groups from the regex expression are automatically added as labels in addition to being added to the extracted map.
If a capture group name matches an existing label name, the existing label's value will be overridden by the extracted value.

If the `source` is empty or missing, then the stage parses the log line itself.
If the `source` is set, the stage parses a previously extracted value with the same name.

Given the following log line and regular expression stage, the extracted values are shown below:

```alloy
2019-01-01T01:00:00.000000001Z stderr P i'm a log message!

stage.regex {
    expression = "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<content>.*)$"
}

time: 2019-01-01T01:00:00.000000001Z,
stream: stderr,
flags: P,
content: i'm a log message
```

If the `source` value is set, then the regular expression is applied to the value stored in the shared map under that name.

The following log line is put through this two-stage pipeline:

```alloy
{"timestamp":"2022-01-01T01:00:00.000000001Z"}

stage.json {
    expressions = { time = "timestamp" }
}
stage.regex {
    expression = "^(?P<year>\\d+)"
    source     = "time"
}
```

The first stage adds the following key-value pair into the extracted map:

```text
time: 2022-01-01T01:00:00.000000001Z
```

Then, the regular expression stage parses the value for time from the shared values and appends the subsequent key-value pair back into the extracted values map:

```text
year: 2022
```

The following example demonstrates how `labels_from_groups` can automatically add the matched groups as labels:

```alloy
{"timestamp":"2022-01-01T01:00:00.000000001Z"}

stage.json {
    expressions = { time = "timestamp" }
}
stage.regex {
    expression = "^(?P<year>\\d+)"
    source = "time"
    labels_from_groups = true   // Sets up an 'year' label, based on the 'year' capture group's value.
}
```

This pipeline produces the same extracted values as before:

```text
time: 2022-01-01T01:00:00.000000001Z
year: 2022
```

### `stage.replace`

The `stage.replace` inner block configures a stage that parses a log line using a regular expression and replaces the log line contents.
Named capture groups in the regular expression also support adding data into the shared extracted map.

The following arguments are supported:

| Name         | Type     | Description                                                     | Default | Required |
| ------------ | -------- | --------------------------------------------------------------- | ------- | -------- |
| `expression` | `string` | A RE2 regular expression containing capture groups.             |         | yes      |
| `replace`    | `string` | Value replaced by the capture group.                            |         | no       |
| `source`     | `string` | Source of the data to parse. If empty, it uses the log message. |         | no       |

Each capture group and named capture group in `expression` is replaced with the value given in `replace`.

`expression` must contain valid RE2 regular expression capture groups.
You can also name some groups using syntax such as `(?P<name>re)`.
If any of the capture groups are named, their values will be set into the shared extracted map under the name of the regular expression group.

The `source` field defines the source of data to parse using `expression`.
When `source` is missing or empty, the stage parses the log line itself, but it can also be used to parse a previously extracted value.
The replaced value is assigned back to the `source` key.

Because of how {{< param "PRODUCT_NAME" >}} syntax treats backslashes in double-quoted strings, all backslashes in a regular expression must be escaped like `"\\w*"`.

The following log line and stage shows how this works.
Since `source` is omitted, the replacement occurs on the log line itself.

```alloy
2023-01-01T01:00:00.000000001Z stderr P i'm a log message who has sensitive information with password xyz!

stage.replace {
    expression = "password (\\S+)"
    replace    = "*****"
}
```

The log line is transformed to

```text
2023-01-01T01:00:00.000000001Z stderr P i'm a log message who has sensitive information with password *****!
```

If `replace` is empty, then the captured value is omitted instead.

In the following example, `source` is defined.

```alloy
{"time":"2023-01-01T01:00:00.000000001Z", "level": "info", "msg":"11.11.11.11 - \"POST /loki/api/push/ HTTP/1.1\" 200 932 \"-\" \"Mozilla/5.0\"}

stage.json {
    expressions = { "level" = "", "msg" = "" }
}

stage.replace {
    expression = "\\S+ - \"POST (\\S+) .*"
    source     = "msg"
    replace    = "redacted_url"
}
```

The JSON stage adds the following key-value pairs into the extracted map:

```text
time: 2023-01-01T01:00:00.000000001Z
level: info
msg: "11.11.11.11 - "POST /loki/api/push/ HTTP/1.1" 200 932 "-" "Mozilla/5.0"
```

The `replace` stage acts on the `msg` value. The capture group matches against `/loki/api/push` and is replaced by `redacted_url`.

The `msg` value is finally transformed into:

```text
msg: "11.11.11.11 - "POST redacted_url HTTP/1.1" 200 932 "-" "Mozilla/5.0"
```

The `replace` field can use a set of templating functions, by utilizing Go's [text/template][] package.

[text/template]: https://pkg.go.dev/text/template

The following example shows how this works with named capture groups with a sample log line and stage.

```alloy
11.11.11.11 - alloy [01/Jan/2023:00:00:01 +0200]

stage.replace {
    expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\]"
    replace    = "{{ .Value | ToUpper }}"
}
```

Since `source` is empty, the regular expression parses the log line itself and extracts the named capture groups to the shared map of values.
The `replace` field acts on these extracted values and converts them to uppercase:

```text
ip: 11.11.11.11
identd: -
user: FRANK
timestamp: 01/JAN/2023:00:00:01 +0200
```

and the log line becomes:

```text
11.11.11.11 - FRANK [01/JAN/2023:00:00:01 +0200]
```

The following list contains available functions with examples of more complex `replace` fields.

```text
ToLower, ToUpper, Replace, Trim, TrimLeftTrimRight, TrimPrefix, TrimSuffix, TrimSpace, Hash, Sha2Hash, regexReplaceAll, regexReplaceAllLiteral

| "{{ if eq .Value \"200\" }}{{ Replace .Value \"200\" \"HttpStatusOk\" -1 }}{{ else }}{{ .Value | ToUpper }}{{ end }}" |
| "*IP4*{{ .Value                                                                                | Hash "salt" }}*"     |
```

### `stage.sampling`

The `sampling` stage is used to sample the logs.
Configuring the value `rate = 0.1` means that 10% of the logs continue to be processed.
The remaining 90% of the logs are dropped.

The following arguments are supported:

| Name                  | Type     | Description                                                                                        | Default          | Required |
| --------------------- | -------- | -------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| `rate`                | `float`  | The sampling rate in a range of `[0, 1]`                                                           |                  | yes      |
| `drop_counter_reason` | `string` | The label to add to `loki_process_dropped_lines_total` metric when logs are dropped by this stage. | `sampling_stage` | no       |

For example, the configuration below will sample 25% of the logs and drop the remaining 75%.
When logs are dropped, the `loki_process_dropped_lines_total` metric is incremented with an additional `reason=logs_sampling` label.

```alloy
stage.sampling {
    rate = 0.25
    drop_counter_reason = "logs_sampling"
}
```

### `stage.static_labels`

The `stage.static_labels` inner block configures a static_labels processing stage that adds a static set of labels to incoming log entries.

For labels that are dynamic, refer to [`stage.labels`][stage.labels]

The following arguments are supported:

| Name     | Type          | Description                                    | Default | Required |
| -------- | ------------- | ---------------------------------------------- | ------- | -------- |
| `values` | `map(string)` | Configures a `static_labels` processing stage. | `{}`    | no       |

```alloy
stage.static_labels {
    values = {
      foo = "fooval",
      bar = "barval",
    }
}
```

### `stage.structured_metadata`

The `stage.structured_metadata` inner block configures a stage that can read data from the extracted values map and add them to log entries as structured metadata.
If you add labels to structured metadata, the labels are removed from the label map.

The following arguments are supported:

| Name     | Type          | Description                                                                                 | Default | Required |
| -------- | ------------- | ------------------------------------------------------------------------------------------- | ------- | -------- |
| `values` | `map(string)` | Specifies the list of labels to add from extracted values map to log entry.                 | `{}`    | no       |
| `regex`  | `string`      | Specifies a regular expression, matching extracted values are added to structured metadata. | `""`    | no       |

In a `structured_metadata` stage, the keys for the `values` map define the labels to set and the values define how to look them up.
If the value is empty, it's inferred to be the same as the key.

```alloy
stage.structured_metadata {
    values = {
      env  = "",         // Sets up an 'env' property to structured metadata, based on the 'env' extracted value.
      user = "username", // Sets up a 'user' property to structured metadata, based on the 'username' extracted value.
    }
    regex = "pod_.*"   // Adds all extracted values starting with 'pod_' to structured metadata.
}
```

### `stage.structured_metadata_drop`

The `stage.structured_metadata_drop` inner block configures a processing stage that drops structured metadata from incoming log entires.

The following arguments are supported:

| Name     | Type           | Description                                               | Default | Required |
| -------- | -------------- | --------------------------------------------------------- | ------- | -------- |
| `values` | `list(string)` | Configures a `structured_metadata_drop` processing stage. | `[]`    |          |

```alloy
stage.structured_metadata_drop {
    values = [ "kubernetes_node_name", "kubernetes_namespace" ]
}
```

### `stage.template`

The `stage.template` inner block configures a transforming stage that allows users to manipulate the values in the extracted map by using Go's `text/template` [package][] syntax.
This stage is primarily useful for manipulating and standardizing data from previous stages before setting them as labels in a subsequent stage.
Example use cases are replacing spaces with underscores, converting uppercase strings to lowercase, or hashing a value.

[package]: https://pkg.go.dev/text/template

The template stage can also create new keys in the extracted map.

The following arguments are supported:

| Name       | Type     | Description                                                                          | Default | Required |
| ---------- | -------- | ------------------------------------------------------------------------------------ | ------- | -------- |
| `source`   | `string` | Name from extracted data to parse. If the key doesn't exist, a new entry is created. |         | yes      |
| `template` | `string` | Go template string to use.                                                           |         | yes      |

The template string can be any valid template that can be used by Go's `text/template`.
It supports all functions from the [sprig package][], as well as the following list of custom functions:

[sprig package]: http://masterminds.github.io/sprig/

```text
ToLower, ToUpper, Replace, Trim, TrimLeftTrimRight, TrimPrefix, TrimSuffix, TrimSpace, Hash, Sha2Hash, regexReplaceAll, regexReplaceAllLiteral
```

More details on each of these functions can be found in the [supported functions][] section.

[supported functions]: #supported-functions

Assuming no data is present on the extracted map, the following stage simply adds the `new_key: "hello_world"` key-value pair to the shared map.

```alloy
stage.template {
    source   = "new_key"
    template = "hello_world"
}
```

If the `source` value exists in the extract fields, its value can be referred to as `.Value` in the template.
The next stage takes the current value of `app` from the extracted map, converts it to lowercase, and adds a suffix to its value:

```alloy
stage.template {
    source   = "app"
    template = "{{ ToLower .Value }}_some_suffix"
}
```

Any previously extracted keys are available for `template` to expand and use.
The next stage takes the current values for `level`, `app` and `module` and creates a new key named `output_message`:

```alloy
stage.template {
    source   = "output_msg"
    template = "{{ .level }} for app {{ ToUpper .app }} in module {{.module}}"
}
```

A special key named `Entry` can be used to reference the current line.
This can be useful when you need to append/prepend something to the log line, like this snippet:

```alloy
stage.template {
    source   = "message"
    template = "{{.app }}: {{ .Entry }}"
}
stage.output {
    source = "message"
}
```

#### Supported functions

In addition to supporting all functions from the [sprig package](http://masterminds.github.io/sprig/), the `template` stage supports the following custom functions.

##### `ToLower` and `ToUpper`

`ToLower` and `ToUpper` convert the entire string to lowercase and uppercase, respectively.

The following example shows how you can use this function.

```alloy
stage.template {
    source   = "out"
    template = "{{ ToLower .app }}"
}
stage.template {
    source   = "out"
    template = "{{ .app | ToUpper }}"
}
```

##### `Replace`

The `Replace` function syntax is defined as `{{ Replace <string> <old> <new> <n> }}`.

The function returns a copy of the input string, with instances of the `<old>` argument being replaced by `<new>`.
The function replaces up to `<n>` non-overlapping instances of the second argument.
If `<n>` is less than zero, there is no limit on the number of replacements.
Finally, if `<old>` is empty, it matches before and after every UTF-8 character in the string.

This example replaces the first two instances of the `loki` word by `Loki`:

```alloy
stage.template {
    source   = "output"
    template = `{{ Replace .Value "loki" "Loki" 2 }}`
}
```

##### `Trim`, `TrimLeft`, `TrimRight`, `TrimSpace`, `TrimPrefix`, `TrimSuffix`

* `Trim` returns a slice of the string `s` with all leading and trailing Unicode code points contained in `cutset` removed.
* `TrimLeft` and `TrimRight` are the same as Trim except that they trim only leading and trailing characters, respectively.
* `TrimSpace` returns a slice of the string s, with all leading and trailing white space removed, as defined by Unicode.
* `TrimPrefix` and `TrimSuffix` trim the supplied prefix or suffix, respectively.

The following example shows how you can use this function.

```alloy
stage.template {
    source   = "output"
    template = `{{ Trim .Value ",. " }}`
}
stage.template {
    source   = "output"
    template = "{{ TrimSpace .Value }}"
}
stage.template {
    source   = "output"
    template = `{{ TrimPrefix .Value "--" }}`
}
```

##### `regexReplaceAll` and `regexReplaceAllLiteral`

`regexReplaceAll` returns a copy of the input string, replacing matches of the Regexp with the replacement string.
Inside the replacement string, `$` characters are interpreted as in Expand functions.
For example, $1 represents the first captured submatch.

`regexReplaceAllLiteral` returns a copy of the input string, replacing matches of the Regexp with the replacement string.
The replacement string is substituted directly, without using Expand.

```alloy
stage.template {
    source   = "output"
    template = `{{ regexReplaceAll "(a*)bc" .Value "${1}a" }}`
}
stage.template {
    source   = "output"
    template = `{{ regexReplaceAllLiteral "(ts=)" .Value "timestamp=" }}`
}
```

##### `Hash` and `Sha2Hash`

`Hash` returns a `Sha3_256` hash of the string, represented as a hexadecimal number of 64 digits.
You can use it to obfuscate sensitive data and PII in the logs.
It requires a (fixed) salt value, to add complexity to low input domains, for example, all possible social security numbers.
`Sha2Hash` returns a `Sha2_256` of the string which is faster and less CPU-intensive than `Hash`, however it's less secure.

The following example shows how you can use this function.

```alloy
stage.template {
    source   = "output"
    template = `{{ Hash .Value "salt" }}`
}
stage.template {
    source   = "output"
    template = `{{ Sha2Hash .Value "salt" }}`
}
```

We recommend using Hash as it has a stronger hashing algorithm.

### `stage.tenant`

The `stage.tenant` inner block sets the tenant ID for the log entry by obtaining it from a field in the extracted data map, a label, or a provided value.

The following arguments are supported:

| Name     | Type     | Description                                            | Default | Required |
| -------- | -------- | ------------------------------------------------------ | ------- | -------- |
| `label`  | `string` | The label to set as tenant ID.                         | `""`    | no       |
| `source` | `string` | The name from the extracted value to use as tenant ID. | `""`    | no       |
| `value`  | `string` | The value to set as the tenant ID.                     | `""`    | no       |

The block expects only one of `label`, `source` or `value` to be provided.

The following stage assigns the fixed value `team-a` as the tenant ID:

```alloy
stage.tenant {
    value = "team-a"
}
```

This stage extracts the tenant ID from the `customer_id` field after parsing the log entry as JSON in the shared extracted map:

```alloy
stage.json {
    expressions = { "customer_id" = "" }
}
stage.tenant {
    source = "customer_id"
}
```

The final example extracts the tenant ID from a label set by a previous stage:

```alloy
stage.labels {
  values = {
    "namespace" = "k8s_namespace",
  }
}

stage.tenant {
    label = "namespace"
}
```

### `stage.timestamp`

The `stage.timestamp` inner block configures a processing stage that sets the timestamp of log entries before they're forwarded to the next component.
When no timestamp stage is set, the log entry timestamp defaults to the time when the log entry was scraped.

The following arguments are supported:

| Name                | Type           | Description                                                 | Default   | Required |
| ------------------- | -------------- | ----------------------------------------------------------- | --------- | -------- |
| `format`            | `string`       | Determines how to parse the source string.                  |           | yes      |
| `source`            | `string`       | Name from extracted values map to use for the timestamp.    |           | yes      |
| `action_on_failure` | `string`       | What to do when the timestamp can't be extracted or parsed. | `"fudge"` | no       |
| `fallback_formats`  | `list(string)` | Fallback formats to try if the `format` field fails.        | `[]`      | no       |
| `location`          | `string`       | IANA Timezone Database location to use when parsing.        | `""`      | no       |

{{< admonition type="note" >}}
Be careful with further stages which may also override the timestamp.
For example, a `stage.pack` with `ingest_timestamp` set to `true` could replace the timestamp which `stage.timestamp` had set earlier in the pipeline.
{{< /admonition >}}

The `source` field defines which value from the shared map of extracted values the stage should attempt to parse as a timestamp.

The `format` field defines _how_ that source should be parsed.

The `format` can be set to one of the following shorthand values for commonly used forms:

```text
ANSIC: Mon Jan _2 15:04:05 2006
UnixDate: Mon Jan _2 15:04:05 MST 2006
RubyDate: Mon Jan 02 15:04:05 -0700 2006
RFC822: 02 Jan 06 15:04 MST
RFC822Z: 02 Jan 06 15:04 -0700
RFC850: Monday, 02-Jan-06 15:04:05 MST
RFC1123: Mon, 02 Jan 2006 15:04:05 MST
RFC1123Z: Mon, 02 Jan 2006 15:04:05 -0700
RFC3339: 2006-01-02T15:04:05-07:00
RFC3339Nano: 2006-01-02T15:04:05.999999999-07:00
```

Additionally, support for common Unix timestamps is supported with the following format values:

```text
Unix: 1562708916 or with fractions 1562708916.000000123
UnixMs: 1562708916414
UnixUs: 1562708916414123
UnixNs: 1562708916000000123
```

Otherwise, the field accepts a custom format string that defines how an arbitrary reference point in history should be interpreted by the stage.
The arbitrary reference point is Mon Jan 2 15:04:05 -0700 MST 2006.

The string value of the field is passed directly to the layout parameter in Go's [`time.Parse`](https://pkg.go.dev/time#Parse) function.

If the custom format has no year component, the stage uses the current year, according to the system's clock.

The following table shows the supported reference values to use when defining a custom format.

| Timestamp Component | Format value                                                                                                             |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Year                | 06, 2006                                                                                                                 |
| Month               | 1, 01, Jan, January                                                                                                      |
| Day                 | 2, 02, _2 (two digits right justified)                                                                                   |
| Day of the week     | Mon, Monday                                                                                                              |
| Hour                | 3 (12-hour), 03 (12-hour zero prefixed), 15 (24-hour)                                                                    |
| Minute              | 4, 04                                                                                                                    |
| Second              | 5, 05                                                                                                                    |
| Fraction of second  | .000 (ms zero prefixed), .000000 (μs), .000000000 (ns), .999 (ms without trailing zeroes), .999999 (μs), .999999999 (ns) |
| 12-hour period      | pm, PM                                                                                                                   |
| Timezone name       | MST                                                                                                                      |
| Timezone offset     | -0700, -070000 (with seconds), -07, -07:00, -07:00:00 (with seconds)                                                     |
| Timezone ISO-8601   | Z0700 (Z for UTC or time offset), Z070000, Z07, Z07:00, Z07:00:00                                                        |

{{< admonition type="note" >}}
If you define a custom timestamp, the `format` value for each timestamp component must be one of the values specified in the table above.
For example, to indicate the year, the `format` value must be `06` or `2006` and not any other value like `23` or `2023`.
{{< /admonition >}}

The `fallback_formats` field defines one or more format fields to try and parse the timestamp with, if parsing with `format` fails.

The `location` field must be a valid IANA Timezone Database location and determines in which timezone the timestamp value is interpreted to be in.

The `action_on_failure` field defines what should happen when the source field doesn't exist in the shared extracted map, or if the timestamp parsing fails.

The supported actions are:

* fudge (default): Change the timestamp to the last known timestamp, summing up 1 nanosecond to guarantee log entries ordering.
* skip: Don't change the timestamp and keep the time when the log entry was scraped.

The following stage fetches the `time` value from the shared values map, parses it as a RFC3339 format, and sets it as the log entry's timestamp.

```alloy
stage.timestamp {
    source = "time"
    format = "RFC3339"
}
```

The following example would parse a timestamp such as `2024-12-20T09:14:58,381+02:00`:

```alloy
stage.timestamp {
    source = "time"
    format = "2006-01-02T15:04:05,000-07:00"
}
```

### `stage.truncate`

The `stage.truncate` inner block configures a processing stage that truncates log entries, labels, extracted map fields, or structured metadata that exceed configured limits.

The `stage.truncate` block doesn't support any arguments and is only configured via a number of nested inner `rule` blocks.

The following block is supported inside the definition of `stage.truncate`:

| Block              | Description                   | Required |
| ------------------ | ----------------------------- | -------- |
| [`rule`][rule]     | Defines a truncation rule.    | yes      |

[rule]: #rule

#### `rule`

Defines a truncation rule that will apply to the log line, labels, structured_metadata, or extracted map.

The following arguments are supported:

| Name          | Type     | Description                                                  | Default  | Required |
|---------------|----------|--------------------------------------------------------------|----------|----------|
| `limit`       | `string` | Maximum length before truncating.                            | `""`     | yes      |
| `source`      | `string` | Source of the data to truncate. If empty, will truncate all. | `""`     | no       |
| `source_type` | `string` | Source location of the data to truncate.                     | `"line"` | no       |
| `suffix`      | `string` | Suffix to append to truncated values.                        | ``       | no       |

The `limit` attribute should be expressed in logical units, for example `"1KiB"`.
The stage checks the byte length of the log line, label values, or structured metadata values against the configured limit and truncates if it exceeds the limit.
If you provide a `suffix`, the limit is reduced by the length of the `suffix`, and the `suffix` is appended to the truncated value.

The `source_type` attribute must be one of `"line"`, `"label"`, `"structured_metadata"`, or `"extracted"`.
If the `source` attribute is specified, the stage will only truncate a label, structured_metadata, or extracted field of the same name.
If `source` is empty, all labels, structured_metadata, or extracted fields will be truncated if they exceed the limit.

Whenever a line, label, extracted field, or structured_metadata value is truncated, the metric `loki_process_truncated_fields_total` is incremented.
The `field` label will either be `line`, `label`, `extracted`, or `structured_metadata`.

If anything has been truncated, the extracted map for the entry contains a `"truncated"` field with a comma delimited list of field types that have been truncated.

#### Example

```alloy
loki.process "default" {
  forward_to = [loki.write.default.receiver]
  stage.truncate {
    rule {
      limit = "12B"
      suffix = "..."
    }

    rule {
      limit = "12B"
      suffix = "..."
      source_type = "label"
    }
  }
}
```

Given the following input entry, the first rule in the stage truncates the entry `"I'm a log message!"` into `"I'm a log..."`.
The second rule truncates the `"label2"` label's value `"hello world"` to `"hello..."`.

```text
entry: "I'm a log message!"
labels: { "label1": "hi", "label2": "hello world" }
structured_metadata: { "metadata1": "here is some metadata", "metadata2": "and here is some more"}
```

The extracted map for the entry contains the `"truncated"` field.
You can use this entry to add a label in `stage.labels` or structured metadata in `stage.structured_metadata` to the entry so you can identify the logs as truncated after processing.

```text
truncated: label,line
```

### `stage.windowsevent`

The `windowsevent` stage extracts data from the message string in the Windows Event Log.

The following arguments are supported:

| Name                  | Type     | Description                                           | Default   | Required |
| --------------------- | -------- | ----------------------------------------------------- | --------- | -------- |
| `source`              | `string` | Name of the field in the extracted data to parse.     | `message` | no       |
| `drop_invalid_labels` | `bool`   | Whether to drop fields that aren't valid label names. | `false`   | no       |
| `overwrite_existing`  | `bool`   | Whether to overwrite existing extracted data fields.  | `false`   | no       |

When `overwrite_existing` is set to `true`, the stage overwrites existing extracted data fields with the same name.
If set to `false`, the `_extracted` suffix is appended to an existing field name.

When `drop_invalid_labels` is set to `true`, the stage drops fields that aren't valid label names.
If set to `false`, the stage automatically converts them into valid labels, replacing invalid characters with underscores.

The `windowsevent` stage expects the message to be structured in sections that are split by empty lines.

The first section of the input is treated as a whole block and stored in the extracted map with the key `Description`.

Sections following the Description are expected to contain key-value pairs in the format key:value.

If the first line of a section has no value, for example "Subject:", the key acts as a prefix for subsequent keys in the same section.

If a line within a section doesn't include the `:` symbol, it's considered part of the previous entry's value.
The line is appended to the previous value, separated by a comma.

Lines in a section without a preceding valid entry (key-value pair) are ignored and discarded.

#### Example with `loki.source.windowsevent`

```alloy
loki.source.windowsevent "security"  {
    eventlog_name = "Security"
    forward_to = [loki.process.default.receiver]
}
loki.process "default" {
  forward_to = [loki.write.default.receiver]
  stage.json {
      expressions = {
          message = "",
          Overwritten = "",
      }
  }
  stage.windowsevent {
      source = "message"
      overwrite_existing = true
  }
  stage.labels {
    values = {
      Description = "",
      Subject_SecurityID  = "",        
      ReadOP = "Subject_ReadOperation",
    }
  }
}
```

The `loki.source.windowsevent` component forwards Windows security events to the `loki.process` component.

Given the following event:

```text
{"event_id": 1, "Overwritten": "old", "message": ""Special privileges assigned to new logon.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tSYSTEM\r\n\tAccount Domain:\t\tNT AUTHORITY\r\n\tLogon ID:\t\t0xAAA\r\n\r\nPrivileges:\t\tSeAssignPrimaryTokenPrivilege\r\n\t\t\tSeTcbPrivilege\r\n\t\t\tSeSecurityPrivilege\r\n\t\t\tSeTakeOwnershipPrivilege\r\n\t\t\tSeLoadDriverPrivilege\r\n\t\t\tSeBackupPrivilege\r\n\t\t\tSeRestorePrivilege\r\n\t\t\tSeDebugPrivilege\r\n\t\t\tSeAuditPrivilege\r\n\t\t\tSeSystemEnvironmentPrivilege\r\n\t\t\tSeImpersonatePrivilege\r\n\t\t\tSeDelegateSessionUserImpersonatePrivilege""}
```

The `json` stage would create the following key-value pairs in the set of extracted data:

- `message`: `"Special privileges assigned to new logon.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tSYSTEM\r\n\tAccount Domain:\t\tNT AUTHORITY\r\n\tLogon ID:\t\t0xAAA\r\n\r\nPrivileges:\t\tSeAssignPrimaryTokenPrivilege\r\n\t\t\tSeTcbPrivilege\r\n\t\t\tSeSecurityPrivilege"`
- `Overwritten`: `old`

The `windowsevent` stage parses the value of `message` from the extracted data and appends or overwrites the following key-value pairs to the set of extracted data:

* `Description`:           "Special privileges assigned to new logon.",
* `Subject_SecurityID`:    "S-1-1-1",
* `Subject_AccountName`:   "SYSTEM",
* `Subject_AccountDomain`: "NT AUTHORITY",
* `Subject_LogonID`:       "0xAAA",
* `Privileges`:            "SeAssignPrimaryTokenPrivilege,SeTcbPrivilege,SeSecurityPrivilege",

Finally the `labels` stage uses the extracted values `Description`, `Subject_SecurityID` and `Subject_ReadOperation` to add them as labels of the log entry before forwarding it to a `loki.write` component.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                   |
| ---------- | -------------- | ------------------------------------------------------------- |
| `receiver` | `LogsReceiver` | A value that other components can use to send log entries to. |

## Component health

`loki.process` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.process` doesn't expose any component-specific debug information.

## Debug metrics

* `loki_process_dropped_lines_total` (counter): Number of lines dropped as part of a processing stage.
* `loki_process_dropped_lines_by_label_total` (counter):  Number of lines dropped when `by_label_name` is non-empty in [stage.limit][].
* `loki_process_truncated_fields_total` (counter): Number of lines, label values, extracted field values, and structured_metadata values truncated as part of a `truncate` stage.

## Example

This example creates a `loki.process` component that extracts the `environment` value from a JSON log line and sets it as a label named 'env'.

```alloy
loki.process "local" {
  forward_to = [loki.write.onprem.receiver]

  stage.json {
      expressions = { "extracted_env" = "environment" }
  }

  stage.labels {
      values = { "env" = "extracted_env" }
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.process` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`loki.process` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
