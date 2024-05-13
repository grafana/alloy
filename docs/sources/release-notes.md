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

## v1.2

### Breaking change: `remotecfg` API (Public preview) using the term `collector` over `agent`

The API has been updated to use `alloy-remote-config` over `agent-remote-config`. This change
aligns `remotecfg` API terminology with Alloy.