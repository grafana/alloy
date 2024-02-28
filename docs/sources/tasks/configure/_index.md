---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/configure/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/configure/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/configure/
- /docs/grafana-cloud/send-data/agent/flow/tasks/configure/
# Previous page aliases for backwards compatibility:  
- /docs/grafana-cloud/agent/flow/setup/configure/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/configure/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/configure/
- /docs/grafana-cloud/send-data/agent/flow/setup/configure/
- ../setup/configure/ # /docs/agent/latest/flow/setup/configure/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/configure/
description: Configure Grafana Agent Flow after it is installed
menuTitle: Configure
title: Configure Grafana Agent Flow
weight: 90
---

# Configure {{% param "PRODUCT_NAME" %}}

You can configure {{< param "PRODUCT_NAME" >}} after it is [installed][Install]. 
The default River configuration file for {{< param "PRODUCT_NAME" >}} is located at:

* Linux: `/etc/grafana-agent-flow.river`
* macOS: `$(brew --prefix)/etc/grafana-agent-flow/config.river`
* Windows: `C:\Program Files\Grafana Agent Flow\config.river`

This section includes information that helps you configure {{< param "PRODUCT_NAME" >}}.

{{< section >}}

{{% docs/reference %}}
[Install]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/install/"
[Install]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/get-started/install/"
{{% /docs/reference %}}
