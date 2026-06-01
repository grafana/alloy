---
canonical: https://grafana.com/docs/alloy/latest/secure/harden-windows/
description: Harden a Grafana Alloy installation on Windows by configuring a dedicated service account, security groups, and filesystem permissions
menuTitle: Harden on Windows
title: Harden Grafana Alloy on Windows
weight: 300
---

# Harden {{% param "FULL_PRODUCT_NAME" %}} on Windows

This page describes how to harden {{< param "PRODUCT_NAME" >}} running as a Windows Service.
It covers the service account, Windows security groups, and filesystem permissions needed to run {{< param "PRODUCT_NAME" >}} with least-privilege access.

For general configuration tasks such as editing the configuration file, changing command-line arguments, and configuring environment variables, refer to [Configure {{< param "PRODUCT_NAME" >}} on Windows][configure-windows].

## Run as a dedicated service account

By default, {{< param "PRODUCT_NAME" >}} runs as the `LOCAL SYSTEM` account after installation.
`LOCAL SYSTEM` has broad access to the local machine and is more privileged than necessary for most {{< param "PRODUCT_NAME" >}} deployments.

To reduce privilege, create a dedicated Windows service account and configure the {{< param "PRODUCT_NAME" >}} service to use it.

### Required user rights

The service account must have the [`Log on as a service`](https://learn.microsoft.com/previous-versions/windows/it-pro/windows-10/security/threat-protection/security-policy-settings/log-on-as-a-service) user right assigned.
Windows requires this right for any account that runs a service.

Assign this right using the Local Security Policy editor at `secpol.msc` under **Security Settings > Local Policies > User Rights Assignment**.

## Windows security groups

To collect common Windows telemetry, such as event logs and performance counters, add the service account to the appropriate [Windows security groups](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups).
These groups provide the minimum required read access.

Add the service account to [Event Log Readers](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#event-log-readers) if your configuration collects Windows Event Log data.
This group allows reading data from local event logs including Application, System, Security, and custom logs.

Add the service account to [Performance Monitor Users](https://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#performance-monitor-users) if your configuration collects Windows performance metrics such as CPU, memory, disk I/O, and network usage.
This group allows non-administrator users to access performance counter data.

Add the service account to [Performance Log Users](http://learn.microsoft.com/windows-server/identity/ad-ds/manage/understand-security-groups#performance-log-users) if your configuration uses Windows Data Collector Sets for advanced or historical data collection.
This group allows scheduling performance counter logging and managing performance alerts.

## File system and network permissions

Beyond group membership, the service account requires specific permissions.

Grant read, write, and modify permissions on `%PROGRAMDATA%\GrafanaLabs\Alloy\data`.
This is where {{< param "PRODUCT_NAME" >}} stores its write-ahead log and other runtime data.

If {{< param "PRODUCT_NAME" >}} reads application log files directly from disk, grant the service account read access to those files and their parent directories.
Modify [Access Control Lists][acl] for those resources, or add the service account to a custom group that has read access.

{{< param "PRODUCT_NAME" >}} needs outbound network connectivity to reach its configured telemetry endpoints, such as Prometheus remote write, Loki, and OTLP endpoints.
Ensure firewall rules allow outbound connections from the host on the necessary ports.

The service account may need read access to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy` to read [environment variables][configure-windows] and [command-line arguments][configure-windows].
Refer to [Registry key security and access rights][registry-security] for details.

If you enable the {{< param "PRODUCT_NAME" >}} UI, the service account needs [permission to listen][firewall-rules] on the configured port.
The default port is `12345`.

Depending on the components and data processing you configure, {{< param "PRODUCT_NAME" >}} may need to create, read, and write temporary files in the system's designated temporary directories.

If you use the process or service collectors within the integrated Windows Exporter, the service account needs permission to enumerate all running processes and services on the system.

## Restrict the HTTP server

By default, {{< param "PRODUCT_NAME" >}} binds its HTTP server to `127.0.0.1:12345`, which is only reachable from the local machine.
Don't expose this endpoint to the network unless you have a specific requirement.
Apply authentication or TLS if you do expose it.

For configuration options, refer to the [`http` block][http-block].

## Next steps

- [Secure {{< param "PRODUCT_NAME" >}}][secure]: overview of all security areas
- [Configure {{< param "PRODUCT_NAME" >}} on Windows][configure-windows]: general Windows configuration
- [Harden {{< param "PRODUCT_NAME" >}} on Linux][harden-linux]
- [Harden {{< param "PRODUCT_NAME" >}} on Kubernetes][harden-kubernetes]

[configure-windows]: ../../configure/windows/
[harden-linux]: ../harden-linux/
[harden-kubernetes]: ../harden-kubernetes/
[secure]: ../
[http-block]: ../../reference/config-blocks/http/
[acl]: https://learn.microsoft.com/windows/win32/secauthz/access-control-lists
[registry-security]: https://learn.microsoft.com/windows/win32/sysinfo/registry-key-security-and-access-rights
[firewall-rules]: https://learn.microsoft.com/windows/security/operating-system-security/network-security/windows-firewall/rules
