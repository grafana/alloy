---
aliases:
- /docs/grafana-cloud/agent/flow/get-started/install/windows/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/install/windows/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/install/windows/
- /docs/grafana-cloud/send-data/agent/flow/get-started/install/windows/
# Previous docs aliases for backwards compatibility:
- ../../install/windows/ # /docs/agent/latest/flow/install/windows/
- /docs/grafana-cloud/agent/flow/setup/install/windows/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/install/windows/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/install/windows/
- /docs/grafana-cloud/send-data/agent/flow/setup/install/windows/
- ../../setup/install/windows/ # /docs/agent/latest/flow/setup/install/windows/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/install/windows/
description: Learn how to install Grafana Agent Flow on Windows
menuTitle: Windows
title: Install Grafana Agent Flow on Windows
weight: 500
---

# Install {{% param "PRODUCT_NAME" %}} on Windows

You can install {{< param "PRODUCT_NAME" >}} on Windows as a standard graphical install, or as a silent install.

## Standard graphical install

To do a standard graphical install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [latest release][latest] on GitHub.

1. Scroll down to the **Assets** section.

1. Download the file called `grafana-agent-flow-installer.exe.zip`.

1. Unzip the downloaded file.

1. Double-click on `grafana-agent-installer.exe` to install {{< param "PRODUCT_NAME" >}}.

{{< param "PRODUCT_NAME" >}} is installed into the default directory `C:\Program Files\Grafana Agent Flow`.

## Silent install

To do a silent install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [latest release][latest] on GitHub.

1. Scroll down to the **Assets** section.

1. Download the file called `grafana-agent-flow-installer.exe.zip`.

1. Unzip the downloaded file.

1. Run the following command in PowerShell or Command Prompt:

   ```cmd
   <PATH_TO_INSTALLER> /S
   ```

   Replace the following:

   - _`<PATH_TO_INSTALLER>`_: The path where the unzipped installer executable is located.

### Silent install options

* `/CONFIG=<path>` Path to the configuration file. Default: `$INSTDIR\config.river`
* `/DISABLEREPORTING=<yes|no>` Disable [data collection][]. Default: `no`
* `/DISABLEPROFILING=<yes|no>` Disable profiling endpoint. Default: `no`
* `/ENVIRONMENT="KEY=VALUE\0KEY2=VALUE2"` Define environment variables for Windows Service. Default: ``

## Service Configuration

{{< param "PRODUCT_NAME" >}} uses the Windows Registry `HKLM\Software\Grafana\Grafana Agent Flow` for service configuration.

* `Arguments` (Type `REG_MULTI_SZ`) Each value represents a binary argument for grafana-agent-flow binary.
* `Environment` (Type `REG_MULTI_SZ`) Each value represents a environment value `KEY=VALUE` for grafana-agent-flow binary.

## Uninstall

You can uninstall {{< param "PRODUCT_NAME" >}} with Windows Remove Programs or `C:\Program Files\Grafana Agent\uninstaller.exe`.
Uninstalling {{< param "PRODUCT_NAME" >}} stops the service and removes it from disk.
This includes any configuration files in the installation directory.

{{< param "PRODUCT_NAME" >}} can also be silently uninstalled by running `uninstall.exe /S` as Administrator.

## Next steps

- [Run {{< param "PRODUCT_NAME" >}}][Start]
- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[latest]: https://github.com/grafana/agent/releases/latest

{{% docs/reference %}}
[Run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/run/windows.md"
[Run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/get-started/run/windows.md"
[Configure]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure/configure-windows.md"
[Configure]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-windows.md"
[data collection]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/data-collection.md"
[data collection]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/data-collection.md"
{{% /docs/reference %}}
