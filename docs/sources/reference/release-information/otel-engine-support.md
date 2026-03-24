---
canonical: https://grafana.com/docs/alloy/latest/reference/release-information/otel-engine-support/
description: Support policy for the OpenTelemetry Engine in Grafana Alloy
menuTitle: OpenTelemetry Engine support
title: OpenTelemetry Engine support policy
weight: 300
---

# {{% param "OTEL_ENGINE" %}} support policy

The {{< param "OTEL_ENGINE" >}} lets you run {{< param "PRODUCT_NAME" >}} as a fully compatible OpenTelemetry Collector.
It combines code maintained by the {{< param "PRODUCT_NAME" >}} maintainers with upstream OpenTelemetry Collector code maintained by the open-source community.
Different parts of the {{< param "OTEL_ENGINE" >}} have different levels of stability and support.

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Quick reference

| Area | Support level |
|---|---|
| {{< param "PRODUCT_NAME" >}} binary, `alloy otel` CLI, `alloyengine` extension | Fully supported |
| Upstream OpenTelemetry Collector core engine | Supported; upstream-dependent resolution |
| Bundled upstream OpenTelemetry Collector components | Supported; upstream-dependent resolution |
| [Community components][] | Not supported by the {{< param "PRODUCT_NAME" >}} maintainers |
| Custom builds (OCB) with a subset of standard {{< param "PRODUCT_NAME" >}} components | Supported; upstream-dependent resolution |
| Custom builds (OCB) with custom or non-bundled components | Best effort / not supported |

## {{< param "PRODUCT_NAME" >}}-maintained components

The {{< param "PRODUCT_NAME" >}} maintainers implement and maintain the following parts of the {{< param "OTEL_ENGINE" >}}:

- **The `alloy otel` CLI**: The command-line interface for starting and managing the {{< param "OTEL_ENGINE" >}}.
- **The `alloyengine` extension**: The extension that runs the {{< param "DEFAULT_ENGINE" >}} alongside the {{< param "OTEL_ENGINE" >}}.
- **The {{< param "PRODUCT_NAME" >}} binary**: The overall binary, build, and release process.

These components follow the same [backward compatibility][] guarantees as the rest of {{< param "PRODUCT_NAME" >}}, subject to the current [stability level][Grafana release life cycle] of the {{< param "OTEL_ENGINE" >}} feature.

## Upstream OpenTelemetry Collector components

The {{< param "OTEL_ENGINE" >}} embeds the upstream OpenTelemetry Collector core engine and bundles components from the [OpenTelemetry Collector][] and [OpenTelemetry Collector Contrib][] repositories.
These components are developed, maintained, and released by the OpenTelemetry community.

The {{< param "PRODUCT_NAME" >}} maintainers support upstream components by triaging issues, investigating root causes, and driving fixes through the upstream community.
However, the final resolution of upstream issues depends on the OpenTelemetry community's review and release processes.

## Community components

[Community components][] are components that the {{< param "PRODUCT_NAME" >}} community implements and maintains.
The same policy applies whether you use them with the {{< param "DEFAULT_ENGINE" >}} or the {{< param "OTEL_ENGINE" >}}:

- The {{< param "PRODUCT_NAME" >}} maintainers don't provide commercial support for community components.
- Community components don't have defined stability levels and aren't covered by the [backward compatibility][] strategy.
- The {{< param "PRODUCT_NAME" >}} maintainers may disable or remove community components without an active maintainer if they impede the development of {{< param "PRODUCT_NAME" >}}.

Refer to [Community components][] for more information.

## Stability level mapping

{{< param "PRODUCT_NAME" >}} and the OpenTelemetry Collector use different stability level systems.
The following table shows how they relate:

| {{< param "PRODUCT_NAME" >}} level | OpenTelemetry Collector level | Description |
|---|---|---|
| [Experimental][Grafana release life cycle] | Development, Alpha | Early-stage. Subject to breaking changes or removal without notice. Not for production use. |
| [Public preview][Grafana release life cycle] | Beta | More mature. Configuration is relatively stable but breaking changes are possible. Not for production use. |
| [Generally available][Grafana release life cycle] | Stable | Production-ready with backward compatibility guarantees and full support. |

The stability level of a bundled upstream component in {{< param "PRODUCT_NAME" >}} may be lower than its upstream stability level based on the {{< param "PRODUCT_NAME" >}} maintainers' assessment of the component's maturity and reliability.
The stability level generally isn't raised above the upstream level.

Refer to the [Grafana release life cycle][] for definitions of {{< param "PRODUCT_NAME" >}} stability levels.
Refer to the [OpenTelemetry Collector component stability][] documentation for definitions of upstream levels.

## Custom builds with OpenTelemetry Collector Builder

You can use the [OpenTelemetry Collector Builder (OCB)][OCB] to create a custom build of {{< param "PRODUCT_NAME" >}} with additional or fewer components.

The [{{< param "PRODUCT_NAME" >}} OCB manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml) is the starting point for custom builds.

### Supported scenarios

The {{< param "PRODUCT_NAME" >}} maintainers will offer support for the following scenarios:

- **Your issue is unrelated to custom components.** If the problem is in functionality provided by the standard {{< param "PRODUCT_NAME" >}} release, the {{< param "PRODUCT_NAME" >}} maintainers support you regardless of whether you use a custom build.
- **Your issue is reproducible with the standard release.** If you can reproduce the issue using the standard {{< param "PRODUCT_NAME" >}} binary without your custom components, the {{< param "PRODUCT_NAME" >}} maintainers treat it as a standard support case.

### Unsupported scenarios

The {{< param "PRODUCT_NAME" >}} maintainers may not be able to provide support if:

- **The issue is caused by a custom component.** If the problem is in a component that you added and that isn't part of the standard {{< param "PRODUCT_NAME" >}} release, the {{< param "PRODUCT_NAME" >}} maintainers can't debug or fix your custom code.
- **The issue is caused by removing essential components.** If you removed components that are part of the standard release and this causes problems, the {{< param "PRODUCT_NAME" >}} maintainers can't support this configuration.
- **The issue can't be isolated from your custom components.** If it isn't possible to determine whether the issue is related to your custom components, the {{< param "PRODUCT_NAME" >}} maintainers may not be able to proceed with troubleshooting until the issue is reproduced with the standard release.

## Further reading

- Refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../../introduction/otel_alloy/) for the full list of bundled components.
- Refer to [{{< param "PRODUCT_NAME" >}} backward compatibility](../backward-compatibility/) for general backward compatibility guarantees.
- Refer to [The {{< param "OTEL_ENGINE" >}}](../../../set-up/otel_engine/) for information about how to run the {{< param "OTEL_ENGINE" >}}.

[backward compatibility]: ../backward-compatibility/
[Grafana release life cycle]: https://grafana.com/docs/release-life-cycle/
[OpenTelemetry Collector]: https://github.com/open-telemetry/opentelemetry-collector
[OpenTelemetry Collector Contrib]: https://github.com/open-telemetry/opentelemetry-collector-contrib
[OpenTelemetry Collector component stability]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md
[Community components]: ../../../get-started/components/community-components/
[OCB]: https://opentelemetry.io/docs/collector/custom-collector/
