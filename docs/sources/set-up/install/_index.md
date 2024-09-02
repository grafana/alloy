---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/
aliases:
  - ../get-started/install/ # /docs/alloy/latest/get-started/install/
description: Learn how to install Grafana Alloy
menuTitle: Install
title: Install Grafana Alloy
weight: 100
---

# Install {{% param "FULL_PRODUCT_NAME" %}}

You can install {{< param "PRODUCT_NAME" >}} on Docker, Kubernetes, Linux, macOS, or Windows.

The following architectures are supported:

- **Linux**: AMD64, ARM64
- **Windows**: AMD64
- **macOS**: AMD64 on Intel, ARM64 on Apple Silicon
- **FreeBSD**: AMD64

{{< admonition type="note" >}}
Installing {{< param "PRODUCT_NAME" >}} on other operating systems is possible, but isn't recommended or supported.
{{< /admonition >}}

{{< section >}}

## Data collection

By default, {{< param "PRODUCT_NAME" >}} sends anonymous usage information to Grafana Labs.
Refer to [data collection][] for more information about what data Grafana collects and how you can opt-out.

[data collection]: "../../../../data-collection/
