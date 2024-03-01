---
canonical: https://grafana.com/docs/alloy/latest/get-started/run/linux/
description: Learn how to run Grafana Agent Flow on Linux
menuTitle: Linux
title: Run Grafana Agent Flow on Linux
weight: 300
---

# Run {{% param "PRODUCT_NAME" %}} on Linux

{{< param "PRODUCT_NAME" >}} is [installed][InstallLinux] as a [systemd][] service on Linux.

## Start {{% param "PRODUCT_NAME" %}}

To start {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
sudo systemctl start grafana-agent-flow
```

(Optional) To verify that the service is running, run the following command in a terminal window:

```shell
sudo systemctl status grafana-agent-flow
```

## Configure {{% param "PRODUCT_NAME" %}} to start at boot

To automatically run {{< param "PRODUCT_NAME" >}} when the system starts, run the following command in a terminal window:

```shell
sudo systemctl enable grafana-agent-flow.service
```

## Restart {{% param "PRODUCT_NAME" %}}

To restart {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
sudo systemctl restart grafana-agent-flow
```

## Stop {{% param "PRODUCT_NAME" %}}

To stop {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
sudo systemctl stop grafana-agent-flow
```

## View {{% param "PRODUCT_NAME" %}} logs on Linux

To view {{< param "PRODUCT_NAME" >}} log files, run the following command in a terminal window:

```shell
sudo journalctl -u grafana-agent-flow
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[InstallLinux]: ../../install/linux/
[systemd]: https://systemd.io/
[Configure]: ../../../tasks/configure/configure-linux/
