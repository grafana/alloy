---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/binary/
aliases:
  - ../../get-started/install/binary/ # /docs/alloy/latest/get-started/install/binary/
description: Learn how to install Grafana Alloy as a standalone binary
menuTitle: Standalone
title: Install Grafana Alloy as a standalone binary
weight: 600
---

# Install {{% param "FULL_PRODUCT_NAME" %}} as a standalone binary

{{< param "PRODUCT_NAME" >}} is distributed as a standalone binary for the following operating systems and architectures:

* Linux: AMD64, ARM64
* Windows: AMD64
* macOS: AMD64 (Intel), ARM64 (Apple Silicon)
* FreeBSD: AMD64

## Download {{% param "PRODUCT_NAME" %}}

To download {{< param "PRODUCT_NAME" >}} as a standalone binary, perform the following steps.

1. Navigate to the current {{< param "PRODUCT_NAME" >}} [release][] page.

1. Scroll down to the **Assets** section.

1. Download the `alloy` zip file that matches your operating system and machine's architecture.

1. Extract the package contents into a directory.

1. If you are installing {{< param "PRODUCT_NAME" >}} on Linux, macOS, or FreeBSD, run the following command in a terminal:

   ```shell
   chmod +x <BINARY_PATH>
   ```

   Replace the following:
   - _`<BINARY_PATH>`_: The path to the extracted binary.

### BoringCrypto binaries

{{< admonition type="note" >}}
BoringCrypto support is in _Public preview_ and is only available for Linux with the AMD64 or ARM64 architecture.
{{< /admonition >}}

BoringCrypto binaries are published for Linux on AMD64 and ARM64 platforms. To
retrieve them, follow the steps above but search the `alloy-boringcrypto` ZIP
file that matches your Linux architecture.

## Next steps

- [Run {{< param "PRODUCT_NAME" >}}][Run]

[release]: https://github.com/grafana/alloy/releases
[Run]: ../../run/binary/
