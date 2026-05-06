---
canonical: https://grafana.com/docs/alloy/latest/reference/release-information/alloy-maintenance/
description: Maintenance scope for Grafana Alloy
menuTitle: Maintenance
title: Grafana Alloy maintenance scope
weight: 300
---

# {{% param "FULL_PRODUCT_NAME" %}} maintenance scope

This page defines maintenance scope for {{< param "PRODUCT_NAME" >}}, including both the {{< param "DEFAULT_ENGINE" >}} and the {{< param "OTEL_ENGINE" >}}.
{{< param "PRODUCT_NAME" >}} includes code maintained by the {{< param "PRODUCT_NAME" >}} maintainers and upstream dependencies maintained by open-source communities.

For full context, read this page together with [{{< param "PRODUCT_NAME" >}} backward compatibility](../backward-compatibility/) and the [Grafana release life cycle](https://grafana.com/docs/release-life-cycle/).

## Maintenance levels

- **Maintained:** The feature is maintained by {{< param "PRODUCT_NAME" >}} maintainers or within Grafana open-source projects integrated with {{< param "PRODUCT_NAME" >}}.
- **Maintained with upstream dependency:** The feature depends on upstream projects outside the Grafana open-source ecosystem. Final resolution may depend on the upstream community's review and release processes.
- **Not maintained by {{< param "PRODUCT_NAME" >}} maintainers:** The feature is outside the standard maintenance scope of {{< param "PRODUCT_NAME" >}} maintainers.

## Maintained

Example features include:

- Core {{< param "DEFAULT_ENGINE" >}} runtime and operational experience, including installation, running, and configuration flow. For engine terminology and differences, refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../../introduction/otel_alloy/).
- {{< param "PRODUCT_NAME" >}} platform capabilities such as {{< param "PRODUCT_NAME" >}} configuration syntax, clustering, Fleet Management, and the built-in debugging UI.
- {{< param "PRODUCT_NAME" >}} installation scripts, Helm chart deployment, and bundled Grafana dashboards.
- {{< param "PRODUCT_NAME" >}}-owned integration points around the {{< param "OTEL_ENGINE" >}}, such as the `alloyengine` extension.
- Features based on Grafana open-source projects integrated with {{< param "PRODUCT_NAME" >}}, such as Mimir, Loki, Beyla, Tempo, Faro, Pyroscope, and Database Observability.

## Maintained with upstream dependency

Example features include:

- Features built on bundled upstream OpenTelemetry Collector components or Prometheus-related projects.
- Compatibility behavior tied to upstream protocols, formats, and ecosystem standards (for example OTLP and Prometheus Remote Write protocols).
- {{< param "OTEL_ENGINE" >}} configuration and features that depend on the upstream OpenTelemetry Collector core engine.

The {{< param "PRODUCT_NAME" >}} maintainers can investigate and collaborate upstream, but resolution timing may depend on upstream acceptance and release timing.
In some cases, maintainers may carry a temporary patch or fork, but this is case-by-case and not guaranteed.

## Not maintained by {{< param "PRODUCT_NAME" >}} maintainers

Example features include:

- [Community components][].
- Custom or non-standard components added to custom {{< param "PRODUCT_NAME" >}} builds via OCB or otherwise.

## Extending {{< param "PRODUCT_NAME" >}} with OCB and custom builds

With {{< param "OTEL_ENGINE" >}}, you can use the [OpenTelemetry Collector Builder (OCB)][OCB] to build a customized {{< param "PRODUCT_NAME" >}} binary, including adding components or removing components from the standard release.
The [{{< param "PRODUCT_NAME" >}} OCB manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml) is the starting point for custom builds.

Response to issues usually follows the standard scope when the issue is unrelated to customizations or reproducible with the standard, maintained {{< param "PRODUCT_NAME" >}} release.
Response to issues may be limited when behavior can't be isolated from customizations and can't be reproduced with the standard, maintained {{< param "PRODUCT_NAME" >}} release.

## Further reading

- Refer to [{{< param "PRODUCT_NAME" >}} backward compatibility](../backward-compatibility/) for general backward compatibility guarantees.
- Refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../../introduction/otel_alloy/) for bundled OpenTelemetry components.
- Refer to [The {{< param "OTEL_ENGINE" >}}](../../../set-up/otel_engine/) for information about how to run the {{< param "OTEL_ENGINE" >}}.
- Refer to the [Grafana release life cycle](https://grafana.com/docs/release-life-cycle/) for stability level definitions.
- Refer to [Community components][] for details and limitations.

[community components]: ../../../get-started/components/community-components/
[OCB]: https://opentelemetry.io/docs/collector/custom-collector/
