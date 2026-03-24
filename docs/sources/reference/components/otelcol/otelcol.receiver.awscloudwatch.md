---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.awscloudwatch/
description: Learn about otelcol.receiver.awscloudwatch
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.awscloudwatch
---

# `otelcol.receiver.awscloudwatch`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.awscloudwatch` receives logs from Amazon CloudWatch and forwards them to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.awscloudwatch` is a wrapper over the upstream OpenTelemetry Collector [`awscloudwatch`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`awscloudwatch`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awscloudwatchreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.awscloudwatch` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.awscloudwatch "<LABEL>" {
  region = "us-west-2"

  output {
    logs = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.awscloudwatch`:

| Name            | Type                       | Description                                                              | Default | Required |
|-----------------|----------------------------|--------------------------------------------------------------------------|---------|----------|
| `region`        | `string`                   | AWS region to collect logs from.                                         |         | yes      |
| `imds_endpoint` | `string`                   | Custom EC2 IMDS endpoint to use.                                         |         | no       |
| `profile`       | `string`                   | AWS credentials profile to use.                                          |         | no       |
| `storage`       | `capsule(otelcol.Handler)` | Handler from an `otelcol.storage` component to use for persisting state. |         | no       |

If `imds_endpoint` isn't specified, and the environment variable `AWS_EC2_METADATA_SERVICE_ENDPOINT` has a value, it will be used as the IMDS endpoint.

## Blocks

You can use the following blocks with `otelcol.receiver.awscloudwatch`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`logs`][logs]                   | Configures the log collection settings.                                    | no       |

[logs]: #logs
[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `logs`

The `logs` block configures how logs are collected from CloudWatch.

The following arguments are supported:

| Name                     | Type       | Description                                                    | Default | Required |
|--------------------------|------------|----------------------------------------------------------------|---------|----------|
| `max_events_per_request` | `int`      | Maximum number of events to process per request to CloudWatch. | `1000`  | no       |
| `poll_interval`          | `duration` | How frequently to poll for new log entries.                    | `"1m"`  | no       |
| `start_from`             | `string`   | Timestamp in RFC3339 format where to start reading logs.       | `""`    | no       |

The `logs` block supports the following blocks:

| Block                   | Description                                  | Required |
|-------------------------|----------------------------------------------|----------|
| [groups](#logs--groups) | Configures which log groups to collect from. | no       |

#### `logs` > `groups`

The `groups` block supports the following blocks:

| Block                                       | Description                                     | Required |
|---------------------------------------------|-------------------------------------------------|----------|
| [autodiscover](#logs--groups--autodiscover) | Configures automatic discovery of log groups.   | no       |
| [named](#logs--groups--named)               | Configures specific log groups to collect from. | no       |

The blocks `autodiscover` or `named` are mutually exclusive.

#### `logs` > `groups` > `autodiscover`

The `autodiscover` block configures automatic discovery of log groups.

The following arguments are supported:

| Name     | Type     | Description                               | Default | Required |
|----------|----------|-------------------------------------------|---------|----------|
| `limit`  | `int`    | Maximum number of log groups to discover. | `50`    | no       |
| `prefix` | `string` | Prefix to filter log groups by.           |         | no       |

The `autodiscover` block supports the following blocks:

| Block                                           | Description                       | Required |
|-------------------------------------------------|-----------------------------------|----------|
| [streams](#logs--groups--autodiscover--streams) | Configures log streams filtering. | no       |

#### `logs` > `groups` > `autodiscover` > `streams`

The `streams` block configures filtering of log streams for the autodiscovered log groups.

The following arguments are supported:

| Name       | Type       | Description                            | Default | Required |
|------------|------------|----------------------------------------|---------|----------|
| `names`    | `[]string` | List of exact stream names to collect. |         | no       |
| `prefixes` | `[]string` | List of prefixes to filter streams by. |         | no       |

#### `logs` > `groups` > `named`

The `named` block explicitly configures specific log groups to collect from. Multiple `named` blocks can be specified.

The following arguments are supported:

| Name         | Type       | Description                            | Required |
|--------------|------------|----------------------------------------|----------|
| `group_name` | `string`   | Name of the CloudWatch log group.      | yes      |
| `names`      | `[]string` | List of exact stream names to collect. | no       |
| `prefixes`   | `[]string` | List of prefixes to filter streams by. | no       |

## Exported fields

`otelcol.receiver.awscloudwatch` doesn't export any fields.

## Component health

`otelcol.receiver.awscloudwatch` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.awscloudwatch` doesn't expose any component-specific debug information.

## Example

The following example collects logs from specific EKS cluster log groups and forwards them through a batch processor:

```alloy
otelcol.receiver.awscloudwatch "default" {
  region = "us-west-2"

  logs {
    poll_interval = "3m"
    max_events_per_request = 5000

    groups {
      named {
        group_name = "/aws/eks/dev-cluster/cluster"
        names = ["api-gateway"]
      }
      named {
        group_name = "/aws/eks/prod-cluster/cluster"
        prefixes = ["app-", "service-"]
      }
    }
  }

  output {
    logs = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.awscloudwatch` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
