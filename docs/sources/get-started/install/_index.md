---
aliases:
- /docs/grafana-cloud/agent/flow/get-started/install/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/install/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/install/
- /docs/grafana-cloud/send-data/agent/flow/get-started/install/
# Previous docs aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/setup/install/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/install/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/install/
- /docs/grafana-cloud/send-data/agent/flow/setup/install/
- /docs/sources/flow/install/
- ../setup/install/ # /docs/agent/latest/flow/setup/install/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/install/
description: Learn how to install Grafana Agent Flow
menuTitle: Install
title: Install Grafana Agent Flow
weight: 50
---

# Install {{% param "PRODUCT_NAME" %}}

You can install {{< param "PRODUCT_NAME" >}} on Docker, Kubernetes, Linux, macOS, or Windows.

The following architectures are supported:

- Linux: AMD64, ARM64
- Windows: AMD64
- macOS: AMD64 (Intel), ARM64 (Apple Silicon)
- FreeBSD: AMD64

{{< admonition type="note" >}}
Installing {{< param "PRODUCT_NAME" >}} on other operating systems is possible, but isn't recommended or supported.
{{< /admonition >}}

{{< section >}}

## Data collection

By default, {{< param "PRODUCT_NAME" >}} sends anonymous usage information to Grafana Labs. Refer to [data collection][] for more information
about what data is collected and how you can opt-out.

{{% docs/reference %}}
[data collection]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/data-collection.md"
[data collection]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/data-collection.md"
{{% /docs/reference %}}
