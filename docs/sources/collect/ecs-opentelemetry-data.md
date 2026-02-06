---
canonical: https://grafana.com/docs/alloy/latest/collect/ecs-opentelemetry-data/
aliases:
  - ./ecs-opentelemetry-data/ # /docs/alloy/latest/collect/ecs-opentelemetry-data/
description: Learn how to collect Amazon ECS or AWS Fargate OpenTelemetry data and forward it to any OpenTelemetry-compatible endpoint
menuTitle: Collect ECS or Fargate OpenTelemetry data
title: Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data
weight: 500
---

# Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data

You can configure {{< param "FULL_PRODUCT_NAME" >}} or AWS ADOT to collect OpenTelemetry-compatible data from Amazon Elastic Container Service (ECS) or AWS Fargate and forward it to any OpenTelemetry-compatible endpoint.

Metrics are available from various sources including ECS itself, the ECS instances when using EC2, X-Ray, and your own application.
You can also collect logs and traces from your applications instrumented for Prometheus or OTLP.

1. [Collect task and container metrics](#collect-task-and-container-metrics)
1. [Collect application telemetry](#collect-application-telemetry)
1. [Collect EC2 instance metrics](#collect-ec2-instance-metrics)
1. [Collect application logs](#collect-logs)

## Before you begin

- Ensure that you have basic familiarity with instrumenting applications with OpenTelemetry.
- Have an available Amazon ECS or AWS Fargate deployment.
- Identify where {{< param "PRODUCT_NAME" >}} writes received telemetry data.
- Familiarize yourself with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Collect task and container metrics

In this configuration, you add an OpenTelemetry Collector to the task running your application, and it uses the ECS Metadata Endpoint to gather task and container metrics in your cluster.

You can choose between two collector implementations:

- You can use ADOT, the AWS OpenTelemetry collector. ADOT has native support for scraping task and container metrics. ADOT comes with default configurations that you can select in the task definition.

- You can use {{< param "PRODUCT_NAME" >}} with either the [`otelcol.receiver.awsecscontainermetrics`][awsecscontainermetrics] component or the [Prometheus ECS exporter](https://github.com/prometheus-community/ecs_exporter) sidecar.

### Configure ADOT

If you use ADOT as a collector, add a container to your task definition and use a custom configuration you define in your AWS Systems Manager Parameter Store.

You can find sample OpenTelemetry configuration files in the [AWS Observability repository][otel-templates].
You can use these samples as a starting point and add the appropriate exporter configuration to send metrics to a Prometheus or OpenTelemetry endpoint.

- Use [`ecs-default-config`][ecs-default-config] to consume StatsD metrics, OTLP metrics and traces, and AWS X-Ray SDK traces.
- Use [`otel-task-metrics-config`][otel-task-metrics-config] to consume StatsD, OTLP, AWS X-Ray, and Container Resource utilization metrics.

Read [`otel-prometheus`][otel-prometheus] to find out how to set the Prometheus remote write. The example uses AWS managed Prometheus.

Complete the following steps to create a sample task. Refer to the [ADOT doc][adot-doc] for more information.

1. Create a Parameter Store entry to hold the collector configuration file.

   1. Open the AWS Console.
   1. In the AWS Console, choose **Parameter Store**.
   1. Choose **Create parameter**.
   1. Create a parameter with the following values:

      - **Name:** `collector-config`
      - **Tier:** Standard
      - **Type:** String
      - **Data type:** Text
      - **Value:** Copy and paste your custom OpenTelemetry configuration file.

1. Download the [ECS Fargate][fargate-template] or [ECS EC2][ec2-template] task definition template from GitHub.
1. Edit the task definition template and add the following parameters.

   - _`{{region}}`_: The region to send the data to.
   - _`{{ecsTaskRoleArn}}`_: The AWSOTTaskRole ARN.
   - _`{{ecsTaskExecutionRoleArn}}`_: The AWSOTTaskExecutionRole ARN.
   - Add an environment variable named `AOT_CONFIG_CONTENT`. Select **ValueFrom** to tell ECS to get the value from the Parameter Store, and set the value to `collector-config`.

1. Follow the ECS Fargate setup instructions to [create a task definition][task] using the template.

### Configure {{% param "PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} offers two approaches for collecting ECS task and container metrics:

- **Native receiver:** Use the [`otelcol.receiver.awsecscontainermetrics`][awsecscontainermetrics] component to read the ECS Task Metadata Endpoint V4 directly. This approach is simpler but requires enabling experimental features.

- **Prometheus exporter:** Use the [Prometheus ECS exporter](https://github.com/prometheus-community/ecs_exporter) as a sidecar container and scrape metrics with `prometheus.scrape`. This approach requires an additional container.

#### Native receiver

The `otelcol.receiver.awsecscontainermetrics` component reads the reads AWS ECS task- and container-level metadata, and resource usage metrics such as CPU, memory, network, and disk, and forwards them to other otelcol.* components.

{{< admonition type="note" >}}
The `otelcol.receiver.awsecscontainermetrics` component is experimental. You must set the `--stability.level=experimental` flag to use it. Refer to [Permitted stability levels][stability-levels] for more information.

[stability-levels]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/#permitted-stability-levels
{{< /admonition >}}

Use the following as a starting point for your {{< param "PRODUCT_NAME" >}} configuration:

```alloy
otelcol.receiver.awsecscontainermetrics "default" {
  collection_interval = "60s"

  output {
    metrics = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")

    auth = otelcol.auth.basic.default.handler
  }
}

otelcol.auth.basic "default" {
  client_auth {
    username = sys.env("OTLP_USERNAME")
    password = sys.env("OTLP_PASSWORD")
  }
}
```

This configuration collects ECS task and container metrics every 60 seconds and exports them to an OTLP endpoint.

#### Prometheus exporter

If you prefer a stable approach, use the Prometheus ECS exporter as a sidecar container.

Use the following as a starting point for your {{< param "PRODUCT_NAME" >}} configuration:

```alloy
prometheus.scrape "ecs" {
  targets    = [{"__address__" = "localhost:9779"}]
  forward_to = [prometheus.remote_write.default.receiver]
}

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

This configuration scrapes metrics from the ECS exporter running on port 9779 and exports them to a Prometheus endpoint.

#### Deploy the task

Complete the following steps to create a sample task.

1. Create a Parameter Store entry to hold the collector configuration file.

   1. Open the AWS Console.
   1. In the AWS Console, choose **Parameter Store**.
   1. Choose **Create parameter**.
   1. Create a parameter with the following values:

      - **Name:** `collector-config`
      - **Tier:** Standard
      - **Type:** String
      - **Data type:** Text
      - **Value:** Copy and paste your custom {{< param "PRODUCT_NAME" >}} configuration file.

1. Download the [ECS Fargate][fargate-template] or [ECS EC2][ec2-template] task definition template from GitHub.
1. Edit the task definition template and add the following parameters.

   - _`{{region}}`_: The region to send the data to.
   - _`{{ecsTaskRoleArn}}`_: The AWSOTTaskRole ARN.
   - _`{{ecsTaskExecutionRoleArn}}`_: The AWSOTTaskExecutionRole ARN.
   - Set the container image to `grafana/alloy:<VERSION>`, for example `grafana/alloy:latest` or a specific version such as `grafana/alloy:v1.8.0`.
   - Add a custom environment variable named `ALLOY_CONFIG_CONTENT`. Select **ValueFrom** to tell ECS to get the value from the Parameter Store, and set the value to `collector-config`. {{< param "PRODUCT_NAME" >}} doesn't read this variable directly, but you use it with the command below to pass the configuration.
   - Add environment variables for your endpoint.
     - For the native receiver:
       - `OTLP_ENDPOINT`
       - `OTLP_USERNAME`
       - `OTLP_PASSWORD` - For increased security, create a password in AWS Secrets Manager and reference the ARN of the secret in the **ValueFrom** field.
     - For the Prometheus exporter:
       - `PROMETHEUS_REMOTE_WRITE_URL`
       - `PROMETHEUS_USERNAME`
       - `PROMETHEUS_PASSWORD` - For increased security, create a password in AWS Secrets Manager and reference the ARN of the secret in the **ValueFrom** field.
   - In the Docker configuration, change the **Entrypoint** to `/bin/sh,-c`. This allows the command to use shell features like redirection and chaining.
   - _`{{command}}`_: For the native receiver, use `"printenv ALLOY_CONFIG_CONTENT > /tmp/config_file && exec /bin/alloy run --stability.level=experimental --server.http.listen-addr=0.0.0.0:12345 /tmp/config_file"`. The `--stability.level=experimental` flag enables the use of the `otelcol.receiver.awsecscontainermetrics` component.

     For the Prometheus exporter, use `"printenv ALLOY_CONFIG_CONTENT > /tmp/config_file && exec /bin/alloy run --server.http.listen-addr=0.0.0.0:12345 /tmp/config_file"`.

     Make sure you don't omit the double quotes around the command.
   - **Prometheus exporter only:** Add the Prometheus ECS exporter as a sidecar container:

     1. Add a container to the task.
     1. Set the container name to `ecs-exporter`.
     1. Set the image to a pinned version of the exporter, for example `quay.io/prometheuscommunity/ecs-exporter:v0.1.1`. Check the [ecs_exporter releases](https://github.com/prometheus-community/ecs_exporter/releases) for the latest stable version. You should avoid using the `latest` tag in production.
     1. Add `tcp/9779` as a port mapping.

1. Follow the ECS Fargate setup instructions to [create a task definition][task] using the template.

## Collect EC2 instance metrics

For ECS Clusters running on EC2, you can collect instance metrics by using AWS ADOT or {{< param "PRODUCT_NAME" >}} in a separate ECS task deployed as a daemon.

### Collect metrics with {{% param "PRODUCT_NAME" %}}

You can follow the steps described in [Configure {{< param "PRODUCT_NAME" >}}](#configure-alloy) to create another task, with the following changes:

- If you're using the Prometheus exporter, you don't need to add the ecs-exporter sidecar for EC2 instance metrics.
- Run the task as a daemon so it automatically runs one instance per node in your cluster.
- Update your {{< param "PRODUCT_NAME" >}} configuration to collect metrics from the instance.
   The configuration varies depending on the type of EC2 node. Refer to the [`collect`](https://grafana.com/docs/alloy/latest/collect/) documentation for details.

### Collect metrics with ADOT

The approach described in [the AWS OpenTelemetry documentation](https://aws-otel.github.io/docs/setup/ecs#3-setup-the-adot-collector-for-ecs-ec2-instance-metrics) uses the `awscontainerinsightreceiver` receiver from OpenTelemetry. ADOT includes this receiver.

You need to use a custom configuration Parameter Store entry based on the [sample](https://github.com/aws-observability/aws-otel-collector/blob/main/config/ecs/otel-instance-metrics-config.yaml) configuration file to route the telemetry to your final destination.

## Collect application telemetry

To collect metrics and traces emitted by your application, use the OTLP endpoints exposed by the collector sidecar container regardless of the collector implementation.
Specify `localhost` as the host name, which is the default for most instrumentation libraries and agents.

For Prometheus endpoints, add a scrape job to the ADOT or {{< param "PRODUCT_NAME" >}} configuration, and use `localhost`, service port, and endpoint path.

## Collect Logs

The easiest way to collect application logs in ECS is to use the [AWS FireLens log driver][firelens-doc].
Depending on your use case, you can forward your logs to the collector container in your task using the [Fluent Bit plugin for OpenTelemetry][fluentbit-otel-plugin] or using the Fluent Bit Loki plugin.
You can also send everything directly to your final destination.

[Components]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/get-started/components
[firelens-doc]: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/firelens-taskdef.html
[fluentbit-otel-plugin]: https://docs.fluentbit.io/manual/pipeline/outputs/opentelemetry
[otel-templates]: https://github.com/aws-observability/aws-otel-collector/tree/main/config/ecs
[otel-prometheus]: https://github.com/aws-observability/aws-otel-collector/blob/357f9c7b8896dba6ee0e03b8efd7ca7117024d2e/config/ecs/ecs-amp-xray-prometheus.yaml
[adot-doc]: https://aws-otel.github.io/docs/setup/ecs
[otel-task-metrics-config]: https://github.com/aws-observability/aws-otel-collector/blob/main/config/ecs/container-insights/otel-task-metrics-config.yaml
[ecs-default-config]: https://github.com/aws-observability/aws-otel-collector/blob/main/config/ecs/ecs-default-config.yaml
[awsecscontainermetrics]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/otelcol.receiver.awsecscontainermetrics/
[ecs-metadata-v4]: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html
[fargate-template]: https://github.com/aws-observability/aws-otel-collector/blob/main/examples/ecs/aws-cloudwatch/ecs-fargate-sidecar.json
[ec2-template]: https://github.com/aws-observability/aws-otel-collector/blob/main/examples/ecs/aws-prometheus/ecs-ec2-task-def.json
[task]: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html
