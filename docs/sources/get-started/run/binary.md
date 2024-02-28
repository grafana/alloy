---
aliases:
   - /docs/grafana-cloud/agent/flow/get-started/run/binary/
   - /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/run/binary/
   - /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/run/binary/
   - /docs/grafana-cloud/send-data/agent/flow/get-started/run/binary/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/run/binary/
description: Learn how to run Grafana Agent Flow as a standalone binary
menuTitle: Standalone
title: Run Grafana Agent Flow as a standalone binary
weight: 600
---

# Run {{% param "PRODUCT_NAME" %}} as a standalone binary

If you [downloaded][InstallBinary] the standalone binary, you must run {{< param "PRODUCT_NAME" >}} from a terminal or command window.

## Start {{% param "PRODUCT_NAME" %}} on Linux, macOS, or FreeBSD

To start {{< param "PRODUCT_NAME" >}} on Linux, macOS, or FreeBSD, run the following command in a terminal window:

```shell
AGENT_MODE=flow <BINARY_PATH> run <CONFIG_PATH>
```

Replace the following:

* _`<BINARY_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary file.
* _`<CONFIG_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} configuration file.

## Start {{% param "PRODUCT_NAME" %}} on Windows

To start {{< param "PRODUCT_NAME" >}} on Windows, run the following commands in a command prompt:

```cmd
set AGENT_MODE=flow
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

1. To create a new user called `grafana-agent-flow` run the following command in a terminal window:

   ```shell
   sudo useradd --no-create-home --shell /bin/false grafana-agent-flow
   ```

1. Create a service file in `/etc/systemd/system` called `grafana-agent-flow.service` with the following contents:

   ```systemd
   [Unit]
   Description=Vendor-neutral programmable observability pipelines.
   Documentation=https://grafana.com/docs/agent/latest/flow/
   Wants=network-online.target
   After=network-online.target

   [Service]
   Restart=always
   User=grafana-agent-flow
   Environment=HOSTNAME=%H
   EnvironmentFile=/etc/default/grafana-agent-flow
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
    * _`<WORKING_DIRECTORY>`_: The path to a working directory, for example `/var/lib/grafana-agent-flow`.

1. Create an environment file in `/etc/default/` called `grafana-agent-flow` with the following contents:

   ```shell
   ## Path:
   ## Description: Grafana Agent Flow settings
   ## Type:        string
   ## Default:     ""
   ## ServiceRestart: grafana-agent-flow
   #
   # Command line options for grafana-agent
   #
   # The configuration file holding the Grafana Agent Flow configuration.
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

{{% docs/reference %}}
[InstallBinary]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/install/binary.md"
[InstallBinary]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/flow/get-started/install/binary.md"
[StartLinux]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/run/linux.md"
[StartLinux]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/flow/get-started/run/linux.md"
{{% /docs/reference %}}
