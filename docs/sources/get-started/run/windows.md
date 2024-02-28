---
aliases:
  - /docs/grafana-cloud/agent/flow/get-started/run/windows/
  - /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/run/windows/
  - /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/run/windows/
  - /docs/grafana-cloud/send-data/agent/flow/get-started/run/windows/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/run/windows/
description: Learn how to run Grafana Agent Flow on Windows
menuTitle: Windows
title: Run Grafana Agent Flow on Windows
weight: 500
---

# Run {{% param "PRODUCT_NAME" %}} on Windows

{{< param "PRODUCT_NAME" >}} is [installed][InstallWindows] as a Windows Service. The service is configured to automatically run on startup.

To verify that {{< param "PRODUCT_NAME" >}} is running as a Windows Service:

1. Open the Windows Services manager (services.msc):

    1. Right click on the Start Menu and select **Run**.

    1. Type: `services.msc` and click **OK**.

1. Scroll down to find the **{{< param "PRODUCT_NAME" >}}** service and verify that the **Status** is **Running**.

## View {{% param "PRODUCT_NAME" %}} logs

When running on Windows, {{< param "PRODUCT_NAME" >}} writes its logs to Windows Event
Logs with an event source name of **{{< param "PRODUCT_NAME" >}}**.

To view the logs, perform the following steps:

1. Open the Event Viewer:

    1. Right click on the Start Menu and select **Run**.

    1. Type `eventvwr` and click **OK**.

1. In the Event Viewer, click on **Windows Logs > Application**.

1. Search for events with the source **{{< param "PRODUCT_NAME" >}}**.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

{{% docs/reference %}}
[InstallWindows]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/install/windows.md"
[InstallWindows]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/flow/get-started/install/windows.md"
[Configure]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure/configure-windows.md"
[Configure]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-windows.md"
{{% /docs/reference %}}
