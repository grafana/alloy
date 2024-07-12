---
canonical: https://grafana.com/docs/alloy/latest/get-started/community_components/
description: Learn about community components
title: Community components
weight: 100
---

# Community components

__Community components__ are [components][Components] implemented and maintained by the community.

While Grafana does not offer commercial support for these components, they still undergo acceptance and review by the {{< param "PRODUCT_NAME" >}} development team before being added to the repository.

To use these community components, you must explicitly pass the `--feature.community-components.enabled` flag to the `run` command.

__Community components__ don't have a stability level. They aren't covered by our [backward compatibility strategy][backward-compatibility].

{{< admonition type="warning" >}}
__Community components__ without a maintainer may be disabled or removed if the components prevent or block the development of {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

[Components]: ../components/
[backward-compatibility]: ../../introduction/backward-compatibility/