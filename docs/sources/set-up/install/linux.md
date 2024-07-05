---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/linux/
aliases:
  - ../../get-started/install/linux/ # /docs/alloy/latest/get-started/install/linux/
description: Learn how to install Grafana Alloy on Linux
menuTitle: Linux
title: Install Grafana Alloy on Linux
weight: 300
---

# Install {{% param "FULL_PRODUCT_NAME" %}} on Linux

You can install {{< param "PRODUCT_NAME" >}} as a systemd service on Linux.

## Before you begin

Some Debian-based cloud Virtual Machines don't have GPG installed by default.
To install GPG in your Linux Virtual Machine, run the following command in a terminal window.

```shell
sudo apt install gpg
```

## Install

To install {{< param "PRODUCT_NAME" >}} on Linux, run the following commands in a terminal window.

1. Import the GPG key and add the Grafana package repository.

   {{< code >}}
   ```debian-ubuntu
   sudo mkdir -p /etc/apt/keyrings/
   wget -q -O - https://apt.grafana.com/gpg.key | gpg --dearmor | sudo tee /etc/apt/keyrings/grafana.gpg > /dev/null
   echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" | sudo tee /etc/apt/sources.list.d/grafana.list
   ```

   ```rhel-fedora
   wget -q -O gpg.key https://rpm.grafana.com/gpg.key
   sudo rpm --import gpg.key
   echo -e '[grafana]\nname=grafana\nbaseurl=https://rpm.grafana.com\nrepo_gpgcheck=1\nenabled=1\ngpgcheck=1\ngpgkey=https://rpm.grafana.com/gpg.key\nsslverify=1
sslcacert=/etc/pki/tls/certs/ca-bundle.crt' | sudo tee /etc/yum.repos.d/grafana.repo
   ```

   ```suse-opensuse
   wget -q -O gpg.key https://rpm.grafana.com/gpg.key
   sudo rpm --import gpg.key
   sudo zypper addrepo https://rpm.grafana.com grafana
   ```
   {{< /code >}}

1. Update the repositories.

   {{< code >}}
   ```debian-ubuntu
   sudo apt-get update
   ```

   ```rhel-fedora
   yum update
   ```

   ```suse-opensuse
   sudo zypper update
   ```
   {{< /code >}}

1. Install {{< param "PRODUCT_NAME" >}}.

   {{< code >}}
   ```debian-ubuntu
   sudo apt-get install alloy
   ```

   ```rhel-fedora
   sudo dnf install alloy
   ```

   ```suse-opensuse
   sudo zypper install alloy
   ```
   {{< /code >}}

## Uninstall

To uninstall {{< param "PRODUCT_NAME" >}} on Linux, run the following commands in a terminal window.

1. Stop the systemd service for {{< param "PRODUCT_NAME" >}}.

   ```All-distros
   sudo systemctl stop alloy
   ```

1. Uninstall {{< param "PRODUCT_NAME" >}}.

   {{< code >}}
   ```debian-ubuntu
   sudo apt-get remove alloy
   ```

   ```rhel-fedora
   sudo dnf remove alloy
   ```

   ```suse-opensuse
   sudo zypper remove alloy
   ```
   {{< /code >}}

1. Optional: Remove the Grafana repository.

   {{< code >}}
   ```debian-ubuntu
   sudo rm -i /etc/apt/sources.list.d/grafana.list
   ```

   ```rhel-fedora
   sudo rm -i /etc/yum.repos.d/rpm.grafana.repo
   ```

   ```suse-opensuse
   sudo zypper removerepo grafana
   ```
   {{< /code >}}

## Next steps

- [Run {{< param "PRODUCT_NAME" >}}][Run]
- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Run]: ../../run/linux/
[Configure]: ../../../configure/linux/
