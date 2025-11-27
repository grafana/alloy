---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/community-components/
aliases:
  - ../concepts/community-components/ # /docs/alloy/latest/concepts/community-components/
description: Learn about community components
title: Community components
weight: 50
---

# Community components

You learned how to create custom components by combining existing components in the previous section.
Now you'll learn about _community components_. These are specialized components that the {{< param "PRODUCT_NAME" >}} community develops and maintains.

Community components extend {{< param "PRODUCT_NAME" >}}'s capabilities with vendor-specific integrations and specialized functionality that may not be suitable for core components.
They follow the same component architecture you've learned about but have different support and stability characteristics.

## What are community components?

Community components are components that community members implement and maintain rather than the core {{< param "PRODUCT_NAME" >}} team.
They're particularly useful for:

1. **Vendor-specific integrations**: Components for services that Grafana Labs doesn't offer commercial support for.
1. **Specialized functionality**: Components that serve specific use cases or niche requirements.
1. **Experimental features**: New capabilities that need community validation before potential inclusion in core components.

Key characteristics of community components:

- **Community ownership**: The community implements, maintains, and supports them.
- **No commercial support**: Grafana Labs doesn't provide commercial support for these components.
- **Review process**: The {{< param "PRODUCT_NAME" >}} development team reviews and accepts them before adding them to the repository.
- **Stability level**: They don't have defined stability levels and aren't covered by the [backward compatibility strategy][backward-compatibility].

## Enable community components

Community components are disabled by default.
To use them, you must explicitly enable them when running {{< param "PRODUCT_NAME" >}}:

```bash
alloy run --feature.community-components.enabled config.alloy
```

This flag ensures that you consciously choose to use components that have different support characteristics than core components.
After enabling community components, you can use them the same way as any built-in component in your configuration.

{{< admonition type="warning" >}}
Grafana Labs may disable or remove **community components** without a maintainer if they block or prevent the development of {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Next steps

Now that you understand community components, explore how to find and use components in your configurations:

- [Component reference][] - Browse all available components including community components
- [Configure components][] - Learn how to use components in your configuration

For component selection guidance:

- [Choose a component][] - Get guidance on selecting the right component for your needs

[backward-compatibility]: ../introduction/backward-compatibility/
[Component reference]: ../../reference/components/
[Configure components]: ./configure-components/
[Choose a component]: ../../collect/choose-component/
