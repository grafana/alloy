---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/runbooks/
description: Learn how to respond to Grafana Alloy alerts
menuTitle: Runbooks
title: Grafana Alloy runbooks
weight: 400
---

# {{% param "FULL_PRODUCT_NAME" %}} runbooks

This page describes how to investigate and resolve the alerts defined in the [{{< param "PRODUCT_NAME" >}} mixin](https://github.com/grafana/alloy/tree/{{< param "ALLOY_RELEASE" >}}/operations/alloy-mixin).

## OpenTelemetry alerts

### `OtelcolReceiverRefused*`

**Alert names:** `OtelcolReceiverRefusedSpans`, `OtelcolReceiverRefusedMetrics`, `OtelcolReceiverRefusedLogs`

**Meaning:** An OpenTelemetry Collector receiver component in {{< param "PRODUCT_NAME" >}} is refusing spans, metrics, or logs.

**Investigation:**

- Check the resource usage of your {{< param "PRODUCT_NAME" >}} instances. If they're overloaded, they may need more resources.
- Check the logs for errors that explain why data is being refused.
- Check the OpenTelemetry dashboard for signs of issues, such as uneven load balancing.

**Resolution:**

- Restart the busy instances if load balancing is very uneven.
- Scale up resources if the issue is related to resource constraints.
- Reduce the volume of data, for example by adjusting sampling, if too much is being sent.

### `OtelcolExporterFailed*`

**Alert names:** `OtelcolExporterFailedSpans`, `OtelcolExporterFailedMetrics`, `OtelcolExporterFailedLogs`

**Meaning:** Exporting spans, metrics, or logs to the downstream endpoint is failing.

**Known issues:**

- The downstream server may reject large traces that exceed the gRPC message size limit. Refer to [opentelemetry-collector issue 6046](https://github.com/open-telemetry/opentelemetry-collector/issues/6046).
- The downstream endpoint may rate limit requests too aggressively.
- There may be network connectivity issues between {{< param "PRODUCT_NAME" >}} and the endpoint.

**Investigation:**

- Check the OpenTelemetry dashboard for signs of issues.
- Check the logs for specific exporter error messages.
- Verify connectivity to the endpoint if all traffic is failing.
- Check whether the downstream endpoint is having issues.
- Look for patterns in the failed exports, such as specific sources or payload sizes.

**Resolution:**

- For message size issues, reduce the batch size in the configuration.
- If requests are rate limited, sample more aggressively or increase the limits on the downstream endpoint.
