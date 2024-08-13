---
canonical: https://grafana.com/docs/alloy/latest/collect/ecs-opentelemetry-data/
description: Learn how to collect Amazon ECS or AWS Fargate OpenTelemetry data and forward it to any OpenTelemetry-compatible endpoint
menuTitle: Collect ECS or Fargate OpenTelemetry data
title: Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data
weight: 500
---

# Collect Amazon Elastic Container Service or AWS Fargate OpenTelemetry data

You can configure {{% param "FULL_PRODUCT_NAME" %}} to collect [OpenTelemetry][]-compatible data from Amazon Elastic Container Service (ECS) or AWS Fargate and forward it to any OpenTelemetry-compatible endpoint.

This topic describes how to:

* Set up and configure an ECS task to use {{< param "PRODUCT_NAME" >}} to collect telemetry.
* Run {{% param "PRODUCT_NAME" %}} directly in your Amazon ECS or AWS Fargate instance, or as a sidecar.

## Before you begin

* Ensure that you have basic familiarity with instrumenting applications with OpenTelemetry.
* Have an available Amazon ECS or AWS Fargate deployment.
* Identify where {{< param "PRODUCT_NAME" >}} writes received telemetry data.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Create an ECS Task definition

To create an ECS Task Definition for AWS Fargate with an ADOT collector, complete the following steps.

1. Download the ECS Fargate task definition template from GitHub.
1. Enter the following parameters in the task definition templates.
   * {{region}} - the region the data will be sent to
   * {{ecsTaskRoleArn}} - AWSOTTaskRole ARN created in the previous section
   * {{ecsExecutionRoleArn}} - AWSOTTaskExcutionRole ARN created in the previous section
   * command - Assign value to the command variable to select the config file path.
     The AWS Collector comes with two configs baked in for ECS customers (choose one of them based on your purpose):
     * --config=/etc/ecs/ecs-default-config.yaml - To consume StatsD metrics, OTLP metrics/traces and X-Ray SDK traces (custom application metrics/traces) * --config=/etc/ecs/container-insights/otel-task-metrics-config.yaml - To Use StatsD, OTLP, Xray and Container Resource utilization metrics

1. Follow the ECS Fargate setup instructions to create a task definition using the given template.

## Use the custom OpenTelemetry configuration file from SSM Parameter

Learn how to upload a custom OpenTelemetry configuration file to the SSM Parameter store and use it with AWS Distro for OpenTelemetry Collector. We can use Grafana Alloy as a custom OpenTelemetry collector using this method.

The AWS Distro for OpenTelemetry Collector can optionally be configured via an environment variable, AOT_CONFIG_CONTENT. This variable is expected to contain a full Collector configuration file; it will override the config file used in the Collector entry point command. In ECS, the values of environment variables can be set from AWS Systems Manager Parameters.

### Update Task Definition

#### Select Task Definition

Go to the AWS Management Console and select Elastic Container Service.
From the left-side navigation, select Task definition.
Select the TaskDefinition we just created to run AWS Distro for OpenTelemetry Collector and click the Create new revision button at the top.

#### Add Environment Variable

From the container definition section, click the AWS Distro for OpenTelemetry Collector container (image: amazon/aws-otel-collector) and go to the Environment variables section.
Add a new environment variable—AOT_CONFIG_CONTENT. Select ValueFrom, which will tell ECS to get the value from the SSM Parameter, and set otel-collector-config (the SSM parameter name we will create in the next section) as the value.
Finish updating the task definition and creating a new revision.

### Create SSM Parameter

#### Go to Parameter Store

Go to System Manager service from AWS Management Console and select Parameter Store from the left side navigation panel.

#### Create New Parameter

From the top-right corner, click the Create new parameter button.
Create a new parameter with the following information.

The parameter name should be the same as we used in the environment variable of our task-definition.

* Name: otel-collector-config
* Tier: Standard
* Type: String
* Data type: Text
* Value: Copy and paste your custom OpenTelemetry configuration file or Alloy config file. For details on creating an Alloy configuration file, read the Grafana Alloy documentation here.

### Run Task

When you run a task with this new Task Definition, it will use your custom OpenTelemetry configuration file from the SSM Parameter.

## Run Alloy in your instance directly or as a sidecar

SSH or connect to the ECS or AWS Fargate-managed container

If you need an overview of how to do this, follow the steps here.

You can also connect via your preferred method as long as we can pass the parameters needed to install and configure Grafana Alloy.

### Install Grafana Alloy

After connecting to your instance, follow the Grafana Alloy installation and configuration instructions.
<https://grafana.com/docs/alloy/latest/get-started/install/linux/>
<https://grafana.com/docs/alloy/latest/reference/config-blocks/>

Deployment methods can be found here.
