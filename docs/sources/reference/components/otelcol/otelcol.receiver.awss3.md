---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.awss3/
description: Learn about otelcol.receiver.awss3
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.awss3
---

# `otelcol.receiver.awss3`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.awss3` receives telemetry stored in S3 by the [AWS S3 Exporter](./otelcol.exporter.awss3.md).

{{< admonition type="warning" >}}
`otelcol.receiver.awss3` is a wrapper over the upstream OpenTelemetry Collector [`awss3`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`awss3`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awss3receiver
{{< /admonition >}}

The receiver has two modes of operation:

- **Time Range Mode** - Specify start and end to fetch data from a specific time range.
- **SQS Message Mode** - Subscribe to SQS messages to process objects as they arrive.

The receiver supports the following encodings:

- `otlp_json` (OpenTelemetry Protocol format represented as JSON) with a suffix of `.json`
- `otlp_proto` (OpenTelemetry Protocol format represented as Protocol Buffers) with a suffix of `.binpb`

{{< admonition type="note" >}}
Currently, `otelcol.receiver.awss3` receiver doesn't support encoding extensions.
{{< /admonition >}}

You can specify multiple `otelcol.receiver.awss3` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.awss3 "<LABEL>" {
  start_time = "..."
  end_time = "..."

  s3downloader {
    s3_bucket = "..."
    s3_prefix = "..."
  }

  output {
    logs = [...]
    metrics = [...]
    trace = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.awss3`:

| Name         | Type     | Description                                 | Default | Required                      |
| ------------ | -------- | ------------------------------------------- | ------- | ----------------------------- |
| `start_time` | `string` | The time at which to start retrieving data. |         | Required if fetching by time. |
| `end_time`   | `string` | The time at which to stop retrieving data.  |         | Required if fetching by time. |

The `start_time` and `end_time` fields use one of the following time formats: RFC3339, `YYYY-MM-DD HH:MM`, or `YYYY-MM-DD`. When using `YYYY-MM-DD`, the time defaults to `00:00`.

{{< admonition type="note" >}}
Time-based configuration (`start_time` and `end_time` arguments) can't be combined together with [`sqs`][] block.

[`sqs`]: #sqs

{{< /admonition >}}

Refer to the upstream receiver [documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awss3receiver#configuration) for more details.

## Blocks

You can use the following blocks with `otelcol.receiver.awss3`:

| Block                          | Description                                                                  | Required                                  |
| ------------------------------ | ---------------------------------------------------------------------------- | ----------------------------------------- |
| [`s3downloader`][s3downloader] | Configures S3 downloader.                                                    | yes                                       |
| [`sqs`][sqs]                   | Configures SQS queue configuration for receiving object change notification. | Required if fetching by SQS notification. |
| [`output`][output]             | Configures where to send received telemetry data.                            | yes                                       |

[s3downloader]: #s3downloader
[sqs]: #sqs
[output]: #output

### `s3downloader`

{{< badge text="Required" >}}

The `s3downloader` block contains AWS S3 downloader related configuration to control things like bucket, prefix, batching, connections, retries, etc.

The following arguments are supported:

| Name                    | Type     | Description                                                                                           | Default       | Required |
| ----------------------- | -------- | ----------------------------------------------------------------------------------------------------- | ------------- | -------- |
| `s3_bucket`             | `string` | S3 bucket.                                                                                            |               | yes      |
| `s3_prefix`             | `string` | Prefix for the S3 key (root directory inside bucket).                                                 |               | yes      |
| `endpoint_partition_id` | `string` | Partition id to use if `endpoint` is specified.                                                       | `"aws"`       | no       |
| `endpoint`              | `string` | Overrides the endpoint used by the exporter instead of constructing it from `region` and `s3_bucket`. |               | no       |
| `file_prefix`           | `string` | Prefix used to filter files for download.                                                             |               | no       |
| `region`                | `string` | AWS region.                                                                                           | `"us-east-1"` | no       |
| `s3_force_path_style`   | `bool`   | When enabled, forces the request to use [path-style addressing][s3-force-path-style-ref].             | `false`       | no       |
| `s3_partition_format`   | `string` | Format for the partition key. See [strftime][] for format specification.                              | `"year=%Y/month=%m/day=%d/hour=%H/minute=%M"` | no       |
| `s3_partition_timezone` | `string` | IANA timezone name applied when formatting the partition key.                                         | Local time    | no       |

[s3-force-path-style-ref]: http://docs.aws.amazon.com/AmazonS3/latest/dev/VirtualHosting.html
[strftime]: https://www.man7.org/linux/man-pages/man3/strftime.3.html

### `sqs`

The `sqs` block holds SQS queue configuration for receiving object change notifications.

The following arguments are supported:

| Name                     | Type     | Description                                                     | Default | Required |
| ------------------------ | -------- | --------------------------------------------------------------- | ------- | -------- |
| `queue_url`              | `string` | The URL of the SQS queue that receives S3 bucket notifications. |         | yes      |
| `region`                 | `string` | AWS region of the SQS queue.                                    |         | yes      |
| `endpoint`               | `string` | Custom endpoint for the SQS service.                            |         | no       |
| `max_number_of_messages` | `int`    | Maximum number of messages to retrieve in a single SQS request. | `10`    | no       |
| `wait_time_seconds`      | `int`    | Wait time in seconds for long polling SQS requests.             | `20`    | no       |

{{< admonition type="note" >}}
You must configure your S3 bucket to send event notifications to the SQS queue.
Time-based configuration (`start_time`/`end_time`) and SQS configuration can't be used together.
{{< /admonition >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.awss3` doesn't export any fields.

## Component health

`otelcol.receiver.awss3` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.awss3` doesn't expose any component-specific debug information.

## Example

This example forwards received traces through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
// Time range mode:
otelcol.receiver.awss3 "default" {
  start_time = "2024-01-01 01:00"
  end_time = "2024-01-02"

  s3downloader {
    region = "us-west-1"
    s3_bucket = "mybucket"
    s3_prefix = "trace"
    s3_partition = "minute"
  }

  output {
    traces = [otelcol.processor.batch.default.input]
  }
}

// SQS message mode:
otelcol.receiver.awss3 "sqs_traces" {
  s3downloader {
    region = "us-east-1"
    s3_bucket = "mybucket"
    s3_prefix = "mytrace"
  }

  sqs {
    queue_url = "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue"
    region = "us-east-1"
  }

  output {
    traces = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.awss3` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
