---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/configure/configure-windows/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/configure/configure-windows/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/configure/configure-windows/
- /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-windows/  
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/setup/configure/configure-windows/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/configure/configure-windows/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/configure/configure-windows/
- /docs/grafana-cloud/send-data/agent/flow/setup/configure/configure-windows/
- ../../setup/configure/configure-windows/ # /docs/agent/latest/flow/setup/configure/configure-windows/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/configure/configure-windows/
description: Learn how to configure Grafana Agent Flow on Windows
menuTitle: Windows
title: Configure Grafana Agent Flow on Windows
weight: 500
---

# Configure {{% param "PRODUCT_NAME" %}} on Windows

To configure {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps:

1. Edit the default configuration file at `C:\Program Files\Grafana Agent Flow\config.river`.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   1. Open the Windows Services manager (`services.msc`):

      1. Right click on the Start Menu and select **Run**.

      1. Type `services.msc` and click **OK**.

   1. Right click on the service called **{{< param "PRODUCT_NAME" >}}**.

   1. Click on **All Tasks > Restart**.

## Change command-line arguments

By default, the {{< param "PRODUCT_NAME" >}} service will launch and pass the
following arguments to the {{< param "PRODUCT_NAME" >}} binary:

* `run`
* `C:\Program Files\Grafana Agent Flow\config.river`
* `--storage.path=C:\ProgramData\Grafana Agent Flow\data`

To change the set of command-line arguments passed to the {{< param "PRODUCT_ROOT_NAME" >}}
binary, perform the following steps:

1. Open the Registry Editor:

   1. Right click on the Start Menu and select **Run**.

   1. Type `regedit` and click **OK**.

1. Navigate to the key at the path `HKEY_LOCAL_MACHINE\SOFTWARE\Grafana\Grafana Agent Flow`.

1. Double-click on the value called **Arguments***.

1. In the dialog box, enter the new set of arguments to pass to the {{< param "PRODUCT_ROOT_NAME" >}} binary.

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

1. Follow [Change command-line arguments](#change-command-line-arguments)
   to edit command line flags passed to {{< param "PRODUCT_NAME" >}}, including the
   following customizations:

    1. Add the following command line argument:

       ```shell
       --server.http.listen-addr=LISTEN_ADDR:12345
       ```

       Replace `LISTEN_ADDR` with an address which other machines on the
       network have access to, like the network IP address of the machine
       {{< param "PRODUCT_NAME" >}} is running on.

       To listen on all interfaces, replace `LISTEN_ADDR` with `0.0.0.0`.

{{% docs/reference %}}
[UI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#grafana-agent-flow-ui"
[UI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#grafana-agent-flow-ui"
{{% /docs/reference %}}

