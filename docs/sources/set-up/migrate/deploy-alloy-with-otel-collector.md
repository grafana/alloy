---
canonical: https://grafana.com/docs/alloy/latest/set-up/migrate/deploy-alloy-with-otel-collector/
description: Learn how to deploy Grafana Alloy alongside another distribution of the OpenTelemetry Collector
menuTitle: Deploy Alloy alongside another OTel Collector
title: Deploy Grafana Alloy alongside another OpenTelemetry Collector distribution
weight: 500
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}} alongside another OpenTelemetry Collector distribution

You can deploy both {{< param "PRODUCT_NAME" >}} and another OpenTelemetry Collector distribution in your architecture.
A common use case for doing this is when you're migrating from a third-party vendor to Grafana OSS or Grafana Cloud.
During migration, you may need to send telemetry to both Grafana and your legacy vendor backend.
Although {{< param "PRODUCT_NAME" >}} includes several third-party vendor exporters, these are provided as [community components][] without commercial support.
To get commercial support for these specific OpenTelemetry components from your legacy vendor, you'll usually need to deploy them as part of the [OpenTelemetry Collector Contrib distribution][].

## Example: Migrate from Datadog

1. Deploy and configure {{< param "PRODUCT_NAME" >}} in your environment.
1. Deploy and configure an OpenTelemetry Collector distribution.
   Alternatively, you can use the [OpenTelemetry Collector builder (OCB)][] to create a custom distribution of the OpenTelemetry Collector that includes the components needed to receive and export telemetry to Datadog using the Datadog protocol.
   This includes the [OTLP Receiver][], [Datadog Connector][], and [Datadog Exporter][] from OpenTelemetry upstream.
1. Configure the OTLP Exporter in your {{< param "PRODUCT_NAME" >}} deployment to send telemetry in OTLP format directly to your preferred Grafana backend and to the custom OpenTelemetry Collector you deployed for exporting to Datadog.

[community components]: ../../get-started/community_components/
[OpenTelemetry Collector Contrib distribution]: https://github.com/open-telemetry/opentelemetry-collector-contrib
[OpenTelemetry Collector builder (OCB)]: https://opentelemetry.io/docs/collector/custom-collector
[OTLP Receiver]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/receiver/otlpreceiver/README.md
[Datadog Connector]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/datadogconnector
[Datadog Exporter]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/datadogexporter