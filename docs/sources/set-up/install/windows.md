---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/windows/
aliases:
  - ../../get-started/install/windows/ # /docs/alloy/latest/get-started/install/windows/
description: Learn how to install Grafana Alloy on Windows
menuTitle: Windows
title: Install Grafana Alloy on Windows
weight: 500
---

# Install {{% param "FULL_PRODUCT_NAME" %}} on Windows

You can install {{< param "PRODUCT_NAME" >}} on Windows as a standard graphical install, or as a silent install.

## Standard graphical install

To do a standard graphical install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [latest release][latest] on GitHub.

1. Scroll down to the **Assets** section.

1. Download the file called `alloy-installer-windows-amd64.exe.zip`.

1. Extract the downloaded file.

1. Double-click on `alloy-installer-windows-amd64.exe` to install {{< param "PRODUCT_NAME" >}}.

{{< param "PRODUCT_NAME" >}} is installed into the default directory `%PROGRAMFILES64%\GrafanaLabs\Alloy`.

## Silent install

To do a silent install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [latest release][latest] on GitHub.

1. Scroll down to the **Assets** section.

1. Download the file called `alloy-installer-windows-amd64.exe.zip`.

1. Extract the downloaded file.

1. Run the following command in PowerShell or Command Prompt:

   ```cmd
   <PATH_TO_INSTALLER> /S
   ```

   Replace the following:

   - _`<PATH_TO_INSTALLER>`_: The path to the uncompressed installer executable.

### Silent install options

* `/CONFIG=<path>` Path to the configuration file. Default: `$INSTDIR\config.alloy`
* `/DISABLEREPORTING=<yes|no>` Disable [data collection][]. Default: `no`
* `/DISABLEPROFILING=<yes|no>` Disable profiling endpoint. Default: `no`
* `/ENVIRONMENT="KEY=VALUE\0KEY2=VALUE2"` Define environment variables for Windows Service. Default: ``
* `/RUNTIMEPRIORITY="normal|below_normal|above_normal|high|idle|realtime"` Set the runtime priority of the {{< param "PRODUCT_NAME" >}} process. Default: `normal`
* `/STABILITY="generally-available|public-preview|experimental"` Set the stability level of {{< param "PRODUCT_NAME" >}}. Default: `generally-available`
* `/USERNAME="<username>"` Set the fully qualified user that Windows will use to run the service. Default: `NT AUTHORITY\LocalSystem`
* `/PASSWORD="<password>"` Set the password of the user that Windows will use to run the service. This is not required for standard Windows Service Accounts like LocalSystem. Default: ``

{{< admonition type="note" >}}
The `--windows.priority` flag is in [Public preview][stability] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.
The `/RUNTIMEPRIORITY` installation option sets this flag, and if Alloy is not running with an appropriate stability level it will fail to start.

[stability]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../../../introduction/backward-compatibility/
{{< /admonition >}}

## Service Configuration

{{< param "PRODUCT_NAME" >}} uses the Windows Registry `HKLM\Software\GrafanaLabs\Alloy` for service configuration.

* `Arguments` (Type `REG_MULTI_SZ`) Each value represents a binary argument for alloy binary.
* `Environment` (Type `REG_MULTI_SZ`) Each value represents a environment value `KEY=VALUE` for alloy binary.

## Uninstall

You can uninstall {{< param "PRODUCT_NAME" >}} with Windows Remove Programs or `%PROGRAMFILES64%\GrafanaLabs\Alloy\uninstaller.exe`.
Uninstalling {{< param "PRODUCT_NAME" >}} stops the service and removes it from disk.
This includes any configuration files in the installation directory.

{{< param "PRODUCT_NAME" >}} can also be silently uninstalled by running `uninstall.exe /S` as Administrator.

## Next steps

* [Run {{< param "PRODUCT_NAME" >}}][Run]
* [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[latest]: https://github.com/grafana/alloy/releases/latest
[data collection]: ../../../data-collection/
[Run]: ../../run/windows/
[Configure]: ../../../configure/windows/
