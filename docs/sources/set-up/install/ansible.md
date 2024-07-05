---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/ansible/
aliases:
  - ../../get-started/install/ansible/ # /docs/alloy/latest/get-started/install/ansible/
description: Learn how to install Grafana Alloy with Ansible
menuTitle: Ansible
title: Install Grafana Alloy with Ansible
weight: 550
_build:
  list: false
noindex: true
---

# Install or uninstall {{% param "FULL_PRODUCT_NAME" %}} using Ansible

You can use [Grafana Ansible Collection](https://github.com/grafana/grafana-ansible-collection) to install and manage {{< param "PRODUCT_NAME" >}} on Linux hosts.

## Before you begin

- These steps assume you already have a working [Ansible][] setup and a pre-existing inventory.
- You can add the tasks below to any new or existing role.

## Steps

To add {{% param "PRODUCT_NAME" %}} to a host:

1. Create a file named `alloy.yml` and add the following:

    ```yaml
    - name: Install Alloy
      hosts: all
      become: true

      tasks:
        - name: Install Alloy
          ansible.builtin.include_role:
            name: grafana.grafana.alloy
          vars:
            config: |
              prometheus.scrape "default" {
                targets = [{"__address__" = "localhost:12345"}]
                forward_to = [prometheus.remote_write.prom.receiver]
              }
              prometheus.remote_write "prom" {
                endpoint {
                    url = "YOUR_PROMETHEUS_PUSH_ENDPOINT"
                }
              }
    ```

    The above snippet has a sample configuration to collect and send Alloy metrics to Prometheus

    Replace the following:
    - _`YOUR_PROMETHEUS_PUSH_ENDPOINT`_:  With the Remote write endpoint of your Prometheus Instance.

1. Run the Ansible playbook. Open a terminal window and run the following command from the Ansible playbook directory.

   ```shell
   ansible-playbook alloy.yml
   ```

## Validate

To verify that the {{< param "PRODUCT_NAME" >}} service on the target machine is `active` and `running`, open a terminal window and run the following command:

```shell
$ sudo systemctl status alloy.service
```

If the service is `active` and `running`, the output should look similar to this:

```
alloy.service - Grafana Alloy
  Loaded: loaded (/etc/systemd/system/alloy.service; enabled; vendor preset: enabled)
  Active: active (running) since Wed 2022-07-20 09:56:15 UTC; 36s ago
Main PID: 3176 (alloy-linux-amd)
  Tasks: 8 (limit: 515)
  Memory: 92.5M
    CPU: 380ms
  CGroup: /system.slice/alloy.service
    └─3176 /usr/local/bin/alloy-linux-amd64 --config.file=/etc/grafana-cloud/alloy-config.yaml
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Grafana Ansible Collection]: https://github.com/grafana/grafana-ansible-collection
[Configure]: ../../../configure/linux/
