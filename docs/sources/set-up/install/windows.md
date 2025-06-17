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

## Configure the Windows permissions

To effectively monitor Windows telemetry with {{< param "PRODUCT_NAME" >}}, the user account you use to run {{< param "PRODUCT_NAME" >}} requires specific access permissions.
These permissions ensure {{< param "PRODUCT_NAME" >}} can collect the necessary data, manage its local storage, and communicate with other services.
The exact permissions depend on your system's security configuration and the specific telemetry you need to collect.

### Windows security groups

To collect common Windows telemetry, for example event logs and performance counters, the user account you use to run {{< param "PRODUCT_NAME" >}} should be a member of the following Windows security groups.
These groups provide the necessary read access without granting excessive privileges.

* `Event Log Readers`: This group allows members to read data from local event logs, including Application, System, Security, and other custom logs.
  This is essential for any Alloy configuration that collects Windows Event Log data.
* `Performance Monitor Users`: This group allows non-administrator users to access performance counter data.
  This group is important for {{< param "PRODUCT_NAME" >}} components that collect Windows performance metrics, for example CPU, memory, disk I/O, and network usage.
* `Performance Log Users`: This group is used to schedule logging of performance counter data and manage performance alerts.
   Performance Log Users is necessary for advanced or historical data collection scenarios, particularly those that involve the Windows Data Collector Sets.

### File system and network permissions

Beyond the standard Windows groups, {{< param "PRODUCT_NAME" >}} requires some specific permissions for its operational functions:

* Storage directory permissions: {{< param "PRODUCT_NAME" >}} needs read, write, and modify permissions to manage files and directories within its data storage location.
  Default Location: %PROGRAMDATA%\GrafanaLabs\Alloy\data
* Application log file read permissions: If you configure {{< param "PRODUCT_NAME" >}} to read application log files directly from disk, the user account you use to run {{< param "PRODUCT_NAME" >}} must have read access to those log files and their containing directories.
  You may need to modify the Access Control Lists for these resources or add the {{< param "PRODUCT_NAME" >}} service account to a custom group that has these permissions.
* Network access for telemetry destinations: {{< param "PRODUCT_NAME" >}} needs network connectivity and, if applicable, proxy configuration, to communicate with its configured telemetry endpoints.
  This includes:
  * Source endpoints: For scraping metrics from Prometheus exporters and pulling logs from remote APIs.
  * Destination Endpoints: For writing metrics to Prometheus or Grafana Cloud, and sending logs to Loki.
  Make sure your firewall rules allow outbound connections from the {{< param "PRODUCT_NAME" >}} host to these destinations on the necessary ports.
* Registry access: The user account you use to run {{< param "PRODUCT_NAME" >}} may need access to the Windows Registry to configure things like [environment variables](https://grafana.com/docs/alloy/latest/configure/windows/#change-environment-variable-values).
* UI port listening permission: If you want to enable the {{< param "PRODUCT_NAME" >}} UI, the user account you use to run {{< param "PRODUCT_NAME" >}} must have permission to listen on the configured UI port.
  The default port is `12345`.
* `Run as a Service` permission: By default, {{< param "PRODUCT_NAME" >}} is installed and run as a Windows Service.
  The user account you use to run {{< param "PRODUCT_NAME" >}} must have the `Log on as a service` user right.
* Temporary directory management: Depending on how you configure components and data processing, {{< param "PRODUCT_NAME" >}} might require permissions to create, read, and write temporary files in the system's designated temporary directories.
* Process and service enumeration: If you are using the process or service collectors within the integrated Windows Exporter, the user account you use to run {{< param "PRODUCT_NAME" >}} must have permissions to enumerate all running processes and services on the system.

## Standard graphical install

To do a standard graphical install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [latest release][latest] on GitHub.

1. Scroll down to the **Assets** section.

1. Download the file called `alloy-installer-windows-amd64.exe.zip`.

1. Extract the downloaded file.

1. Double-click on `alloy-installer-windows-amd64.exe` to install {{< param "PRODUCT_NAME" >}}.

{{< param "PRODUCT_NAME" >}} is installed into the default directory `%PROGRAMFILES%\GrafanaLabs\Alloy`.

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

You can uninstall {{< param "PRODUCT_NAME" >}} with Windows Add or Remove Programs or `%PROGRAMFILES%\GrafanaLabs\Alloy\uninstall.exe`.
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
