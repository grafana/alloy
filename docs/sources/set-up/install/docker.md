---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/docker/
aliases:
  - ../../get-started/install/docker/ # /docs/alloy/latest/get-started/install/docker/
description: Learn how to run Grafana Alloy in a Docker container
review_date: 2026-09-17
menuTitle: Docker
title: Run Grafana Alloy in a Docker container
weight: 350
---

# Run {{% param "FULL_PRODUCT_NAME" %}} in a Docker container

{{< param "PRODUCT_NAME" >}} is available as a Docker container image on the following platforms:

- [Linux containers][] for AMD64, ARM64, ppc64le, and s390x.
- macOS for AMD64 on Intel and ARM64 on Apple Silicon using [Docker Desktop][].
- [Windows containers][] for AMD64.

{{< admonition type="note" >}}
On macOS, Docker Desktop manages a Linux virtual machine transparently, so the Linux container commands work without modification.
{{< /admonition >}}

## Before you begin

- Install [Docker][] or [Docker Desktop][] on your computer.
- Create and save an {{< param "PRODUCT_NAME" >}} configuration file on your computer, for example:

  ```alloy
  logging {
    level  = "info"
    format = "logfmt"
  }
  ```

## Run a Linux Docker container

To run {{< param "PRODUCT_NAME" >}} as a Linux Docker container, run the following command in a terminal window:

```shell
docker run \
  -v <CONFIG_FILE_PATH>:/etc/alloy/config.alloy \
  -p 12345:12345 \
  grafana/alloy:latest \
    run --server.http.listen-addr=0.0.0.0:12345 --storage.path=/var/lib/alloy/data \
    /etc/alloy/config.alloy
```

Replace the following:

- _`<CONFIG_FILE_PATH>`_: The path of the configuration file on your host system.

Docker passes everything after the image name to the {{< param "PRODUCT_NAME" >}} binary as the `run` command and its arguments.
You can change them as needed.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example.
If you don't pass this argument, the [debugging UI][UI] won't be available outside the Docker container.

[UI]: ../../../troubleshoot/debug/#alloy-ui
{{< /admonition >}}

### BoringCrypto images

{{< admonition type="note" >}}
BoringCrypto support is in _Public preview_ and is only available for Linux containers on AMD64 and ARM64.
{{< /admonition >}}

Every release starting with version 1.1 includes BoringCrypto images:

- The `grafana/alloy:boringcrypto` tag always points to the most recent stable release.
- The `grafana/alloy:<VERSION>-boringcrypto` tag pins a specific version, for example `grafana/alloy:v1.1.0-boringcrypto`.

### Distroless images

> **EXPERIMENTAL**: Distroless images are [experimental][].
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.

[experimental]: https://grafana.com/docs/release-life-cycle/

{{< admonition type="note" >}}
BoringCrypto variants of distroless images are only available on AMD64 and ARM64 platforms.
{{< /admonition >}}

Distroless images use an Ubuntu root filesystem that contains only the packages {{< param "PRODUCT_NAME" >}} needs at runtime.
They use the same entrypoint, configuration path, and storage path as the standard image, so the `docker run` command is the same.

Every release includes distroless images:

- The `grafana/alloy:latest-distroless` tag always points to the most recent stable release.
- The `grafana/alloy:<VERSION>-distroless` tag pins a specific version.
- The `grafana/alloy:boringcrypto-distroless` tag always points to the most recent stable BoringCrypto release.
- The `grafana/alloy:<VERSION>-boringcrypto-distroless` tag pins a specific BoringCrypto version.

## Run a Windows Docker container

The Windows image uses a Windows Server 2022 base image, so the host must be a Windows Server 2022 system or a Windows version that supports `ltsc2022` containers.

To run {{< param "PRODUCT_NAME" >}} as a Windows Docker container, run the following command in a Command Prompt or PowerShell window:

{{< tabs >}}
{{< tab-content name="Command Prompt" >}}

```cmd
docker run ^
  -v "<CONFIG_FILE_PATH>:C:\Program Files\GrafanaLabs\Alloy\config.alloy" ^
  -p 12345:12345 ^
  grafana/alloy:windowsservercore-ltsc2022 ^
    run --server.http.listen-addr=0.0.0.0:12345 "--storage.path=C:\ProgramData\GrafanaLabs\Alloy\data" ^
    "C:\Program Files\GrafanaLabs\Alloy\config.alloy"
```

{{< /tab-content >}}
{{< tab-content name="PowerShell" >}}

```powershell
docker run `
  -v "<CONFIG_FILE_PATH>:C:\Program Files\GrafanaLabs\Alloy\config.alloy" `
  -p 12345:12345 `
  grafana/alloy:windowsservercore-ltsc2022 `
    run --server.http.listen-addr=0.0.0.0:12345 "--storage.path=C:\ProgramData\GrafanaLabs\Alloy\data" `
    "C:\Program Files\GrafanaLabs\Alloy\config.alloy"
```

{{< /tab-content >}}
{{< /tabs >}}

Replace the following:

- _`<CONFIG_FILE_PATH>`_: The path of the configuration file on your host system.

Docker passes everything after the image name to the {{< param "PRODUCT_NAME" >}} binary as the `run` command and its arguments.
You can change them as needed.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example.
If you don't pass this argument, the [debugging UI][debug] won't be available outside the Docker container.

[debug]: ../../../troubleshoot/debug/#alloy-ui
{{< /admonition >}}

Every release includes Windows images:

- The `grafana/alloy:windowsservercore-ltsc2022` tag always points to the most recent stable release.
- The `grafana/alloy:<VERSION>-windowsservercore-ltsc2022` tag pins a specific version.

## Verify

To verify that {{< param "PRODUCT_NAME" >}} is running successfully, navigate to <http://localhost:12345> and make sure the {{< param "PRODUCT_NAME" >}} [UI][] loads without error.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Linux containers]: #run-a-linux-docker-container
[Windows containers]: #run-a-windows-docker-container
[Docker]: https://www.docker.com/
[Docker Desktop]: https://www.docker.com/products/docker-desktop/
[run]: ../../../reference/cli/run/
[UI]: ../../../troubleshoot/debug/
[Configure]: ../../../configure/
