---
canonical: https://grafana.com/docs/alloy/latest/access_permissions/windows/
description: Set access and permissions for a Grafana Alloy installation on Windows with a dedicated service account, security groups, and filesystem permissions
menuTitle: Windows
title: Access and permissions for Grafana Alloy on Windows
weight: 300
---

# Access and permissions for {{% param "FULL_PRODUCT_NAME" %}} on Windows

{{< param "PRODUCT_NAME" >}} requires read access to Windows Event Logs, performance counters, application log files, and credentials for observability backends.
The Windows installer registers the {{< param "PRODUCT_NAME" >}} service to run as `LOCAL SYSTEM`.
Set the service account, security group membership, and filesystem permissions to match the components in your configuration.

## Run as a dedicated service account

Create a dedicated Windows service account and assign it to the {{< param "PRODUCT_NAME" >}} service.

### Required user rights

The service account needs the [`Log on as a service`](https://learn.microsoft.com/previous-versions/windows/it-pro/windows-10/security/threat-protection/security-policy-settings/log-on-as-a-service) user right.
Windows requires this right for any account that runs a service.

Assign the right in the Local Security Policy editor at `secpol.msc` under **Security Settings > Local Policies > User Rights Assignment**.

### Assign the service account

Stop the service, assign the account, and start the service again.

Run these commands in an elevated Command Prompt or PowerShell session:

{{< code >}}

```cmd
sc stop Alloy
sc config Alloy obj= "DOMAIN\username" password= "password"
sc start Alloy
```

```powershell
Stop-Service Alloy
sc.exe config Alloy obj= "DOMAIN\username" password= "password"
Start-Service Alloy
```

{{< /code >}}

Replace `DOMAIN\username` and `password` with the service account credentials.
For a local account, use `COMPUTERNAME\username` or `.\username`.

You can also open **Services** (`services.msc`), open **Grafana Alloy** properties, select the **Log On** tab, and choose **This account**.

If you haven't installed {{< param "PRODUCT_NAME" >}} yet, you can set the service account during a silent install with `/USERNAME` and `/PASSWORD`.
Refer to [Install {{< param "PRODUCT_NAME" >}} on Windows][install-windows] for those options.

## Windows security groups

Add the service account to [Windows security groups](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups) based on what you collect:

- **[Event Log Readers](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#event-log-readers)**: Read Application, System, Security, and custom event logs.
  Required for Windows Event Log collection.

- **[Performance Monitor Users](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#performance-monitor-users)**: Read performance counter data for CPU, memory, disk I/O, and network usage.
  Required for Windows performance metrics.

- **[Performance Log Users](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#performance-log-users)**: Manage Data Collector Sets and performance counter logs.
  Required for advanced or historical data collection.

## File system and network permissions

Grant read, write, and modify permissions on `%PROGRAMDATA%\GrafanaLabs\Alloy\data`, where {{< param "PRODUCT_NAME" >}} stores its write-ahead log and runtime data.

If {{< param "PRODUCT_NAME" >}} reads application log files from disk, grant the service account read access to those files and their parent directories through [Access Control Lists][acl] or a custom group.

{{< param "PRODUCT_NAME" >}} needs outbound network access to its telemetry endpoints, such as Prometheus remote write, Loki, and OTLP.
Allow outbound connections from the host on the ports your configuration uses.

The service account may need read access to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy` to read [environment variables][configure-windows] and [command-line arguments][configure-windows].
Refer to [Registry key security and access rights][registry-security] for details.

If you enable the {{< param "PRODUCT_NAME" >}} UI, the service account needs [permission to listen][firewall-rules] on the configured port.
The default port is `12345`.

Some components write temporary files in the system temp directories.
If you use the process or service collectors in the integrated Windows Exporter, the service account also needs permission to enumerate processes and services.

## Restrict the HTTP server

By default, {{< param "PRODUCT_NAME" >}} binds its HTTP server to `127.0.0.1:12345`.
Expose the endpoint only when you need remote access to the UI or metrics, and add authentication or TLS when you do.
Refer to the [`http` block][http-block] for configuration options.

## Components that require elevated access

Windows deployments rarely need the elevated Linux capabilities described in [Components that require elevated access][elevated-access].
Review that table if your configuration includes eBPF or host-level collectors on other platforms in the same fleet.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}} on Windows][configure-windows]
- [Monitor Windows with {{< param "PRODUCT_NAME" >}}][monitor-windows]
- [Collect and forward data][collect]

[install-windows]: ../../set-up/install/windows/
[configure-windows]: ../../configure/windows/
[monitor-windows]: ../../monitor/monitor-windows/
[collect]: ../../collect/
[elevated-access]: ../#components-that-require-elevated-access
[http-block]: ../../reference/config-blocks/http/
[acl]: https://learn.microsoft.com/windows/win32/secauthz/access-control-lists
[registry-security]: https://learn.microsoft.com/windows/win32/sysinfo/registry-key-security-and-access-rights
[firewall-rules]: https://learn.microsoft.com/windows/security/operating-system-security/network-security/windows-firewall/rules
