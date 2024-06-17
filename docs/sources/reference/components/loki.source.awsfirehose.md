---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki.source.awsfirehose/
description: Learn about loki.source.awsfirehose
title: loki.source.awsfirehose
---

# loki.source.awsfirehose

`loki.source.awsfirehose` receives log entries over HTTP
from [AWS Firehose](https://docs.aws.amazon.com/firehose/latest/dev/what-is-this-service.html)
and forwards them to other `loki.*` components.

The HTTP API exposed is compatible
with the [Firehose HTTP Delivery API](https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html).
Since the API model that AWS Firehose uses to deliver data over HTTP is generic enough, the same component can be used
to receive data from multiple origins:

- [AWS CloudWatch logs](https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html)
- [AWS CloudWatch events](https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-events.html)
- Custom data through [DirectPUT requests](https://docs.aws.amazon.com/firehose/latest/dev/writing-with-sdk.html)

The component uses a heuristic to try to decode as much information as possible from each log record, and it falls back to writing
the raw records to Loki. The decoding process goes as follows:

- AWS Firehose sends batched requests
- Each record is treated individually
- For each `record` received in each request:
  - If the `record` comes from a [CloudWatch logs subscription filter](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/SubscriptionFilters.html#DestinationKinesisExample), it is decoded and each logging event is written to Loki
  - All other records are written raw to Loki

The component exposes some internal labels, available for relabeling. The following tables describes internal labels available
in records coming from any source.

| Name                        | Description                   | Example                                                                  |
|-----------------------------|-------------------------------|--------------------------------------------------------------------------|
| `__aws_firehose_request_id` | Firehose request ID.          | `a1af4300-6c09-4916-ba8f-12f336176246`                                   |
| `__aws_firehose_source_arn` | Firehose delivery stream ARN. | `arn:aws:firehose:us-east-2:123:deliverystream/aws_firehose_test_stream` |

If the source of the Firehose record is CloudWatch logs, the request is further decoded and enriched with even more labels,
exposed as follows:

| Name                       | Description                                                                                                               | Example                                  |
|----------------------------|---------------------------------------------------------------------------------------------------------------------------|------------------------------------------|
| `__aws_owner`              | The AWS Account ID of the originating log data.                                                                           | `111111111111`                           |
| `__aws_cw_log_group`       | The log group name of the originating log data.                                                                           | `CloudTrail/logs`                        |
| `__aws_cw_log_stream`      | The log stream name of the originating log data.                                                                          | `111111111111_CloudTrail/logs_us-east-1` |
| `__aws_cw_matched_filters` | The list of subscription filter names that match the originating log data. The list is encoded as a comma-separated list. | `Destination,Destination2`               |
| `__aws_cw_msg_type`        | Data messages will use the `DATA_MESSAGE` type. Sometimes CloudWatch Logs may emit Kinesis Data Streams records with a `CONTROL_MESSAGE` type, mainly for checking if the destination is reachable. | `DATA_MESSAGE`                           |

See [Examples](#example) for a full example configuration showing how to enrich each log entry with these labels.

## Usage

```alloy
loki.source.awsfirehose "LABEL" {
    http {
        listen_address = "LISTEN_ADDRESS"
        listen_port = PORT 
    }
    forward_to = RECEIVER_LIST
}
```

The component will start an HTTP server on the configured port and address with the following endpoints:

- `/awsfirehose/api/v1/push` - accepting `POST` requests compatible with [AWS Firehose HTTP Specifications](https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html).

You can use the [X-Amz-Firehose-Common-Attributes](https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html) header to set extra static labels. 
You can configure the header in the **Parameters** section of the Amazon Data Firehose delivery stream configuration. 
Label names must be prefixed with `lbl_`. The prefix is removed before the label is stored in the log entry.
Label names and label values must be compatible with the [Prometheus data model](https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels) specification.

Example of the valid `X-Amz-Firehose-Common-Attributes` value with two custom labels:
```json
{
  "commonAttributes": {
    "lbl_label1": "value1",
    "lbl_label2": "value2"
  }
}
```

## Arguments

`loki.source.awsfirehose` supports the following arguments:

| Name                     | Type                 | Description                                                    | Default | Required |
| ------------------------ | -------------------- | -------------------------------------------------------------- | ------- | -------- |
| `forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.                      |         | yes      |
| `use_incoming_timestamp` | `bool`               | Whether or not to use the timestamp received from the request. | `false` | no       |
| `relabel_rules`          | `RelabelRules`       | Relabeling rules to apply on log entries.                      | `{}`    | no       |
| `access_key`             | `secret`             | If set, require AWS Firehose to provide a matching key.        | `""`    | no       |

The `relabel_rules` field can make use of the `rules` export value from a
[`loki.relabel`][loki.relabel] component to apply one or more relabeling rules to log entries before they're forwarded
to the list of receivers in `forward_to`.

[loki.relabel]: ../loki.relabel/

## Blocks

The following blocks are supported inside the definition of `loki.source.awsfirehose`:

| Hierarchy | Name     | Description                                        | Required |
|-----------|----------|----------------------------------------------------|----------|
| `http`    | [http][] | Configures the HTTP server that receives requests. | no       |
| `grpc`    | [grpc][] | Configures the gRPC server that receives requests. | no       |

[http]: #http
[grpc]: #grpc

### http

{{< docs/shared lookup="reference/components/loki-server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### grpc

{{< docs/shared lookup="reference/components/loki-server-grpc.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.awsfirehose` does not export any fields.

## Component health

`loki.source.awsfirehose` is only reported as unhealthy if given an invalid configuration.

## Debug metrics

The following are some of the metrics that are exposed when this component is used.

{{< admonition type="note" >}}
The metrics include labels  such as `status_code` where relevant, which you can use to measure request success rates.
{{< /admonition >}}

- `loki_source_awsfirehose_request_errors` (counter): Count of errors while receiving a request.
- `loki_source_awsfirehose_record_errors` (counter): Count of errors while decoding an individual record.
- `loki_source_awsfirehose_records_received` (counter): Count of records received.
- `loki_source_awsfirehose_batch_size` (histogram): Size (in units) of the number of records received per request.
- `loki_source_awsfirehose_invalid_static_labels_errors` (counter): Count number of errors while processing AWS Firehose static labels.

## Example

This example starts an HTTP server on `0.0.0.0` address and port `9999`. The server receives log entries and forwards
them to a `loki.write` component. The `loki.write` component will send the logs to the specified loki instance using
basic auth credentials provided.

```alloy
loki.write "local" {
    endpoint {
        url = "http://loki:3100/api/v1/push"
        basic_auth {
            username = "<your username>"
            password_file = "<your password file>"
        }
    }
}

loki.source.awsfirehose "loki_fh_receiver" {
    http {
        listen_address = "0.0.0.0"
        listen_port = 9999
    }
    forward_to = [
        loki.write.local.receiver,
    ]
}
```

As another example, if you are receiving records that originated from a CloudWatch logs subscription, you can enrich each
received entry by relabeling internal labels. The following configuration builds upon the one above but keeps the origin
log stream and group as `log_stream` and `log_group`, respectively.

```alloy
loki.write "local" {
    endpoint {
        url = "http://loki:3100/api/v1/push"
        basic_auth {
            username = "<your username>"
            password_file = "<your password file>"
        }
    }
}

loki.source.awsfirehose "loki_fh_receiver" {
    http {
        listen_address = "0.0.0.0"
        listen_port = 9999
    }
    forward_to = [
        loki.write.local.receiver,
    ]
    relabel_rules = loki.relabel.logging_origin.rules
}

loki.relabel "logging_origin" {
  rule {
    action = "replace"
    source_labels = ["__aws_cw_log_group"]
    target_label = "log_group"
  }
  rule {
    action = "replace"
    source_labels = ["__aws_cw_log_stream"]
    target_label = "log_stream"
  }
  forward_to = []
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.awsfirehose` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
