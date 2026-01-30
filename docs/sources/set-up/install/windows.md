---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/windows/
aliases:
  - ../../get-started/install/windows/ # /docs/alloy/latest/get-started/install/windows/
description: Learn how to install Grafana Alloy on Windows
menuTitle: Windows
title: Install Grafana Alloy on Windows
weight: 300
---

# Install {{% param "FULL_PRODUCT_NAME" %}} on Windows

You can install {{< param "PRODUCT_NAME" >}} on Windows as a standard graphical install, a WinGet install, or a silent install.

## Standard graphical install

To do a standard graphical install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [releases page][releases] on GitHub.
1. Scroll down to the **Assets** section.
1. Download `alloy-installer-windows-amd64.exe` or download and extract `alloy-installer-windows-amd64.exe.zip`.
1. Double-click on `alloy-installer-windows-amd64.exe` to install {{< param "PRODUCT_NAME" >}}.

The installer places {{< param "PRODUCT_NAME" >}} in the default directory `%PROGRAMFILES%\GrafanaLabs\Alloy`.

## WinGet install

To install {{< param "PRODUCT_NAME" >}} with WinGet, perform the following steps.

1. Make sure that the [WinGet package manager](https://learn.microsoft.com/windows/package-manager/winget/) is installed.
1. Run the following command.

   {{< code >}}

   ```cmd
   winget install GrafanaLabs.Alloy
   ```

   ```powershell
   winget install GrafanaLabs.Alloy
   ```

   {{< /code >}}

## Silent install

To do a silent install of {{< param "PRODUCT_NAME" >}} on Windows, perform the following steps.

1. Navigate to the [releases page][releases] on GitHub.
1. Scroll down to the **Assets** section.
1. Download `alloy-installer-windows-amd64.exe` or download and extract `alloy-installer-windows-amd64.exe.zip`.
1. Run the following command as Administrator.

   {{< code >}}

   ```cmd
   <PATH>\alloy-installer-windows-amd64.exe /S
   ```

   ```powershell
   & <PATH>\alloy-installer-windows-amd64.exe /S
   ```

   {{< /code >}}

   Replace the following:

   - _`<PATH_TO_INSTALLER>`_: The path to the uncompressed installer executable.

### Silent install options

- `/CONFIG=<path>` Path to the configuration file. Default: `$INSTDIR\config.alloy`
- `/DISABLEREPORTING=<yes|no>` Disable [data collection][]. Default: `no`
- `/DISABLEPROFILING=<yes|no>` Disable profiling endpoint. Default: `no`
- `/ENVIRONMENT="KEY=VALUE\0KEY2=VALUE2"` Define environment variables for Windows Service. Default: ``
- `/RUNTIMEPRIORITY="normal|below_normal|above_normal|high|idle|realtime"` Set the runtime priority of the {{< param "PRODUCT_NAME" >}} process. Default: `normal`
- `/STABILITY="generally-available|public-preview|experimental"` Set the stability level of {{< param "PRODUCT_NAME" >}}. Default: `generally-available`
- `/USERNAME="<username>"` Set the fully qualified user that Windows uses to run the service. Default: `NT AUTHORITY\LocalSystem`
- `/PASSWORD="<password>"` Set the password of the user that Windows uses to run the service. This isn't required for standard Windows Service Accounts like LocalSystem. Default: ``

{{< admonition type="note" >}}
The `--windows.priority` flag is in [public preview][stability] and isn't covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.
The `/RUNTIMEPRIORITY` installation option sets this flag, and if {{< param "PRODUCT_NAME" >}} isn't running with an appropriate stability level, it fails to start.

[stability]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../../../introduction/backward-compatibility/

{{< /admonition >}}

## Service configuration

{{< param "PRODUCT_NAME" >}} uses the Windows registry key `HKLM\Software\GrafanaLabs\Alloy` for service configuration.

- `Arguments`: Type `REG_MULTI_SZ`. Each value represents a binary argument for the {{< param "PRODUCT_NAME" >}} binary.
- `Environment`: Type `REG_MULTI_SZ`. Each value represents an environment value `KEY=VALUE` for the {{< param "PRODUCT_NAME" >}} binary.

## Uninstall

Uninstalling {{< param "PRODUCT_NAME" >}} stops the service and removes it from disk.
This includes any configuration files in the installation directory.

### Standard graphical uninstall

To uninstall {{< param "PRODUCT_NAME" >}}, use Add or Remove Programs or run the following command as Administrator.

{{< code >}}

```cmd
%PROGRAMFILES%\GrafanaLabs\Alloy\uninstall.exe
```

```powershell
& ${env:PROGRAMFILES}\GrafanaLabs\Alloy\uninstall.exe
```

{{< /code >}}

### Uninstall with WinGet

To uninstall {{< param "PRODUCT_NAME" >}} with WinGet, run the following command.

{{< code >}}

```cmd
winget uninstall GrafanaLabs.Alloy
```

```powershell
winget uninstall GrafanaLabs.Alloy
```

{{< /code >}}

### Silent uninstall

To silently uninstall {{< param "PRODUCT_NAME" >}}, run the following command as Administrator.

{{< code >}}

```cmd
%PROGRAMFILES%\GrafanaLabs\Alloy\uninstall.exe /S
```

```powershell
& ${env:PROGRAMFILES}\GrafanaLabs\Alloy\uninstall.exe /S
```

{{< /code >}}

## Next steps

- [Run {{< param "PRODUCT_NAME" >}}][Run]
- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[releases]: https://github.com/grafana/alloy/releases
[data collection]: ../../../data-collection/
[Run]: ../../run/windows/
[Configure]: ../../../configure/windows/
