---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/macos/
aliases:
  - ../../get-started/install/macos/ # /docs/alloy/latest/get-started/install/macos/
description: Learn how to install Grafana Alloy on macOS
menuTitle: macOS
title: Install Grafana Alloy on macOS
weight: 400
---

# Install {{% param "FULL_PRODUCT_NAME" %}} on macOS

You can install {{< param "PRODUCT_NAME" >}} on macOS with Homebrew.

{{< admonition type="note" >}}
The default prefix for Homebrew on Intel is `/usr/local`.
The default prefix for Homebrew on Apple Silicon is `/opt/Homebrew`.

To verify the default prefix for Homebrew on your computer, open a terminal window and type `brew --prefix`.
{{< /admonition >}}

## Before you begin

* Install [Homebrew][] on your computer.

## Install

To install {{< param "PRODUCT_NAME" >}} on macOS, run the following commands in a terminal window.

1. Add the Grafana Homebrew tap:

   ```shell
   brew tap grafana/grafana
   ```

1. Install {{< param "PRODUCT_NAME" >}}:

   ```shell
   brew install grafana/grafana/alloy
   ```

## Upgrade

To upgrade {{< param "PRODUCT_NAME" >}} on macOS, run the following commands in a terminal window.

1. Upgrade {{< param "PRODUCT_NAME" >}}:

   ```shell
   brew upgrade grafana/grafana/alloy
   ```

1. Restart {{< param "PRODUCT_NAME" >}}:

   ```shell
   brew services restart alloy
   ```

## Uninstall

To uninstall {{< param "PRODUCT_NAME" >}} on macOS, run the following command in a terminal window:

```shell
brew uninstall grafana/grafana/alloy
```

## Next steps

- [Run {{< param "PRODUCT_NAME" >}}][Run]
- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Homebrew]: https://brew.sh
[Run]: ../../run/macos/
[Configure]: ../../../configure/macos/
