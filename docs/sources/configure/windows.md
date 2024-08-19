---
canonical: https://grafana.com/docs/alloy/latest/configure/windows/
aliases:
  - ../tasks/configure/configure-windows/ # /docs/alloy/latest/tasks/configure/configure-windows/
description: Learn how to configure Grafana Alloy on Windows
menuTitle: Windows
title: Configure Grafana Alloy on Windows
weight: 500
---

# Configure {{% param "FULL_PRODUCT_NAME" %}} on Windows

To configure {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps:

1. Edit the default configuration file at `%PROGRAMFILES%\GrafanaLabs\Alloy\config.alloy`.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   1. Open the Windows Services manager (`services.msc`):

      1. Right click on the Start Menu and select **Run**.

      1. Type `services.msc` and click **OK**.

   1. Right click on the service called **{{< param "PRODUCT_NAME" >}}**.

   1. Click on **All Tasks > Restart**.

## Change command-line arguments

By default, the {{< param "PRODUCT_NAME" >}} service will launch and pass the following arguments to the {{< param "PRODUCT_NAME" >}} binary:

* `run`
* `%PROGRAMFILES%\GrafanaLabs\Alloy\config.alloy`
* `--storage.path=%PROGRAMDATA%\GrafanaLabs\Alloy\data`

To change the set of command-line arguments passed to the {{< param "PRODUCT_NAME" >}} binary, perform the following steps:

1. Open the Registry Editor:

   1. Right click on the Start Menu and select **Run**.

   1. Type `regedit` and click **OK**.

1. Navigate to the key at the path `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.

1. Double-click on the value called **Arguments***.

1. In the dialog box, enter the new set of arguments to pass to the {{< param "PRODUCT_NAME" >}} binary.
   Make sure that each argument is on its own line.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   1. Open the Windows Services manager (`services.msc`):

      1. Right click on the Start Menu and select **Run**.

      1. Type `services.msc` and click **OK**.

   1. Right click on the service called **{{< param "PRODUCT_NAME" >}}**.

   1. Click on **All Tasks > Restart**.

## Expose the UI to other machines

By default, {{< param "PRODUCT_NAME" >}} listens on the local network for its HTTP
server. This prevents other machines on the network from being able to access
the [UI for debugging][UI].

To expose the UI to other machines, complete the following steps:

1. Follow [Change command-line arguments](#change-command-line-arguments) to edit command line flags passed to {{< param "PRODUCT_NAME" >}}.

1. Add the following command line argument:

   ```shell
   --server.http.listen-addr=LISTEN_ADDR:12345
   ```

   Replace the following:

   * _`<LISTEN_ADDR>`_: An IP address which other machines on the network have access to.
     For example, the IP address of the machine {{< param "PRODUCT_NAME" >}} is running on.

     To listen on all interfaces, replace _`<LISTEN_ADDR>`_ with `0.0.0.0`.

[UI]: ../../troubleshoot/debug/#alloy-ui
