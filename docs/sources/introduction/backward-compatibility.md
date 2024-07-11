---
canonical: https://grafana.com/docs/alloy/latest/introduction/backward-compatibility/
description: Grafana Alloy backward compatibility
menuTitle: Backward compatibility
title: Grafana Alloy backward compatibility
weight: 999
---

# {{% param "FULL_PRODUCT_NAME" %}} backward compatibility

{{< param "FULL_PRODUCT_NAME" >}} follows [semantic versioning][].
{{< param "PRODUCT_NAME" >}} is stable, and we strive to maintain backward compatibility between minor and patch versions.

Documented functionality that's released as _Generally available_ is covered by backward compatibility, including:

* **User configuration**, including the {{< param "PRODUCT_NAME" >}} configuration syntax, the semantics of the configuration file, and the command-line interface.

* **APIs**, for any network or code API released as v1.0.0 or later.

* **Observability data used in official dashboards**, where the official set of dashboards are found in [the `alloy-mixin/` directory][alloy-mixin].

## Exceptions

We strive to maintain backward compatibility, but there are situations that may arise that require a breaking change without a new major version:

* **Security**: A security issue may arise that requires breaking compatibility.

* **Legal requirements**: If we learn that exposed behavior violates a licensing or legal requirement, a breaking change may be required.

* **Specification errors**: If a specification for a feature is found to be incomplete or inconsistent, fixing the specification may require a breaking change.

* **Bugs**: If a bug is found that goes against the documented specification of that functionality, fixing the bug may require breaking compatibility for users who are relying on the incorrect behavior.

* **Upstream changes**: Much of the functionality of {{< param "PRODUCT_NAME" >}} is built on top of other software, such as OpenTelemetry Collector and Prometheus. If upstream software breaks compatibility, we may need to reflect this in {{< param "PRODUCT_NAME" >}}.

* **Community components**: Community components are components implemented and maintained by the community. They are not covered by our backward compatibility strategy.

We try, whenever possible, to resolve these issues without breaking compatibility.

[semantic versioning]: https://semver.org/
[alloy-mixin]: https://github.com/grafana/alloy/tree/main/operations/alloy-mixin
