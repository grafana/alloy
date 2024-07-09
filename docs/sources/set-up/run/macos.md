---
canonical: https://grafana.com/docs/alloy/latest/set-up/run/macos/
aliases:
  - ../../get-started/run/macos/ # /docs/alloy/latest/get-started/run/macos/
description: Learn how to run Grafana Alloy on macOS
menuTitle: macOS
title: Run Grafana Alloy on macOS
weight: 400
---

# Run {{% param "FULL_PRODUCT_NAME" %}} on macOS

{{< param "PRODUCT_NAME" >}} is [installed][InstallMacOS] as a launchd service on macOS.

## Start {{% param "PRODUCT_NAME" %}}

To start {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
brew services start alloy
```

{{< param "PRODUCT_NAME" >}} automatically runs when the system starts.

(Optional) To verify that the service is running, run the following command in a terminal window:

```shell
brew services info alloy
```

## Restart {{% param "PRODUCT_NAME" %}}

To restart {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
brew services restart alloy
```

## Stop {{% param "PRODUCT_NAME" %}}

To stop {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
brew services stop  alloy
```

## View {{% param "PRODUCT_NAME" %}} logs on macOS

By default, logs are written to `$(brew --prefix)/var/log/alloy.log` and `$(brew --prefix)/var/log/alloy.err.log`.

If you followed [Configure the {{< param "PRODUCT_NAME" >}} service][ConfigureService] and changed the path where logs are written, refer to your current copy of the {{< param "PRODUCT_NAME" >}} formula to locate your log files.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][ConfigureMacOS]

[InstallMacOS]: ../../install/macos/
[ConfigureMacOS]: ../../../configure/macos/
[ConfigureService]: ../../../configure/macos/#configure-the-alloy-service
