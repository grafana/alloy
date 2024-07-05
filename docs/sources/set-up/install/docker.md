---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/docker/
aliases:
  - ../../get-started/install/docker/ # /docs/alloy/latest/get-started/install/docker/
description: Learn how to install Grafana Alloy on Docker
menuTitle: Docker
title: Run Grafana Alloy in a Docker container
weight: 100
---

# Run {{% param "FULL_PRODUCT_NAME" %}} in a Docker container

{{< param "PRODUCT_NAME" >}} is available as a Docker container image on the following platforms:

* [Linux containers][] for AMD64 and ARM64.
* [Windows containers][] for AMD64.

## Before you begin

* Install [Docker][] on your computer.
* Create and save an {{< param "PRODUCT_NAME" >}} configuration file on your computer, for example:

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

You can modify the last line to change the arguments passed to the {{< param "PRODUCT_NAME" >}} binary.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example.
If you don't pass this argument, the [debugging UI][UI] won't be available outside of the Docker container.

[UI]: ../../../troubleshoot/debug/#alloy-ui
{{< /admonition >}}

### BoringCrypto images

{{< admonition type="note" >}}
BoringCrypto support is in _Public preview_ and is only available on AMD64 and ARM64 platforms.
{{< /admonition >}}

BoringCrypto images are published with every release starting with version
1.1:

* The latest BoringCrypto image is published as `grafana/alloy:boringcrypto`.
* A specific version of the BoringCrypto image is published as
  `grafana/alloy:<VERSION>-boringcrypto`, such as
  `grafana/alloy:v1.1.0-boringcrypto`.

## Run a Windows Docker container

To run {{< param "PRODUCT_NAME" >}} as a Windows Docker container, run the following command in a terminal window:

```shell
docker run \
  -v "<CONFIG_FILE_PATH>:C:\Program Files\GrafanaLabs\Alloy\config.alloy" \
  -p 12345:12345 \
  grafana/alloy:nanoserver-1809 \
    run --server.http.listen-addr=0.0.0.0:12345 "--storage.path=C:\ProgramData\GrafanaLabs\Alloy\data" \
    "C:\Program Files\GrafanaLabs\Alloy\config.alloy"
```

Replace the following:

- _`<CONFIG_FILE_PATH>`_: The path of the configuration file on your host system.

You can modify the last line to change the arguments passed to the {{< param "PRODUCT_NAME" >}} binary.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example above.
If you don't pass this argument, the [debugging UI][UI] won't be available outside of the Docker container.

[UI]: ../../../troubleshoot/debug/#alloy-ui
{{< /admonition >}}

## Verify

To verify that {{< param "PRODUCT_NAME" >}} is running successfully, navigate to <http://localhost:12345> and make sure the {{< param "PRODUCT_NAME" >}} [UI][] loads without error.

[Linux containers]: #run-a-linux-docker-container
[Windows containers]: #run-a-windows-docker-container
[Docker]: https://docker.io
[run]: ../../../reference/cli/run/
[UI]: ../../../troubleshoot/debug/#alloy-ui
