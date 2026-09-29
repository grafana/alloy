---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.cloudwatch/
aliases:
  - ../prometheus.exporter.cloudwatch/ # /docs/alloy/latest/reference/components/prometheus.exporter.cloudwatch/
description: Learn about prometheus.exporter.cloudwatch
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.cloudwatch
---

# `prometheus.exporter.cloudwatch`

The `prometheus.exporter.cloudwatch` component embeds [`yet-another-cloudwatch-exporter`][], letting you collect [Amazon CloudWatch metrics][] in a Prometheus-compatible format.

This component lets you scrape CloudWatch metrics in a set of configurations called _jobs_.
There are two kinds of jobs: [`discovery`][discovery] and [`static`][static].

[`yet-another-cloudwatch-exporter`]: https://github.com/prometheus-community/yet-another-cloudwatch-exporter
[Amazon CloudWatch metrics]: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/WhatIsCloudWatch.html
[discovery]: #discovery
[static]: #static
[metric]: #metric

## Authentication

{{< param "PRODUCT_NAME" >}} must run in an environment with access to AWS.
The exporter uses the [AWS SDK for Go][] and provides authentication via the [AWS default credential chain][].
However you acquire the credentials, the exporter requires the following permissions:

```text
"tag:GetResources",
"cloudwatch:GetMetricData",
"cloudwatch:GetMetricStatistics",
"cloudwatch:ListMetrics"
```

[Transit Gateway][] attachment (`tgwa`) metrics require the following AWS Identity and Access Management (IAM) permissions:

```text
"ec2:DescribeTags",
"ec2:DescribeInstances",
"ec2:DescribeRegions",
"ec2:DescribeTransitGateway*"
```

To discover tagged [API Gateway][] REST APIs, the exporter requires the following IAM permission:

```text
"apigateway:GET"
```

To discover tagged [Database Migration Service][] (DMS) replication instances and tasks, the exporter requires the following IAM permissions:

```text
"dms:DescribeReplicationInstances",
"dms:DescribeReplicationTasks"
```

To retrieve the AWS account alias, the exporter requires the following IAM permission:

```text
"iam:ListAccountAliases"
```

{{< param "PRODUCT_NAME" >}} adds the alias as an `account_alias` label on the exported metrics.
Without this permission, {{< param "PRODUCT_NAME" >}} logs a `Couldn't get account alias` warning on each scrape and omits the label.
Metric collection continues normally.

To use all of the component's features, use the following AWS IAM policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Stmt1674249227793",
      "Action": [
        "tag:GetResources",
        "cloudwatch:GetMetricData",
        "cloudwatch:GetMetricStatistics",
        "cloudwatch:ListMetrics",
        "ec2:DescribeTags",
        "ec2:DescribeInstances",
        "ec2:DescribeRegions",
        "ec2:DescribeTransitGateway*",
        "apigateway:GET",
        "dms:DescribeReplicationInstances",
        "dms:DescribeReplicationTasks",
        "iam:ListAccountAliases"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

[AWS SDK for Go]: https://aws.github.io/aws-sdk-go-v2/docs/getting-started/
[AWS default credential chain]: https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials
[Transit Gateway]: https://aws.amazon.com/transit-gateway/
[API Gateway]: https://aws.amazon.com/api-gateway/
[Database Migration Service]: https://aws.amazon.com/dms/

## Usage

```alloy
prometheus.exporter.cloudwatch "queues" {
    sts_region      = "us-east-2"
    discovery {
        type        = "AWS/SQS"
        regions     = ["us-east-2"]
        search_tags = {
            "scrape" = "true",
        }

        metric {
            name       = "NumberOfMessagesSent"
            statistics = ["Sum", "Average"]
            period     = "1m"
        }

        metric {
            name       = "NumberOfMessagesReceived"
            statistics = ["Sum", "Average"]
            period     = "1m"
        }
    }
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.cloudwatch`:

| Name                      | Type                | Description                                                                 | Default | Required |
| ------------------------- | ------------------- | --------------------------------------------------------------------------- | ------- | -------- |
| `sts_region`              | `string`            | AWS region to use when calling [STS][] for retrieving account information.  |         | yes      |
| `aws_sdk_version_v2`      | `bool`              | (Deprecated, no-op) Has no effect. AWS SDK for Go v2 is always used.        | `true`  | no       |
| `debug`                   | `bool`              | (Deprecated, no-op) Has no effect. Use the global log level instead.        | `false` | no       |
| `discovery_exported_tags` | `map(list(string))` | List of tags (value) per service (key) to export in all metrics.            | `{}`    | no       |
| `fips_disabled`           | `bool`              | Disable use of FIPS endpoints. Set to `false` to enable them in US regions. | `true`  | no       |
| `labels_snake_case`       | `bool`              | Output labels on metrics in snake case instead of camel case.               | `false` | no       |

If you define the `["name", "type"]` under `"AWS/EC2"` in the `discovery_exported_tags` argument, it exports the name and type tags and its values as labels in all metrics.
This affects all discovery jobs.

[STS]: https://docs.aws.amazon.com/STS/latest/APIReference/welcome.html

{{< admonition type="caution" >}}
Starting with {{< param "PRODUCT_NAME" >}} v1.16, the `aws_sdk_version_v2` argument is deprecated and has no effect. AWS SDK for Go v2 is always used.<br />
Remove this argument from your configuration. The argument will be removed in a future release.
{{< /admonition >}}

{{< admonition type="caution" >}}
The `debug` argument is deprecated and has no effect. CloudWatch exporter logging now follows the global {{< param "PRODUCT_NAME" >}} log level.<br />
Remove this argument from your configuration. The argument will be removed in a future release.
{{< /admonition >}}

## Blocks

You can use the following blocks with `prometheus.exporter.cloudwatch`:

{{< docs/alloy-config >}}

| Block                                      | Description                                               | Required |
| ------------------------------------------ | --------------------------------------------------------- | -------- |
| [`custom_namespace`][custom_namespace]     | Configures a custom namespace job.                        | no       |
| `custom_namespace` > [`metric`][metric]    | Configures a metric to scrape.                            | yes      |
| `custom_namespace` > [`role`][role]        | Configures the IAM roles to assume when scraping metrics. | no       |
| [`decoupled_scraping`][decoupled_scraping] | Configures background scraping on a schedule.             | no       |
| [`discovery`][discovery]                   | Configures a discovery job.                               | no       |
| `discovery` > [`metric`][metric]           | Configures a metric to scrape.                            | yes      |
| `discovery` > [`role`][role]               | Configures the IAM roles to assume when scraping metrics. | no       |
| [`static`][static]                         | Configures a static job.                                  | no       |
| `static` > [`metric`][metric]              | Configures a metric to scrape.                            | yes      |
| `static` > [`role`][role]                  | Configures the IAM roles to assume when scraping metrics. | no       |

You must configure at least one `discovery`, `static`, or `custom_namespace` block.
Each job block requires at least one `metric` block.

[discovery]: #discovery
[static]: #static
[custom_namespace]: #custom_namespace
[metric]: #metric
[role]: #role
[decoupled_scraping]: #decoupled_scraping

{{< /docs/alloy-config >}}

### `custom_namespace`

The `custom_namespace` block allows the component to scrape CloudWatch metrics from custom namespaces using only the namespace name and a list of metrics under that namespace.
For example:

```alloy
prometheus.exporter.cloudwatch "discover_instances" {
    sts_region = "eu-west-1"

    custom_namespace "customEC2Metrics" {
        namespace = "CustomEC2Metrics"
        regions   = ["us-east-1"]

        metric {
            name       = "cpu_usage_idle"
            statistics = ["Average"]
            period     = "5m"
        }

        metric {
            name       = "disk_free"
            statistics = ["Average"]
            period     = "5m"
        }
    }
}
```

You can configure the `custom_namespace` block multiple times to scrape metrics from different namespaces.

You can use the following arguments with the `custom_namespace` block:

| Name                          | Type           | Description                                                                                  | Default                          | Required |
| ----------------------------- | -------------- | -------------------------------------------------------------------------------------------- | -------------------------------- | -------- |
| `namespace`                   | `string`       | CloudWatch metric namespace.                                                                 |                                  | yes      |
| `regions`                     | `list(string)` | List of AWS regions.                                                                         |                                  | yes      |
| `custom_tags`                 | `map(string)`  | Key/value pairs to add as labels named `custom_tag_{key}`.                                   | `{}`                             | no       |
| `delay`                       | `duration`     | Delay the query start time by this duration.                                                 | `0`                              | no       |
| `period`                      | `duration`     | Default period for metrics in this job.                                                      | `5m`                             | no       |
| `length`                      | `duration`     | Default length for metrics in this job.                                                      | Derived from [`period`][period]. | no       |
| `dimension_name_requirements` | `list(string)` | Only query metrics with exactly these dimensions. If empty, all combinations are queried.    | `[]`                             | no       |
| `nil_to_zero`                 | `bool`         | Whether to convert `NaN` metric values to 0. The [`metric`][metric] block can override this. | `true`                           | no       |
| `recently_active_only`        | `bool`         | Whether to return only metrics active in the last 3 hours.                                   | `false`                          | no       |
| `add_cloudwatch_timestamp`    | `bool`         | Whether to use the CloudWatch timestamp instead of the scrape time.                          | `false`                          | no       |

### `metric`

{{< badge text="Required" >}}

The `metric` block defines an AWS metric to scrape.

You can configure the `metric` block multiple times to define multiple target metrics.
Refer to the [View available metrics][] topic in the Amazon CloudWatch documentation for detailed metrics information.

You can use the following arguments with the `metric` block:

| Name                       | Type           | Description                                                                          | Default                          | Required |
| -------------------------- | -------------- | ------------------------------------------------------------------------------------ | -------------------------------- | -------- |
| `name`                     | `string`       | Metric name.                                                                         |                                  | yes      |
| `statistics`               | `list(string)` | Statistics to scrape, for example, `"Minimum"` or `"Maximum"`.                       |                                  | yes      |
| `add_cloudwatch_timestamp` | `bool`         | Whether to use the CloudWatch timestamp instead of the scrape time.                  | `false`                          | no       |
| `length`                   | `duration`     | How far back in time to consider metrics. Refer to [`period` and `length`][period].  | Derived from [`period`][period]. | no       |
| `nil_to_zero`              | `bool`         | Whether to convert `NaN` metric values to 0.                                         | `true`                           | no       |
| `period`                   | `duration`     | Time bucket width for aggregating metrics. Refer to [`period` and `length`][period]. | `5m`                             | no       |

In [`custom_namespace`][custom_namespace] and [`discovery`][discovery] blocks, these arguments default to the parent block's value when the parent sets one.

[custom_namespace]: #custom_namespace
[period]: #period-and-length
[View available metrics]: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/viewing_metrics_with_cloudwatch.html

#### `period` and `length`

`period` sets the width of the time bucket that CloudWatch uses to aggregate metrics.
`length` sets how far back in time each {{< param "PRODUCT_NAME" >}} scrape looks for CloudWatch metrics.
When you set both, {{< param "PRODUCT_NAME" >}} calls the CloudWatch APIs as follows:

{{< figure src="/media/docs/alloy/cloudwatch-period-and-length-time-model-2.png" alt="An example of a CloudWatch period and length time model" >}}

When metrics in the same `static` or `discovery` job use different `period` or `length` values, {{< param "PRODUCT_NAME" >}} takes the minimum of all periods and the maximum of all lengths.

When you don't set `length`, {{< param "PRODUCT_NAME" >}} derives both the period and the length from the required `period` attribute.

When every metric in a job uses the same `period` value, {{< param "PRODUCT_NAME" >}} requests metrics from the scrape time back to `period` seconds earlier.
It then exports those values to Prometheus.

{{< figure src="/media/docs/alloy/cloudwatch-single-period-time-model.png" alt="An example of a CloudWatch single period and time model" >}}

When metrics in one job use different `period` values, the behavior differs.
{{< param "PRODUCT_NAME" >}} first aggregates all periods into two values: `length` takes the maximum of all periods, and `period` takes the minimum.
It then requests metrics from `now - length` to `now`, aggregating each into samples of `period` seconds, and exports the most recent sample for each metric to Prometheus.

{{< figure src="/media/docs/alloy/cloudwatch-multiple-period-time-model.png" alt="An example of a CloudWatch multiple period and time model" >}}

### `role`

The `role` block defines an [AWS IAM Role][].
If you omit this block, {{< param "PRODUCT_NAME" >}} uses the AWS role that corresponds to the credentials configured in the environment.

Multiple roles can be useful when scraping metrics from different AWS accounts with a single pair of credentials.
In this case, configure a different role for {{< param "PRODUCT_NAME" >}} to assume before it calls AWS APIs.
Therefore, the credentials configured in the system need permission to assume the target role.
Refer to [Granting a user permissions to switch roles][] in the AWS IAM documentation for more information about how to configure this.

You can use the following arguments with the `role` block:

| Name          | Type     | Description                                                                          | Default | Required |
| ------------- | -------- | ------------------------------------------------------------------------------------ | ------- | -------- |
| `role_arn`    | `string` | Amazon Resource Name (ARN) of the IAM role to assume for AWS API calls.              |         | yes      |
| `external_id` | `string` | External ID for the STS AssumeRole API call. Refer to the [IAM User Guide][details]. | `""`    | no       |

[AWS IAM Role]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html
[Granting a user permissions to switch roles]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_permissions-to-switch.html
[details]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user_externalid.html

### `decoupled_scraping`

The `decoupled_scraping` block configures an optional feature that scrapes CloudWatch metrics in the background on a scheduled interval.
When this feature is enabled, CloudWatch metrics are gathered asynchronously at the scheduled interval instead of synchronously when the CloudWatch component is scraped.

The decoupled scraping feature reduces the number of API requests sent to AWS.
This feature also prevents component scrape timeouts when you gather high volumes of CloudWatch metrics.

You can use the following arguments with the `decoupled_scraping` block:

| Name              | Type       | Description                                                             | Default | Required |
| ----------------- | ---------- | ----------------------------------------------------------------------- | ------- | -------- |
| `enabled`         | `bool`     | Whether to enable decoupled scraping.                                   | `false` | no       |
| `scrape_interval` | `duration` | How frequently to gather CloudWatch metrics asynchronously.             | `5m`    | no       |

### `discovery`

The `discovery` block allows the component to scrape CloudWatch metrics with only the AWS service and a list of metrics under that service/namespace.
{{< param "PRODUCT_NAME" >}} finds AWS resources in the specified service, scrapes the metrics, labels them appropriately, and exports them to Prometheus.
The following example configuration, shows you how to scrape CPU utilization and network traffic metrics from all AWS EC2 instances:

```alloy
prometheus.exporter.cloudwatch "discover_instances" {
    sts_region = "us-east-2"

    discovery {
        type    = "AWS/EC2"
        regions = ["us-east-2"]

        metric {
            name       = "CPUUtilization"
            statistics = ["Average"]
            period     = "5m"
        }

        metric {
            name       = "NetworkPacketsIn"
            statistics = ["Average"]
            period     = "5m"
        }
    }
}
```

You can configure the `discovery` block one or multiple times to scrape metrics from different services or with different `search_tags`.

You can use the following arguments with the `discovery` block:

| Name                          | Type           | Description                                                                                  | Default                          | Required |
| ----------------------------- | -------------- | -------------------------------------------------------------------------------------------- | -------------------------------- | -------- |
| `regions`                     | `list(string)` | List of AWS regions.                                                                         |                                  | yes      |
| `type`                        | `string`       | CloudWatch namespace name. Refer to [supported-services][] for the complete list.            |                                  | yes      |
| `custom_tags`                 | `map(string)`  | Key/value pairs to add as labels named `custom_tag_{key}`.                                   | `{}`                             | no       |
| `dimension_name_requirements` | `list(string)` | Only query metrics with exactly these dimensions. If empty, all combinations are queried.    | `[]`                             | no       |
| `delay`                       | `duration`     | Delay the query start time by this duration.                                                 | `0`                              | no       |
| `period`                      | `duration`     | Default period for metrics in this job.                                                      | `5m`                             | no       |
| `length`                      | `duration`     | Default length for metrics in this job.                                                      | Derived from [`period`][period]. | no       |
| `nil_to_zero`                 | `bool`         | Whether to convert `NaN` metric values to 0. The [`metric`][metric] block can override this. | `true`                           | no       |
| `recently_active_only`        | `bool`         | Whether to return only metrics active in the last 3 hours.                                   | `false`                          | no       |
| `search_tags`                 | `map(string)`  | Key/value pairs to filter by tag. All must match. Values can be regular expressions.         | `{}`                             | no       |
| `add_cloudwatch_timestamp`    | `bool`         | Whether to use the CloudWatch timestamp instead of the scrape time.                          | `false`                          | no       |

{{< admonition type="note" >}}
Don't use CloudWatch service aliases such as `alb` or `ec2`.
Use the namespace name instead, for example, `AWS/ApplicationELB` or `AWS/EC2`.
{{< param "PRODUCT_NAME" >}} logs a warning each time it converts an alias to a namespace.
{{< /admonition >}}

[supported-services]: #supported-services-in-discovery-jobs

### `static`

The `static` block configures the component to scrape a specific set of CloudWatch metrics.
The metrics need to be fully qualified with the following specifications:

1. `namespace`: For example, `AWS/EC2`, `AWS/EBS`, `CoolApp` if it were a custom metric, etc.
2. `dimensions`: CloudWatch identifies a metric by a set of dimensions, which are essentially label / value pairs.
   For example, all `AWS/EC2` metrics are identified by the `InstanceId` dimension and the identifier itself.
3. `metric`: Metric name and statistics.

The following example configuration shows you how to scrape the same metrics in the discovery example, but for a specific AWS EC2 instance:

```alloy
prometheus.exporter.cloudwatch "static_instances" {
    sts_region = "us-east-2"

    static "instances" {
        regions    = ["us-east-2"]
        namespace  = "AWS/EC2"
        dimensions = {
            "InstanceId" = "i01u29u12ue1u2c",
        }

        metric {
            name       = "CPUUsage"
            statistics = ["Sum", "Average"]
            period     = "1m"
        }
    }
}
```

You must give each `static` block a label, which becomes the `name` label in the exported metric.

```alloy
static "<LABEL>" {
    regions    = ["us-east-2"]
    namespace  = "AWS/EC2"
    // ...
}
```

You can configure the `static` block one or multiple times to scrape metrics with different sets of `dimensions`.

You can use the following arguments with the `static` block:

| Name          | Type           | Description                                                                                  | Default | Required |
| ------------- | -------------- | -------------------------------------------------------------------------------------------- | ------- | -------- |
| `dimensions`  | `map(string)`  | CloudWatch dimensions as name/value pairs. Must uniquely define all metrics in this job.     |         | yes      |
| `namespace`   | `string`       | CloudWatch metric namespace.                                                                 |         | yes      |
| `regions`     | `list(string)` | List of AWS regions.                                                                         |         | yes      |
| `custom_tags` | `map(string)`  | Key/value pairs to add as labels named `custom_tag_{key}`.                                   | `{}`    | no       |
| `nil_to_zero` | `bool`         | Whether to convert `NaN` metric values to 0. The [`metric`][metric] block can override this. | `true`  | no       |

Setting `period`, `length`, or `delay` on a `static` block has no effect.
Configure these arguments on each `metric` block instead.

Specify all dimensions when you scrape single metrics, as in the preceding example.
For example, `AWS/Logs` metrics require the `Resource`, `Service`, `Class`, and `Type` dimensions.
The same applies to CloudWatch custom metrics: specify every dimension attached to the metric in CloudWatch.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.cloudwatch` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.cloudwatch` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.cloudwatch` doesn't expose any component-specific debug metrics.

## Example

For detailed examples, refer to the [`discovery`][discovery] and [`static`][static] sections.

## Supported services in discovery jobs

The following AWS services are supported in `cloudwatch_exporter` discovery jobs.
When you configure a discovery job, make sure the `type` field of each `discovery_job` matches the desired job namespace.

{{< docs/shared lookup="reference/components/prometheus-exporter-cloudwatch-supported-services.md" source="alloy" version="<ALLOY_VERSION>" >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.cloudwatch` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
