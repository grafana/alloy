---
canonical: https://grafana.com/docs/alloy/latest/collect/ecs-opentelemetry-data/
alias:
  - ./ecs-openteletry-data/ # /docs/alloy/latest/collect/ecs-openteletry-data/
description: Learn how to collect Amazon ECS or AWS Fargate OpenTelemetry data and forward it to any OpenTelemetry-compatible endpoint
menuTitle: Collect ECS or Fargate OpenTelemetry data
title: Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data
weight: 500
---

# Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data

You can configure {{< param "FULL_PRODUCT_NAME" >}} or AWS ADOT to collect OpenTelemetry-compatible data from Amazon Elastic Container Service (ECS) or AWS Fargate and forward it to any OpenTelemetry-compatible endpoint.

Metrics are available from various sources including ECS itself, the ECS instances when using EC2, X-Ray and your own application. You can also collect logs and traces from your applications instrumented for Prometheus or OTLP.

## Before you begin

* Ensure that you have basic familiarity with instrumenting applications with OpenTelemetry.
* Have an available Amazon ECS or AWS Fargate deployment.
* Identify where {{< param "PRODUCT_NAME" >}} writes received telemetry data.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Running a collector in a task

In this configuration, an OTEL collector is added to the task running your application and uses the ECS Metadata Endpoint to gather task and container metrics in your cluster.

You can choose between two collector implementations:

- AWS supports its own OpenTelemetry collector called ADOT. ADOT has native support for scraping task and container metrics. ADOT comes with default configurations that can be selected in the task definition.

- Alloy can also be used as a collector alongside the ECS shim which exports ECS stats as Prometheus metrics.

### Configuring ADOT

When using ADOT as a collector, you simply add a new container to your task definition and use a custom configuration defined in AWS SSM Parameter Store.

Sample OTEL configuration files can be found in the [AWS Observability repo][otel-templates]. Use those as a starting point and add the appropriate exporter configuration to send metrics to a Prometheus or Otel endpoint.

* Use [ecs-default-config] to consume StatsD metrics, OTLP metrics and traces, and AWS X-Ray SDK traces
* Use [otel-task-metrics-config] to consume StatsD, OTLP, AWS X-Ray, and Container Resource utilization metrics
* Use [otel-prometheus] to find out how to set the prometheus remote write (AWS managed prometheus in the example)

You can create a sample task by completing the following steps (inspired by [the official ADOT doc][adot-doc])

1. Create a SSM Parameter Store entry to hold the collector configuration file
Open the AWS Systems Manager console.

   1. In the AWS Console, choose Parameter Store.
   1. Choose Create parameter.
   1. Create a parameter with the following values:
      
      ```
      Name: collector-config
      Tier: Standard
      Type: String
      Data type: Text
      Value: Copy and paste your custom OpenTelemetry configuration file.
      ```

1. Download the [ECS Fargate][fargate-template] or [ECS EC2][ec2-template] task definition template from GitHub.
1. Edit the task definition template and add the following parameters.
   * `{{region}}`: The region to send the data to.
   * `{{ecsTaskRoleArn}}`: The AWSOTTaskRole ARN.
   * `{{ecsExecutionRoleArn}}`: The AWSOTTaskExcutionRole ARN.
   * Add an environment variable named AOT_CONFIG_CONTENT.
Select ValueFrom to tell ECS to get the value from the SSM Parameter, and set the value to collector-config (created above)
1. Follow the ECS Fargate setup instructions to [create a task definition][task] using the template.

### Configuring Alloy

Use the following as a starting point for your Alloy configuration:

```
prometheus.scrape "stats" {
  targets    = [
    { "__address__" = "localhost:9779" },
  ]
  metrics_path = "/metrics"
  scheme       = "http"
  forward_to   = [prometheus.remote_write.default.receiver]
}

// additional OTEL config as in [ecs-default-config]
// OTLP receiver
// statsd
// Use the alloy convert command to use one of the AWS ADOT files
// https://grafana.com/docs/alloy/latest/reference/cli/convert/
...

prometheus.remote_write "default" {
  endpoint {
    url = sys.env("PROMETHEUS_REMOTE_WRITE_URL")
      basic_auth {
        username = sys.env("PROMETHEUS_USERNAME")
        password = sys.env("PROMETHEUS_PASSWORD")
      }
  }
}
```

This sets up a scrape job for the container metrics and export them to a prometheus endpoint.

You can create a sample task by completing the following steps (inspired by [the official ADOT doc][adot-doc])

1. Create a SSM Parameter Store entry to hold the collector configuration file
Open the AWS Systems Manager console.

   1. In the AWS Console, choose Parameter Store.
   1. Choose Create parameter.
   1. Create a parameter with the following values:
      
      ```
      Name: collector-config
      Tier: Standard
      Type: String
      Data type: Text
      Value: Copy and paste your custom Alloy configuration file.
      ```

1. Download the [ECS Fargate][fargate-template] or [ECS EC2][ec2-template] task definition template from GitHub.
1. Edit the task definition template and add the following parameters.
   * `{{region}}`: The region to send the data to.
   * `{{ecsTaskRoleArn}}`: The AWSOTTaskRole ARN.
   * `{{ecsExecutionRoleArn}}`: The AWSOTTaskExcutionRole ARN.
   * Add an environment variable named ALLOY_CONFIG_CONTENT.
Select ValueFrom to tell ECS to get the value from the SSM Parameter, and set the value to collector-config (created above).
   * In the docker configuration, change the Entrypoint to `bash,-c`
   * `{{command}}`: `"echo \"$ALLOY_CONFIG_CONTENT\" > /tmp/config_file && exec alloy run --server.http.listen-addr=0.0.0.0:12345 /tmp/config_file"` *making sure you don't ommit the double quotes around the command*
   * Alloy doesn't currently support collecting container metrics from the ECS metadata endpoint directly, so you need to add a second container for the [prometheus exporter](https://github.com/prometheus-community/ecs_exporter) if needed:

      1. Add a new container to the task
      1. Set "ecs-exporter" as container name.
      1. Set "quay.io/prometheuscommunity/ecs-exporter:latest" as image
      1. Add tcp/9779 as a port mapping.
1. Follow the ECS Fargate setup instructions to [create a task definition][task] using the template.

## Collecting EC2 instance metrics

For ECS Clusters running on EC2, you can collect instance metrics by using AWS ADOT or Alloy in a separate ECS task deployed as a daemon.

### Alloy

You can follow the steps described in [Configure Alloy](#configuring-alloy) above with the following changes:

* Only add the Alloy container, not the prometheus exporter, and run the task as daemon, so it will automatically run one instance per node in your cluster.
* Update your Alloy configuration to collect metrics from the instance. Configuration varies depending on the type of EC2 node, refer to the [Alloy documentation](https://grafana.com/docs/alloy/latest/collect/) for details.

### ADOT

The approach described in [this document](https://aws-otel.github.io/docs/setup/ecs#3-setup-the-adot-collector-for-ecs-ec2-instance-metrics) uses the awscontainerinsightreceiver receiver from the OTEL contribs, included in ADOT out of the box.

Just like described in the [Configuring ADOT](#configuring-adot) section, you need to use a custom configuration SSM Parameter based on the [sample](https://github.com/aws-observability/aws-otel-collector/blob/main/config/ecs/otel-instance-metrics-config.yaml) config file in order to route the telemetry to your final destination.

## Collecting application telemetry

To collect telemetry emitted by your application, you can use the OTLP endpoints exposed by the collector side car containerm regardless of the collector implementation. Just use `localhost` as the host name.

For prometheus endpoints, add a scrape job to the ADOT or Alloy config, and use `localhost` and your service port and endpoint path.

## Logs

The easiest way to collect application logs in ECS is to leverage the [AWS firelens log driver][firelens-doc]. Depending on your use case, you can forward your logs to the collector container in your task using the [FluentBit plugin for OpenTelemetry][fluentbit-otel-plugin] or using the FluentBit Loki plugin. You can also send everything directly to your final destination.

[Components]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/get-started/components
[firelens-doc]: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/firelens-taskdef.html
[fluentbit-otel-plugin]: https://docs.fluentbit.io/manual/pipeline/outputs/opentelemetry
[otel-templates]: https://github.com/aws-observability/aws-otel-collector/tree/main/config/ecs
[otel-prometheus]: https://github.com/aws-observability/aws-otel-collector/blob/357f9c7b8896dba6ee0e03b8efd7ca7117024d2e/config/ecs/ecs-amp-xray-prometheus.yaml
[adot-doc]: https://aws-otel.github.io/docs/setup/ecs
[otel-task-metrics-config]: https://github.com/aws-observability/aws-otel-collector/blob/main/config/ecs/container-insights/
[ecs-default-config]: https://github.com/aws-observability/aws-otel-collector/blob/main/config/ecs/ecs-default-config.yaml
[fargate-template]: https://github.com/aws-observability/aws-otel-collector/blob/master/examples/ecs/aws-cloudwatch/ecs-fargate-sidecar.json
[ec2-template]: https://github.com/aws-observability/aws-otel-collector/blob/main/examples/ecs/aws-prometheus/ecs-fargate-task-def.json
[configure]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/configure/
[steps]: https://medium.com/ci-t/9-steps-to-ssh-into-an-aws-fargate-managed-container-46c1d5f834e2
[install]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/set-up/install/linux/
[deploy]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/set-up/deploy/
[task]: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html
[run]: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/standalone-task-create.html
