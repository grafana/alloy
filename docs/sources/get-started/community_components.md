---
canonical: https://grafana.com/docs/alloy/latest/get-started/community_components/
description: Learn about community components
title: Community components
weight: 100
---

# Community components

__Community components__ are [components][Components] that the community implements and maintains.

Grafana doesn't offer commercial support for these components, but the {{< param "PRODUCT_NAME" >}} development team reviews and accepts them before adding them to the repository.

To use these community components, pass the `--feature.community-components.enabled` flag to the `run` command.

__Community components__ don't have a defined stability level and aren't covered by the [backward compatibility strategy][backward-compatibility].

{{< admonition type="warning" >}}
__Community components__ without a maintainer may be disabled or removed if they block or prevent the development of {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

[Components]: ../components/
[backward-compatibility]: ../../introduction/backward-compatibility/
