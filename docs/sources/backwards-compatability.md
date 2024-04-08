---
canonical: https://grafana.com/docs/alloy/latest/backwards-compatability/
description: Grafana Alloy backwards compatibility
menuTitle: Backwards compatibility
title: Grafana Alloy backwards compatibility
weight: 950
---

# {{% param "FULL_PRODUCT_NAME" %}} backwards compatibility

{{< param "FULL_PRODUCT_NAME" >}} follows [semantic versioning][].
This means that {{< param "PRODUCT_NAME" >}} is stable, and that we strive to maintain backwards compatibility between minor and patch versions.

Functionality which is documented and released as _Generally available_ is covered by backwards compatibility, including:

* **User configuration**, including the {{< param "PRODUCT_NAME" >}} configuration syntax, semantics of the configuration file, and the command-line interface.

* **APIs**, for any network or code API released as v1.0.0 or later.

* **Observability data used in official dashboards**, where the official set of dashboards are found in [the `alloy-mixin/` directory][alloy-mixin].

## Exceptions

We strive to maintain backwards compatibility, but there are situations which may arise that require a breaking change without a new major version:

* **Security**: A security issue may arise that requires breaking compatibility.

* **Legal requirements**: If we learn that exposed behavior violates a licensing or legal requirement, a breaking change may be required.

* **Specification errors**: If a specification for a feature is found to be incomplete or inconsistent, fixing the specification may require a breaking change.

* **Bugs**: If a bug is found that goes against the documented specification of that functionality, fixing the bug may require breaking compatibility for users who are relying on the incorrect behavior.

* **Upstream changes**: Much of the functionality of {{< param "PRODUCT_NAME" >}} is built on top of other software, such as OpenTelemetry Collector and Prometheus. If upstream software breaks compatibility, we may need to reflect this in {{< param "PRODUCT_NAME" >}}.

We try whenever possible to resolve these issues without breaking compatibility.

[semantic versioning]: https://semver.org/
[alloy-mixin]: https://github.com/grafana/alloy/tree/main/operations/alloy-mixin
