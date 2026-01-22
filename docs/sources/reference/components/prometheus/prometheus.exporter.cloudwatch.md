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
There are two kinds of jobs: [discovery][] and [static][].

[`yet-another-cloudwatch-exporter`]: https://github.com/prometheus-community/yet-another-cloudwatch-exporter
[Amazon CloudWatch metrics]: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/WhatIsCloudWatch.html

## Authentication

{{< param "PRODUCT_NAME" >}} must be running in an environment with access to AWS.
The exporter uses the [AWS SDK for Go](https://aws.github.io/aws-sdk-go-v2/docs/getting-started/) and provides authentication via the [AWS default credential chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).
Regardless of the method used to acquire the credentials, some permissions are required for the exporter to work.

```text
"tag:GetResources",
"cloudwatch:GetMetricData",
"cloudwatch:GetMetricStatistics",
"cloudwatch:ListMetrics"
```

The following IAM permissions are required for the [Transit Gateway](https://aws.amazon.com/transit-gateway/) attachment (`tgwa`) metrics to work.

```text
"ec2:DescribeTags",
"ec2:DescribeInstances",
"ec2:DescribeRegions",
"ec2:DescribeTransitGateway*"
```

The following IAM permission is required to discover tagged [API Gateway](https://aws.amazon.com/es/api-gateway/) REST APIs:

```text
"apigateway:GET"
```

The following IAM permissions are required to discover tagged [Database Migration Service](https://aws.amazon.com/dms/) (DMS) replication instances and tasks:

```text
"dms:DescribeReplicationInstances",
"dms:DescribeReplicationTasks"
```

To use all of the integration features, use the following AWS IAM Policy:

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
        "dms:DescribeReplicationTasks"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

## Usage

```alloy
prometheus.exporter.cloudwatch "queues" {
    sts_region      = "us-east-2"
    aws_sdk_version_v2 = false
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

| Name                      | Type                | Description                                                                    | Default | Required |
| ------------------------- | ------------------- | ------------------------------------------------------------------------------ | ------- | -------- |
| `sts_region`              | `string`            | AWS region to use when calling [STS][] for retrieving account information.     |         | yes      |
| `aws_sdk_version_v2`      | `bool`              | Use AWS SDK version 2.                                                         | `false` | no       |
| `fips_disabled`           | `bool`              | Disable use of FIPS endpoints. Set 'true' when running outside of USA regions. | `true`  | no       |
| `debug`                   | `bool`              | Enable debug logging on CloudWatch exporter internals.                         | `false` | no       |
| `discovery_exported_tags` | `map(list(string))` | List of tags (value) per service (key) to export in all metrics.               | `{}`    | no       |
| `labels_to_snake_case`    | `bool`              | Output labels on metrics in snake case instead of camel case.                  | `false` | no       |

If you define the `["name", "type"]` under `"AWS/EC2"` in the `discovery_exported_tags` argument, it exports the name and type tags and its values as labels in all metrics.
This affects all discovery jobs.

[STS]: https://docs.aws.amazon.com/STS/latest/APIReference/welcome.html

## Blocks

You can use the following blocks with `prometheus.exporter.cloudwatch`:

| Name                                       | Description                                                                                                                                                | Required |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| [`discovery`][discovery]                   | Configures a discovery job. You can configure multiple jobs.                                                                                               | no\*     |
| `discovery` > [`role`][role]               | Configures the IAM roles the job should assume to scrape metrics. Defaults to the role configured in the environment {{< param "PRODUCT_NAME" >}} runs on. | no       |
| `discovery` > [`metric`][metric]           | Configures the list of metrics the job should scrape. You can define multiple metrics inside one job.                                                      | yes      |
| [`static`][static]                         | Configures a static job. You can configure multiple jobs.                                                                                                  | no\*     |
| `static` > [`role`][role]                  | Configures the IAM roles the job should assume to scrape metrics. Defaults to the role configured in the environment {{< param "PRODUCT_NAME" >}} runs on. | no       |
| `static` > [`metric`][metric]              | Configures the list of metrics the job should scrape. You can define multiple metrics inside one job.                                                      | yes      |
| [`custom_namespace`][custom_namespace]     | Configures a custom namespace job. You can configure multiple jobs.                                                                                        | no\*     |
| `custom_namespace` > [`role`][role]        | Configures the IAM roles the job should assume to scrape metrics. Defaults to the role configured in the environment {{< param "PRODUCT_NAME" >}} runs on. | no       |
| `custom_namespace` > [`metric`][metric]    | Configures the list of metrics the job should scrape. You can define multiple metrics inside one job.                                                      | yes      |
| [`decoupled_scraping`][decoupled_scraping] | Configures the decoupled scraping feature to retrieve metrics on a schedule and return the cached metrics.                                                 | no       |

The > symbol indicates deeper levels of nesting.
For example, `discovery` > `role` refers to a `role` block defined inside a `discovery` block.

{{< admonition type="note" >}}
The `static`, `discovery`, and `custom_namespace` blocks are marked as not required, but you must configure at least one `static`, `discovery`, or `custom_namespace` job.
{{< /admonition >}}

[discovery]: #discovery
[static]: #static
[custom_namespace]: #custom_namespace
[metric]: #metric
[role]: #role
[decoupled_scraping]: #decoupled_scraping

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

| Name                          | Type           | Description                                                                                                                                                                                                                                            | Default                                                        | Required |
| ----------------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------- | -------- |
| `regions`                     | `list(string)` | List of AWS regions.                                                                                                                                                                                                                                   |                                                                | yes      |
| `type`                        | `string`       | CloudWatch service alias (`"alb"`, `"ec2"`, etc) or namespace name (`"AWS/EC2"`, `"AWS/S3"`, etc). Refer to [supported-services][] for a complete list.                                                                                                |                                                                | yes      |
| `custom_tags`                 | `map(string)`  | Custom tags to be added as a list of key / value pairs. When exported to Prometheus format, the label name follows the following format: `custom_tag_{key}`.                                                                                           | `{}`                                                           | no       |
| `dimension_name_requirements` | `list(string)` | List of metric dimensions to query. Before querying metric values, the total list of metrics are filtered to only those that contain exactly this list of dimensions. If the list is empty or undefined, all dimension combinations are included. | `{}`                                                           | no       |
| `delay`                       | `duration`     | Delay the start time of the CloudWatch metrics query by this duration.                                                                                                                                                                                 | `0`                                                            | no       |
| `period`                      | `duration`     | Default period for metrics in this job.                                                                                                                                                                                                                | `5m`                                                           | no       |
| `length`                      | `duration`     | Default length for metrics in this job.                                                                                                                                                                                                                | Calculated based on `period`. Refer to [period][] for details. | no       |
| `nil_to_zero`                 | `bool`         | When `true`, `NaN` metric values are converted to 0. Individual metrics can override this value in the [metric][] block.                                                                                                                               | `true`                                                         | no       |
| `recently_active_only`        | `bool`         | Only return metrics that have been active in the last 3 hours.                                                                                                                                                                                         | `false`                                                        | no       |
| `search_tags`                 | `map(string)`  | List of key/value pairs to use for tag filtering. All must match. The value can be a regular expression.                                                                                                                                            | `{}`                                                           | no       |
| `add_cloudwatch_timestamp`    | `bool`         | When `true`, use the timestamp from CloudWatch instead of the scrape time.                                                                                                                                                                             | `false`                                                        | no       |

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

As shown above, `static` blocks must be specified with a label, which translates to the `name` label in the exported metric.

```alloy
static "<LABEL>" {
    regions    = ["us-east-2"]
    namespace  = "AWS/EC2"
    // ...
}
```

You can configure the `static` block one or multiple times to scrape metrics with different sets of `dimensions`.

| Name          | Type           | Description                                                                                                                                                  | Default | Required |
| ------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- | -------- |
| `dimensions`  | `map(string)`  | CloudWatch metric dimensions as a list of name / value pairs. Must uniquely define all metrics in this job.                                                  |         | yes      |
| `namespace`   | `string`       | CloudWatch metric namespace.                                                                                                                                 |         | yes      |
| `regions`     | `list(string)` | List of AWS regions.                                                                                                                                         |         | yes      |
| `custom_tags` | `map(string)`  | Custom tags to be added as a list of key / value pairs. When exported to Prometheus format, the label name follows the following format: `custom_tag_{key}`. | `{}`    | no       |
| `nil_to_zero` | `bool`         | When `true`, `NaN` metric values are converted to 0. Individual metrics can override this value in the [metric][] block.                                     | `true`  | no       |

All dimensions must be specified when scraping single metrics like the example above.
For example, `AWS/Logs` metrics require `Resource`, `Service`, `Class`, and `Type` dimensions to be specified.
The same applies to CloudWatch custom metrics, all dimensions attached to a metric when saved in CloudWatch are required.

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

| Name                          | Type           | Description                                                                                                                                                                                                                                            | Default                                                        | Required |
| ----------------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------- | -------- |
| `namespace`                   | `string`       | CloudWatch metric namespace.                                                                                                                                                                                                                           |                                                                | yes      |
| `regions`                     | `list(string)` | List of AWS regions.                                                                                                                                                                                                                                   |                                                                | yes      |
| `custom_tags`                 | `map(string)`  | Custom tags to be added as a list of key/value pairs. When exported to Prometheus format, the label name follows the following format: `custom_tag_{key}`.                                                                                           | `{}`                                                           | no       |
| `delay`                       | `duration`     | Delay the start time of the CloudWatch metrics query by this duration.                                                                                                                                                                                 | `0`                                                            | no       |
| `period`                      | `duration`     | Default period for metrics in this job.                                                                                                                                                                                                                | `5m`                                                           | no       |
| `length`                      | `duration`     | Default length for metrics in this job.                                                                                                                                                                                                                | Calculated based on `period`. Refer to [period][] for details. | no       |
| `dimension_name_requirements` | `list(string)` | List of metric dimensions to query. Before querying metric values, the total list of metrics are filtered to only those that contain exactly this list of dimensions. If the list is empty or undefined, all dimension combinations are included. | `{}`                                                           | no       |
| `nil_to_zero`                 | `bool`         | When `true`, `NaN` metric values are converted to 0. Individual metrics can override this value in the [metric][] block.                                                                                                                               | `true`                                                         | no       |
| `recently_active_only`        | `bool`         | Only return metrics that have been active in the last 3 hours.                                                                                                                                                                                         | `false`                                                        | no       |
| `add_cloudwatch_timestamp`    | `bool`         | When `true`, use the timestamp from CloudWatch instead of the scrape time.                                                                                                                                                                             | `false`                                                        | no       |

### `metric`

{{< badge text="Required" >}}

Represents an AWS Metric to scrape.

The `metric` block may be specified multiple times to define multiple target metrics.
Refer to the [View available metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/viewing_metrics_with_cloudwatch.html) topic in the Amazon CloudWatch documentation for detailed metrics information.

| Name                       | Type           | Description                                                                | Default                                                                                                            | Required |
| -------------------------- | -------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | -------- |
| `name`                     | `string`       | Metric name.                                                               |                                                                                                                    | yes      |
| `period`                   | `duration`     | Refer to the [period][] section below.                                     | The value of `period` in the parent job.                                                                           | no       |
| `statistics`               | `list(string)` | List of statistics to scrape. For example, `"Minimum"`, `"Maximum"`, etc.  |                                                                                                                    | yes      |
| `add_cloudwatch_timestamp` | `bool`         | When `true`, use the timestamp from CloudWatch instead of the scrape time. | The value of `add_cloudwatch_timestamp` in the parent job.                                                         | no       |
| `length`                   | `duration`     | Refer to the [period][] section below.                                     | The value of `length` in the parent job.                                                                           | no       |
| `nil_to_zero`              | `bool`         | When `true`, `NaN` metric values are converted to 0.                       | The value of `nil_to_zero` in the parent [static][] or [discovery][] block. `true` if not set in the parent block. | no       |

[period]: #period-and-length

#### `period` and `length`

`period` controls primarily the width of the time bucket used for aggregating metrics collected from CloudWatch.
`length` controls how far back in time CloudWatch metrics are considered during each {{< param "PRODUCT_NAME" >}} scrape.
If both settings are configured, the time parameters when calling CloudWatch APIs works as follows:

{{< figure src="/media/docs/alloy/cloudwatch-period-and-length-time-model-2.png" alt="An example of a CloudWatch period and length time model" >}}

If, across multiple metrics under the same static or discovery job, there's a different `period` or `length`, the minimum of all periods, and maximum of all lengths is configured.

On the other hand, if `length` isn't configured, both period and length settings are calculated based on the required `period` configuration attribute.

If all metrics within a job (discovery or static) have the same `period` value configured, CloudWatch APIs are requested for metrics from the scrape time, to `period` seconds in the past.
The values of these are exported to Prometheus.

{{< figure src="/media/docs/alloy/cloudwatch-single-period-time-model.png" alt="An example of a CloudWatch single period and time model" >}}

On the other hand, if metrics with different `period`s are configured under an individual job, this works differently.
First, two variables are calculated aggregating all periods: `length`, taking the maximum value of all periods, and the new `period` value, taking the minimum of all periods.
Then, CloudWatch APIs are requested for metrics from `now - length` to `now`, aggregating each in samples for `period` seconds. For each metric, the most recent sample is exported to CloudWatch.

{{< figure src="/media/docs/alloy/cloudwatch-multiple-period-time-model.png" alt="An example of a CloudWatch multiple period and time model" >}}

### `role`

Represents an [AWS IAM Role](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html).
If omitted, the AWS role that corresponds to the credentials configured in the environment is used.

Multiple roles can be useful when scraping metrics from different AWS accounts with a single pair of credentials.
In this case, a different role is configured for {{< param "PRODUCT_NAME" >}} to assume before calling AWS APIs.
Therefore, the credentials configured in the system need permission to assume the target role.
Refer to [Granting a user permissions to switch roles](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_permissions-to-switch.html) in the AWS IAM documentation for more information about how to configure this.

| Name          | Type     | Description                                                                                                    | Default | Required |
| ------------- | -------- | -------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `external_id` | `string` | External ID used when calling STS AssumeRole API. Refer to the [IAM User Guide][details] for more information. | `""`    | no       |
| `role_arn`    | `string` | AWS IAM Role ARN the exporter should assume to perform AWS API calls.                                          |         | yes      |

[details]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user_externalid.html

### `decoupled_scraping`

The `decoupled_scraping` block configures an optional feature that scrapes CloudWatch metrics in the background on a scheduled interval.
When this feature is enabled, CloudWatch metrics are gathered asynchronously at the scheduled interval instead of synchronously when the CloudWatch component is scraped.

The decoupled scraping feature reduces the number of API requests sent to AWS.
This feature also prevents component scrape timeouts when you gather high volumes of CloudWatch metrics.

| Name              | Type     | Description                                                             | Default | Required |
| ----------------- | -------- | ----------------------------------------------------------------------- | ------- | -------- |
| `enabled`         | `bool`   | Controls whether the decoupled scraping featured is enabled             | false   | no       |
| `scrape_interval` | `string` | Controls how frequently to asynchronously gather new CloudWatch metrics | 5m      | no       |

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

For detailed examples, refer to the [discovery][] and [static] sections.

## Supported services in discovery jobs

The following AWS services are supported in `cloudwatch_exporter` discovery jobs.
When you configure a discovery job, make sure the `type` field of each `discovery_job` matches the desired job namespace.

{{< column-list >}}

* Namespace: `/aws/sagemaker/Endpoints`
* Namespace: `/aws/sagemaker/InferenceRecommendationsJobs`
* Namespace: `/aws/sagemaker/ProcessingJobs`
* Namespace: `/aws/sagemaker/TrainingJobs`
* Namespace: `/aws/sagemaker/TransformJobs`
* Namespace: `AmazonMWAA`
* Namespace: `AWS/ACMPrivateCA`
* Namespace: `AWS/AmazonMQ`
* Namespace: `AWS/AOSS`
* Namespace: `AWS/ApiGateway`
* Namespace: `AWS/ApplicationELB`
* Namespace: `AWS/AppRunner`
* Namespace: `AWS/AppStream`
* Namespace: `AWS/AppSync`
* Namespace: `AWS/Athena`
* Namespace: `AWS/AutoScaling`
* Namespace: `AWS/Backup`
* Namespace: `AWS/Bedrock`
* Namespace: `AWS/Billing`
* Namespace: `AWS/Cassandra`
* Namespace: `AWS/CertificateManager`
* Namespace: `AWS/ClientVPN`
* Namespace: `AWS/CloudFront`
* Namespace: `AWS/Cognito`
* Namespace: `AWS/DataSync`
* Namespace: `AWS/DDoSProtection`
* Namespace: `AWS/DMS`
* Namespace: `AWS/DocDB`
* Namespace: `AWS/DX`
* Namespace: `AWS/DynamoDB`
* Namespace: `AWS/EBS`
* Namespace: `AWS/EC2`
* Namespace: `AWS/EC2CapacityReservations`
* Namespace: `AWS/EC2Spot`
* Namespace: `AWS/ECR`
* Namespace: `AWS/ECS`
* Namespace: `AWS/EFS`
* Namespace: `AWS/ElastiCache`
* Namespace: `AWS/ElasticBeanstalk`
* Namespace: `AWS/ElasticMapReduce`
* Namespace: `AWS/ELB`
* Namespace: `AWS/EMRServerless`
* Namespace: `AWS/ES`
* Namespace: `AWS/Events`
* Namespace: `AWS/Firehose`
* Namespace: `AWS/FSx`
* Namespace: `AWS/GameLift`
* Namespace: `AWS/GatewayELB`
* Namespace: `AWS/GlobalAccelerator`
* Namespace: `AWS/IoT`
* Namespace: `AWS/IPAM`
* Namespace: `AWS/Kafka`
* Namespace: `AWS/KafkaConnect`
* Namespace: `AWS/Kinesis`
* Namespace: `AWS/KinesisAnalytics`
* Namespace: `AWS/KMS`
* Namespace: `AWS/Lambda`
* Namespace: `AWS/Logs`
* Namespace: `AWS/MediaConnect`
* Namespace: `AWS/MediaConvert`
* Namespace: `AWS/MediaLive`
* Namespace: `AWS/MediaPackage`
* Namespace: `AWS/MediaTailor`
* Namespace: `AWS/MemoryDB`
* Namespace: `AWS/MWAA`
* Namespace: `AWS/NATGateway`
* Namespace: `AWS/Neptune`
* Namespace: `AWS/Network Manager`
* Namespace: `AWS/NetworkELB`
* Namespace: `AWS/NetworkFirewall`
* Namespace: `AWS/PrivateLinkEndpoints`
* Namespace: `AWS/PrivateLinkServices`
* Namespace: `AWS/Prometheus`
* Namespace: `AWS/QLDB`
* Namespace: `AWS/QuickSight`
* Namespace: `AWS/RDS`
* Namespace: `AWS/Redshift`
* Namespace: `AWS/Route53`
* Namespace: `AWS/Route53Resolver`
* Namespace: `AWS/RUM`
* Namespace: `AWS/S3`
* Namespace: `AWS/SageMaker`
* Namespace: `AWS/Sagemaker/ModelBuildingPipeline`
* Namespace: `AWS/Scheduler`
* Namespace: `AWS/SecretsManager`
* Namespace: `AWS/SES`
* Namespace: `AWS/SNS`
* Namespace: `AWS/SQS`
* Namespace: `AWS/States`
* Namespace: `AWS/StorageGateway`
* Namespace: `AWS/Timestream`
* Namespace: `AWS/TransitGateway`
* Namespace: `AWS/TrustedAdvisor`
* Namespace: `AWS/Usage`
* Namespace: `AWS/VpcLattice`
* Namespace: `AWS/VPN`
* Namespace: `AWS/WAFV2`
* Namespace: `AWS/WorkSpaces`
* Namespace: `ContainerInsights`
* Namespace: `CWAgent`
* Namespace: `ECS/ContainerInsights`
* Namespace: `Glue`

{{< /column-list >}}


<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.cloudwatch` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
