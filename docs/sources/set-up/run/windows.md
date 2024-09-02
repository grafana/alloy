---
canonical: https://grafana.com/docs/alloy/latest/set-up/run/windows/
aliases:
  - ../../get-started/run/windows/ # /docs/alloy/latest/get-started/run/windows/
description: Learn how to run Grafana Alloy on Windows
menuTitle: Windows
title: Run Grafana Alloy on Windows
weight: 500
---

# Run {{% param "FULL_PRODUCT_NAME" %}} on Windows

{{< param "PRODUCT_NAME" >}} is [installed][InstallWindows] as a Windows Service.
The service is configured to automatically run on startup.

To verify that {{< param "PRODUCT_NAME" >}} is running as a Windows Service:

1. Open the Windows Services manager (services.msc):

    1. Right click on the Start Menu and select **Run**.

    1. Type: `services.msc` and click **OK**.

1. Scroll down to find the **{{< param "PRODUCT_NAME" >}}** service and verify that the **Status** is **Running**.

## View {{% param "PRODUCT_NAME" %}} logs

When running on Windows, {{< param "PRODUCT_NAME" >}} writes its logs to Windows Event Logs with an event source name of **{{< param "PRODUCT_NAME" >}}**.

To view the logs, perform the following steps:

1. Open the Event Viewer:

    1. Right click on the Start Menu and select **Run**.

    1. Type `eventvwr` and click **OK**.

1. In the Event Viewer, click on **Windows Logs > Application**.

1. Search for events with the source **{{< param "FULL_PRODUCT_NAME" >}}**.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[InstallWindows]: ../../install/windows/
[Configure]: ../../../configure/windows/
