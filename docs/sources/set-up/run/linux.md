---
canonical: https://grafana.com/docs/alloy/latest/set-up/run/linux/
aliases:
  - ../../get-started/run/linux/ # /docs/alloy/latest/get-started/run/linux/
description: Learn how to run Grafana Alloy on Linux
menuTitle: Linux
title: Run Grafana Alloy on Linux
weight: 300
---

# Run {{% param "FULL_PRODUCT_NAME" %}} on Linux

{{< param "PRODUCT_NAME" >}} is [installed][InstallLinux] as a [systemd][] service on Linux.

## Start {{% param "PRODUCT_NAME" %}}

To start {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
sudo systemctl start alloy
```

(Optional) To verify that the service is running, run the following command in a terminal window:

```shell
sudo systemctl status alloy
```

## Configure {{% param "PRODUCT_NAME" %}} to start at boot

To automatically run {{< param "PRODUCT_NAME" >}} when the system starts, run the following command in a terminal window:

```shell
sudo systemctl enable alloy.service
```

## Restart {{% param "PRODUCT_NAME" %}}

To restart {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
sudo systemctl restart alloy
```

## Stop {{% param "PRODUCT_NAME" %}}

To stop {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
sudo systemctl stop alloy
```

## View {{% param "PRODUCT_NAME" %}} logs

To view {{< param "PRODUCT_NAME" >}} log files, run the following command in a terminal window:

```shell
sudo journalctl -u alloy
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[InstallLinux]: ../../install/linux/
[systemd]: https://systemd.io/
[Configure]: ../../../configure/linux/
