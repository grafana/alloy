---
canonical: https://Grafana.com/docs/alloy/latest/get-started/components/community-components/
aliases:
  - ./community_components/ # /docs/alloy/latest/get-started/community_components/
description: Learn about community components
title: Community components
weight: 80
---

# Community components

**Community components** are [components][Components] that the community implements and maintains.

Grafana doesn't offer commercial support for these components, but the {{< param "PRODUCT_NAME" >}} development team reviews and accepts them before adding them to the repository.

To use these community components, pass the `--feature.community-components.enabled` flag to the `run` command.

**Community components** don't have a defined stability level and aren't covered by the [backward compatibility strategy][backward-compatibility].

{{< admonition type="warning" >}}
**Community components** without a maintainer may be disabled or removed if they block or prevent the development of {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Next steps

Learn more about components:

- [Component reference][] to find available components
- [Configure components][] to learn how to use components in your configuration
- [Choose a component][] for guidance on selecting the right component for your needs

[Components]: ../components/
[backward-compatibility]: ../../introduction/backward-compatibility/
[Component reference]: ../../../reference/components/
[Configure components]: ./configure-components/
[Choose a component]: ../../../collect/choose-component/
