---
canonical: https://grafana.com/docs/alloy/latest/collect/ecs-opentelemetry-data/
description: Learn how to collect Amazon ECS or AWS Fargate OpenTelemetry data and forward it to any OpenTelemetry-compatible endpoint
menuTitle: Collect ECS or Fargate OpenTelemetry data
title: Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data
weight: 500
---

# Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data

You can configure {{< param "FULL_PRODUCT_NAME" >}} to collect OpenTelemetry-compatible data from Amazon Elastic Container Service (ECS) or AWS Fargate and forward it to any OpenTelemetry-compatible endpoint.

There are three different ways you can use {{< param "PRODUCT_NAME" >}} to collect Amazon ECS or AWS Fargate telemetry data.

1. [Use a custom OpenTelemetry configuration file from the SSM Parameter store](#use-a-custom-opentelemetry-configuration-file-from-the-ssm-parameter-store).
1. [Create an ECS task definition](#create-an-ecs-task-definition).
1. [Run {{< param "PRODUCT_NAME" >}} directly in your instance, or as a Kubernetes sidecar](#run-alloy-directly-in-your-instance-or-as-a-kubernetes-sidecar).

## Before you begin

* Ensure that you have basic familiarity with instrumenting applications with OpenTelemetry.
* Have an available Amazon ECS or AWS Fargate deployment.
* Identify where {{< param "PRODUCT_NAME" >}} writes received telemetry data.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Use a custom OpenTelemetry configuration file from the SSM Parameter store

You can upload a custom OpenTelemetry configuration file to the SSM Parameter store and use {{< param "PRODUCT_NAME" >}} as a telemetry data collector.

You can configure the AWS Distro for OpenTelemetry Collector with the `AOT_CONFIG_CONTENT` environment variable.
This environment variable contains a full collector configuration file and it overrides the configuration file used in the collector entry point command.
In ECS, you can set the values of environment variables from AWS Systems Manager Parameters.

### Update the task definition

 1. Select the task definition.
    1. Open the AWS Systems Manager console.
    1. Select Elastic Container Service.
    1. In the navigation pane, choose *Task definition*.
    1. Select the TaskDefinition you just created to run AWS Distro for OpenTelemetry Collector and click the Create new revision button at the top.
1. Add an environment variable.
   1. From the container definition section, click the AWS Distro for OpenTelemetry Collector container and go to the Environment variables section.
   1. Add a new environment variable `AOT_CONFIG_CONTENT`.
   1. Select ValueFrom, to tell ECS to get the value from the SSM Parameter, and set the value to `otel-collector-config`.
1. Finish updating the task definition and creating a new revision.

### Create the SSM parameter

1. Open the AWS Systems Manager console.
1. In the navigation pane, choose *Parameter Store*.
1. Choose *Create parameter*.
1. Create a new parameter with the following values:
   * `Name`: otel-collector-config
   * `Tier`: Standard
   * `Type`: String
   * `Data type`: Text
   * `Value`: Copy and paste your custom OpenTelemetry configuration file or [{{< param "PRODUCT_NAME" >}} configuration file][configure].

### Run your new task

When you run a task with this new Task Definition, it will use your custom OpenTelemetry configuration file from the SSM Parameter.

## Create an ECS Task definition

To create an ECS Task Definition for AWS Fargate with an ADOT collector, complete the following steps.

1. Download the [ECS Fargate task definition template][template] from GitHub.
1. Enter the following parameters in the task definition templates.
   * `{{region}}`: The region the data is sent to.
   * `{{ecsTaskRoleArn}}`: The AWSOTTaskRole ARN.
   * `{{ecsExecutionRoleArn}}`: The AWSOTTaskExcutionRole ARN.
   * `command` - Assign a value to the command variable to select the path to the configuration file.
     The AWS Collector comes with two configurations. Select one of them based on your environment:
     * Use `--config=/etc/ecs/ecs-default-config.yaml` to consume StatsD metrics, OTLP metrics and traces, and X-Ray SDK traces.
     * Use `--config=/etc/ecs/container-insights/otel-task-metrics-config.yaml` to Use StatsD, OTLP, Xray, and Container Resource utilization metrics.
1. Follow the ECS Fargate setup instructions to create a task definition using the template.

## Run {{% param "PRODUCT_NAME" %}} directly in your instance, or as a Kubernetes sidecar

SSH or connect to the Amazon ECS or AWS Fargate-managed container. Refer to [9 steps to SSH into an AWS Fargate managed container][steps] for more information about using SSH with Amazon ECS or AWS Fargate.

You can also use your own method to connect to the Amazon ECS or AWS Fargate-managed container as long as you can pass the parameters needed to install and configure {{< param "PRODUCT_NAME" >}}.

### Install Grafana Alloy

After connecting to your instance, follow the {{< param "PRODUCT_NAME" >}} [installation][install], [configuration][configure] and [deployment][deploy] instructions.

[template]: https://github.com/aws-observability/aws-otel-collector/blob/master/examples/ecs/aws-cloudwatch/ecs-fargate-sidecar.json
[configure]: https://grafana.com/docs/alloy/latest/configure/
[steps]: https://medium.com/ci-t/9-steps-to-ssh-into-an-aws-fargate-managed-container-46c1d5f834e2
[install]: https://grafana.com/docs/alloy/latest/set-up/install/linux/
[deploy]: https://grafana.com/docs/alloy/latest/set-up/deploy/
