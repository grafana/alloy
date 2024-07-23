---
canonical: https://grafana.com/docs/alloy/latest/set-up/run/binary/
aliases:
  - ../../get-started/run/binary/ # /docs/alloy/latest/get-started/run/binary/
description: Learn how to run Grafana Alloy as a standalone binary
menuTitle: Standalone
title: Run Grafana Alloy as a standalone binary
weight: 600
---

# Run {{% param "FULL_PRODUCT_NAME" %}} as a standalone binary

If you [downloaded][InstallBinary] the standalone binary, you must run {{< param "PRODUCT_NAME" >}} from a terminal or command window.

Refer to the [run][] documentation for more information about the command line flags you can use when you run {{< param "PRODUCT_NAME" >}}.

## Start {{% param "PRODUCT_NAME" %}}

To start {{< param "PRODUCT_NAME" >}}, run the following command in a terminal or command window:

```shell
<BINARY_PATH> run <CONFIG_PATH>
```

Replace the following:

* _`<BINARY_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary file.
* _`<CONFIG_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} configuration file.

## Set up {{% param "PRODUCT_NAME" %}} as a Linux systemd service

You can set up and manage the standalone binary for {{< param "PRODUCT_NAME" >}} as a Linux systemd service.

{{< admonition type="note" >}}
These steps assume you have a default systemd and {{< param "PRODUCT_NAME" >}} configuration.
{{< /admonition >}}

1. To create a new user called `alloy` run the following command in a terminal window:

   ```shell
   sudo useradd --no-create-home --shell /bin/false alloy
   ```

1. Create a service file in `/etc/systemd/system` called `alloy.service` with the following contents:

   ```systemd
   [Unit]
   Description=Vendor-neutral programmable observability pipelines.
   Documentation=https://grafana.com/docs/alloy/
   Wants=network-online.target
   After=network-online.target

   [Service]
   Restart=always
   User=alloy
   Environment=HOSTNAME=%H
   EnvironmentFile=/etc/default/alloy
   WorkingDirectory=<WORKING_DIRECTORY>
   ExecStart=<BINARY_PATH> run $CUSTOM_ARGS --storage.path=<WORKING_DIRECTORY> $CONFIG_FILE
   ExecReload=/usr/bin/env kill -HUP $MAINPID
   TimeoutStopSec=20s
   SendSIGKILL=no

   [Install]
   WantedBy=multi-user.target
   ```

   Replace the following:

    * _`<BINARY_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary file.
    * _`<WORKING_DIRECTORY>`_: The path to a working directory, for example `/var/lib/alloy`.

1. Create an environment file in `/etc/default/` called `alloy` with the following contents:

   ```shell
   ## Path:
   ## Description: Grafana Alloy settings
   ## Type:        string
   ## Default:     ""
   ## ServiceRestart: alloy
   #
   # Command line options for alloy
   #
   # The configuration file holding the Grafana Alloy configuration.
   CONFIG_FILE="<CONFIG_PATH>"

   # User-defined arguments to pass to the run command.
   CUSTOM_ARGS=""

   # Restart on system upgrade. Defaults to true.
   RESTART_ON_UPGRADE=true
   ```

   Replace the following:

    * _`<CONFIG_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} configuration file.

1. To reload the service files, run the following command in a terminal window:

   ```shell
   sudo systemctl daemon-reload
   ```

1. Use the [Linux][StartLinux] systemd commands to manage your standalone Linux installation of {{< param "PRODUCT_NAME" >}}.

## View {{% param "PRODUCT_NAME" %}} logs

By default, {{% param "PRODUCT_NAME" %}} writes the output to `stdout` and errors to `stderr`.

To write the output and error logs to a file, you can use the redirection operator for your operating system. For example, the following command combines the standard output and standard errors into a single text file:

{{< code >}}

```linux
<BINARY_PATH> run <CONFIG_PATH> &> <OUTPUT_FILE>
```

```macos
<BINARY_PATH> run <CONFIG_PATH> &> <OUTPUT_FILE>
```

```windows
<BINARY_PATH> run <CONFIG_PATH> 1> <OUTPUT_FILE> 2>&1
```

{{< /code >}}

Replace the following:

* _`<BINARY_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary file.
* _`<CONFIG_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} configuration file.
* _`<OUTPUT_FILE>`_: The output filename.

[InstallBinary]: ../../install/binary/
[StartLinux]: ../linux/
[run]: ../../../reference/cli/run/
