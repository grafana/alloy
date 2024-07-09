---
canonical: https://grafana.com/docs/alloy/latest/concepts/community_components/
description: Learn about community components
title: Community components
weight: 100
---

# Community components

__Community components__ are [components][Components] implemented and maintained by the community.

While Grafana does not offer commercial support for these components, they undergo acceptance and review by {{< param "PRODUCT_NAME" >}}'s team before being added to the repository.

To use these community components, you need to explicitly pass the `--feature.community-components.enabled` flag to the `run` command.

{{< admonition type="warning" >}}
__Community components__ without an owner may be eventually disabled or removed if they preventing us from being able to continue work on Alloy.
{{< /admonition >}}

[Components]: ../components/