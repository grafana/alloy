---
canonical: https://grafana.com/docs/alloy/latest/release-notes/
description: Release notes for Grafana Alloy
menuTitle: Release notes
title: Release notes for Grafana Alloy
weight: 999
---

# Release notes for {{% param "FULL_PRODUCT_NAME" %}}

The release notes provide information about deprecations and breaking changes in {{< param "FULL_PRODUCT_NAME" >}}.

For a complete list of changes to {{< param "FULL_PRODUCT_NAME" >}}, with links to pull requests and related issues when available, refer to the [Changelog][].

[Changelog]: https://github.com/grafana/alloy/blob/main/CHANGELOG.md

## v1.3

### Breaking change: `remotecfg` block updated argument name from `metadata` to `attributes`

{{< admonition type="note" >}}
This feature is in [Public preview][] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.

[Public preview]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../introduction/backward-compatibility/
{{< /admonition >}}

The `remotecfg` block has an updated argument name from `metadata` to `attributes`.

## v1.2

### Breaking change: `remotecfg` block updated for Agent rename

{{< admonition type="note" >}}
This feature is in [Public preview][] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.

[Public preview]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../introduction/backward-compatibility/
{{< /admonition >}}

The `remotecfg` block has been updated to use [alloy-remote-config](https://github.com/grafana/alloy-remote-config)
over [agent-remote-config](https://github.com/grafana/agent-remote-config). This change
aligns `remotecfg` API terminology with Alloy and includes updated endpoints.