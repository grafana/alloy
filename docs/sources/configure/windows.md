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

   1. Open the Windows Services manager:

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

1. In the dialog box, enter the arguments to pass to the {{< param "PRODUCT_NAME" >}} binary.
   Make sure that each argument is on its own line.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   1. Open the Windows Services manager:

      1. Right click on the Start Menu and select **Run**.

      1. Type `services.msc` and click **OK**.

   1. Right click on the service called **{{< param "PRODUCT_NAME" >}}**.

   1. Click on **All Tasks > Restart**.

## Change environment variable values

The Go runtime provides several ways to modify the execution of a binary using [environment variables][environment].

To change the environment variables used by {{< param "PRODUCT_NAME" >}}, perform the following steps.

1. Open the Registry Editor:

   1. Right click on the Start Menu and select **Run**.

   1. Type `regedit` and click **OK**.

1. Navigate to the key at the path `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.

1. Double-click on the multi-string value called **Environment***.

1. In the dialog box, enter the environment variable values to pass to the {{< param "PRODUCT_NAME" >}} binary.
   Make sure that each variable is on its own line.

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

## Configure Windows permissions

To effectively monitor Windows telemetry with {{< param "PRODUCT_NAME" >}}, the user account you use to run {{< param "PRODUCT_NAME" >}} requires specific access permissions.
These permissions ensure {{< param "PRODUCT_NAME" >}} can collect the necessary data, manage its local storage, and communicate with other services.
The exact permissions depend on your system's security configuration and the specific telemetry you need to collect.

### Windows security groups

To collect common Windows telemetry, for example event logs and performance counters, the user account you use to run {{< param "PRODUCT_NAME" >}} should be a member of the following [Windows security groups](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups).
These groups provide the minimum required read access.

* [Event Log Readers](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#event-log-readers)
  : This group allows members to read data from local event logs, including Application, System, Security, and other custom logs.
  This is essential for any Alloy configuration that collects Windows Event Log data.
* [Performance Monitor Users](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#performance-monitor-users)
  : This group allows non-administrator users to access performance counter data.
  This group is important for {{< param "PRODUCT_NAME" >}} components that collect Windows performance metrics, for example CPU, memory, disk I/O, and network usage.
* [Performance Log Users](http://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#performance-log-users)
  : This group is used to schedule logging of performance counter data and manage performance alerts.
  Performance Log Users is necessary for advanced or historical data collection scenarios, particularly those that involve the Windows Data Collector Sets.

### File system and network permissions

Beyond the standard Windows groups, {{< param "PRODUCT_NAME" >}} requires some specific permissions for its operational functions:

* Storage directory permissions
  : {{< param "PRODUCT_NAME" >}} needs [read, write, and modify permissions](https://learn.microsoft.com/windows/security/identity-protection/access-control/access-control) to manage files and directories within its data storage location.
  The default location for the data storage is `%PROGRAMDATA%\GrafanaLabs\Alloy\data`.
* Application log file read permissions
  : If you configure {{< param "PRODUCT_NAME" >}} to read application log files directly from disk, the user account you use to run {{< param "PRODUCT_NAME" >}} must have read access to those log files and their containing directories.
  You may need to modify the [Access Control Lists](https://learn.microsoft.com/windows/win32/secauthz/access-control-lists) for these resources or add the {{< param "PRODUCT_NAME" >}} service account to a custom group that has these permissions.
* Network access for telemetry destinations
  : {{< param "PRODUCT_NAME" >}} needs network connectivity and, if applicable, proxy configuration, to communicate with its configured telemetry endpoints.
    This includes source endpoints for scraping metrics from Prometheus exporters and pulling logs from remote APIs and destination endpoints for writing metrics to Prometheus or Grafana Cloud, and sending logs to Loki.
    Make sure your firewall rules allow outbound connections from the {{< param "PRODUCT_NAME" >}} host to these destinations on the necessary ports.
* Registry access
  : The user account you use to run {{< param "PRODUCT_NAME" >}} may need access to the [Windows Registry](https://learn.microsoft.com/windows/win32/sysinfo/registry-key-security-and-access-rights) to configure things like [environment variables](https://grafana.com/docs/alloy/latest/configure/windows/#change-environment-variable-values).
* UI port listening permission
  : If you want to enable the {{< param "PRODUCT_NAME" >}} UI, the user account you use to run {{< param "PRODUCT_NAME" >}} must have [permission to listen](https://learn.microsoft.com/windows/security/operating-system-security/network-security/windows-firewall/rules) on the configured UI port.
    The default port is `12345`.
* Run as a service permission
  : By default, {{< param "PRODUCT_NAME" >}} is installed and run as a Windows Service.
  The user account you use to run {{< param "PRODUCT_NAME" >}} must have the [`Log on as a service`](https://learn.microsoft.com/previous-versions/windows/it-pro/windows-10/security/threat-protection/security-policy-settings/log-on-as-a-service) user right.
* Temporary directory management
  : Depending on how you configure components and data processing, {{< param "PRODUCT_NAME" >}} might require permissions to create, read, and write temporary files in the system's designated temporary directories.
* Process and service enumeration
  : If you are using the process or service collectors within the integrated Windows Exporter, the user account you use to run {{< param "PRODUCT_NAME" >}} must have permissions to enumerate all running processes and services on the system.

[UI]: ../../troubleshoot/debug/#alloy-ui
[environment]: ../../reference/cli/environment-variables/
