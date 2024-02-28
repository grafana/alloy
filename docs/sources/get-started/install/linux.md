---
aliases:
- /docs/grafana-cloud/agent/flow/get-started/install/linux/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/install/linux/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/install/linux/
- /docs/grafana-cloud/send-data/agent/flow/get-started/install/linux/
# Previous docs aliases for backwards compatibility:
- ../../install/linux/ # /docs/agent/latest/flow/install/linux/
- /docs/grafana-cloud/agent/flow/setup/install/linux/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/install/linux/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/install/linux/
- /docs/grafana-cloud/send-data/agent/flow/setup/install/linux/
- ../../setup/install/linux/ # /docs/agent/latest/flow/setup/install/linux/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/install/linux/
description: Learn how to install Grafana Agent Flow on Linux
menuTitle: Linux
title: Install Grafana Agent Flow on Linux
weight: 300
---

# Install or uninstall {{% param "PRODUCT_NAME" %}} on Linux

You can install {{< param "PRODUCT_NAME" >}} as a systemd service on Linux.

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
   sudo apt-get install grafana-agent-flow
   ```

   ```rhel-fedora
   sudo dnf install grafana-agent-flow
   ```

   ```suse-opensuse
   sudo zypper install grafana-agent-flow
   ```
   {{< /code >}}

## Uninstall

To uninstall {{< param "PRODUCT_NAME" >}} on Linux, run the following commands in a terminal window.

1. Stop the systemd service for {{< param "PRODUCT_NAME" >}}.

   ```All-distros
   sudo systemctl stop grafana-agent-flow
   ```

1. Uninstall {{< param "PRODUCT_NAME" >}}.

   {{< code >}}
   ```debian-ubuntu
   sudo apt-get remove grafana-agent-flow
   ```

   ```rhel-fedora
   sudo dnf remove grafana-agent-flow
   ```

   ```suse-opensuse
   sudo zypper remove grafana-agent-flow
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

{{% docs/reference %}}
[Run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/run/linux.md"
[Run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/get-started/run/linux.md"
[Configure]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure/configure-linux.md"
[Configure]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-linux.md"
{{% /docs/reference %}}
