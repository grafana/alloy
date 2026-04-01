---
canonical: https://grafana.com/docs/alloy/latest/reference/release-information/alloy-support/
description: Support scope for Grafana Alloy
menuTitle: Support
title: Grafana Alloy support scope
weight: 300
---

# {{% param "FULL_PRODUCT_NAME" %}} support

This page defines support scope for {{< param "PRODUCT_NAME" >}}, including both the {{< param "DEFAULT_ENGINE" >}} and the {{< param "OTEL_ENGINE" >}}.
{{< param "PRODUCT_NAME" >}} includes code maintained by the {{< param "PRODUCT_NAME" >}} maintainers and upstream dependencies maintained by open-source communities.

For full context, read this page together with [{{< param "PRODUCT_NAME" >}} backward compatibility](../backward-compatibility/) and the [Grafana release life cycle](https://grafana.com/docs/release-life-cycle/).

## Support levels

- **Supported:** The feature is maintained by {{< param "PRODUCT_NAME" >}} maintainers or within Grafana open-source projects integrated with {{< param "PRODUCT_NAME" >}}.
- **Supported with upstream dependency:** The feature depends on upstream projects outside the Grafana open-source ecosystem. The ultimate resolution of upstream issues may depend on the upstream community's review and release processes.
- **Not supported / best effort:** The feature is outside standard supported scope.

## Supported

Example features include:

- Core {{< param "DEFAULT_ENGINE" >}} runtime and operational experience, including installation, running, and configuration flow. For engine terminology and differences, refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../../introduction/otel_alloy/).
- {{< param "PRODUCT_NAME" >}} platform capabilities such as {{< param "PRODUCT_NAME" >}} configuration syntax, clustering, Fleet Management, and the built-in debugging UI.
- {{< param "PRODUCT_NAME" >}} installation scripts, Helm chart deployment, and bundled Grafana dashboards.
- {{< param "PRODUCT_NAME" >}}-owned integration points around the {{< param "OTEL_ENGINE" >}}, such as the `alloyengine` extension.
- Features based on Grafana open-source projects integrated with {{< param "PRODUCT_NAME" >}}, such as Mimir, Loki, Beyla, Tempo, Faro, Pyroscope and Database Observability.

## Supported with upstream dependency

Example features include:

- Features built on bundled upstream OpenTelemetry Collector components or Prometheus-related projects.
- Compatibility behavior tied to upstream protocols, formats, and ecosystem standards (for example OTLP and Prometheus Remote Write protocols).
- {{< param "OTEL_ENGINE" >}} configuration and features that depend on the upstream OpenTelemetry Collector core engine.

The {{< param "PRODUCT_NAME" >}} maintainers can investigate and collaborate upstream, but final resolution may depend on upstream acceptance and release timing.
In some cases, maintainers may carry a temporary patch or fork, but this is case-by-case and not guaranteed.

## Not supported / best effort

Example features include:

- [Community components][] (not officially supported).
- Custom or non-standard components added to custom {{< param "PRODUCT_NAME" >}} builds via OCB or otherwise.

## Extending {{< param "PRODUCT_NAME" >}} with OCB and custom builds

You can use the [OpenTelemetry Collector Builder (OCB)][OCB] to build a customized {{< param "PRODUCT_NAME" >}} binary, including adding components or removing components from the standard release.
The [{{< param "PRODUCT_NAME" >}} OCB manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml) is the starting point for custom builds.

Support usually follows the standard support scope when feature behavior is unrelated to customizations or reproducible with the standard {{< param "PRODUCT_NAME" >}} release.
Support may be limited when feature behavior is caused by a custom-added component, caused by removing components from the standard release, or can't be isolated from customizations.

## Further reading

- Refer to [{{< param "PRODUCT_NAME" >}} backward compatibility](../backward-compatibility/) for general backward compatibility guarantees.
- Refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../../introduction/otel_alloy/) for bundled OpenTelemetry components.
- Refer to [The {{< param "OTEL_ENGINE" >}}](../../../set-up/otel_engine/) for information about how to run the {{< param "OTEL_ENGINE" >}}.
- Refer to the [Grafana release life cycle](https://grafana.com/docs/release-life-cycle/) for stability level definitions.
- Refer to [Community components][] for details and limitations.

[community components]: ../../../get-started/components/community-components/
[OCB]: https://opentelemetry.io/docs/collector/custom-collector/
