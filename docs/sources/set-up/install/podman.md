---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/podman/
description: Learn how to run Grafana Alloy in a Podman container
menuTitle: Podman
title: Run Grafana Alloy in a Podman container
weight: 700
---

# Run {{% param "FULL_PRODUCT_NAME" %}} in a Podman container

Podman is a container engine that runs without a daemon for developing, managing, and running Open Container Initiative (OCI) containers.
You can use Podman as a drop-in replacement for Docker to run {{< param "PRODUCT_NAME" >}}.

{{< param "PRODUCT_NAME" >}} is available as a Docker container image that you can use with Podman on the following platforms:

* Linux for AMD64 and ARM64.
* macOS for AMD64 (Intel) and ARM64 (Apple Silicon).
* Windows for AMD64.

{{< admonition type="note" >}}
On macOS and Windows, Podman runs containers in a Linux virtual machine managed by `podman machine`.
The container commands are the same across all platforms once the machine is running.
{{< /admonition >}}

## Before you begin

* Install [Podman][] or [Podman Desktop][] on your computer.
* On macOS or Windows, initialize and start the Podman machine:

  ```shell
  podman machine init
  podman machine start
  ```

* Create and save an {{< param "PRODUCT_NAME" >}} configuration file on your computer, for example:

  ```alloy
  logging {
    level  = "info"
    format = "logfmt"
  }
  ```

## Run a rootless Podman container

One of the key features of Podman is the ability to run containers without root privileges.
To run {{< param "PRODUCT_NAME" >}} as a rootless Podman container, run the following command in a terminal window:

```shell
podman run \
  -v <CONFIG_FILE_PATH>:/etc/alloy/config.alloy:Z \
  -p 12345:12345 \
  docker.io/grafana/alloy:latest \
    run --server.http.listen-addr=0.0.0.0:12345 --storage.path=/var/lib/alloy/data \
    /etc/alloy/config.alloy
```

Replace the following:

* _`<CONFIG_FILE_PATH>`_: The absolute path of the configuration file on your host system.

{{< admonition type="note" >}}
The `:Z` suffix on the volume mount is required on Linux systems with Security-Enhanced Linux enabled (such as Fedora, RHEL, and CentOS) to set the correct security context for the mounted file.

If you're running on macOS, Windows, or a Linux system without Security-Enhanced Linux, you can omit the `:Z` suffix.
{{< /admonition >}}

You can modify the last line to change the arguments passed to the {{< param "PRODUCT_NAME" >}} binary.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example.
If you don't pass this argument, the [debugging UI][UI] won't be available outside of the Podman container.
{{< /admonition >}}

## Run a Podman container with root privileges

If you need to run {{< param "PRODUCT_NAME" >}} with root privileges, for example to access host-level resources, run the following command:

```shell
sudo podman run \
  -v <CONFIG_FILE_PATH>:/etc/alloy/config.alloy:Z \
  -p 12345:12345 \
  docker.io/grafana/alloy:latest \
    run --server.http.listen-addr=0.0.0.0:12345 --storage.path=/var/lib/alloy/data \
    /etc/alloy/config.alloy
```

Replace the following:

* _`<CONFIG_FILE_PATH>`_: The absolute path of the configuration file on your host system.

## Run with systemd integration on Linux

On Linux, Podman integrates with systemd to manage containers as services.
To generate a systemd unit file for {{< param "PRODUCT_NAME" >}}:

1. Run the container with a name:

   ```shell
   podman run -d --name alloy \
     -v <CONFIG_FILE_PATH>:/etc/alloy/config.alloy:Z \
     -p 12345:12345 \
     docker.io/grafana/alloy:latest \
       run --server.http.listen-addr=0.0.0.0:12345 --storage.path=/var/lib/alloy/data \
       /etc/alloy/config.alloy
   ```

   Replace the following:

   * _`<CONFIG_FILE_PATH>`_: The absolute path of the configuration file on your host system.

1. Generate a systemd unit file:

   ```shell
   podman generate systemd --name alloy --files --new
   ```

1. Move the generated file to the systemd directory:

   ```shell
   mv container-alloy.service ~/.config/systemd/user/
   ```

1. Reload systemd and enable the service:

   ```shell
   systemctl --user daemon-reload
   systemctl --user enable --now container-alloy.service
   ```

## Use Podman Compose

If you prefer using Compose files, Podman supports Docker Compose files through `podman-compose`.

1. Create a `compose.yaml` file:

   ```yaml
   services:
     alloy:
       image: docker.io/grafana/alloy:latest
       ports:
         - "12345:12345"
       volumes:
         - <CONFIG_FILE_PATH>:/etc/alloy/config.alloy:Z
       command:
         - run
         - --server.http.listen-addr=0.0.0.0:12345
         - --storage.path=/var/lib/alloy/data
         - /etc/alloy/config.alloy
   ```

   Replace the following:

   * _`<CONFIG_FILE_PATH>`_: The absolute path of the configuration file on your host system.

1. Run the container:

   ```shell
   podman-compose up -d
   ```

## BoringCrypto images

{{< admonition type="note" >}}
BoringCrypto support is in _Public preview_ and is only available on AMD64 and ARM64 platforms.
{{< /admonition >}}

BoringCrypto images are published with every release starting with version 1.1:

* The current BoringCrypto image is published as `docker.io/grafana/alloy:boringcrypto`.
* A specific version of the BoringCrypto image is published as `docker.io/grafana/alloy:<VERSION>-boringcrypto`, such as `docker.io/grafana/alloy:v1.1.0-boringcrypto`.

## Verify

To verify that {{< param "PRODUCT_NAME" >}} is running successfully, navigate to <http://localhost:12345> and make sure the {{< param "PRODUCT_NAME" >}} [UI][] loads without error.

You can also check the container status:

```shell
podman ps
```

## Next steps

* [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Podman]: https://podman.io/
[Podman Desktop]: https://podman-desktop.io/
[run]: ../../../reference/cli/run/
[UI]: ../../../troubleshoot/debug/
[Configure]: ../../../configure/linux/
