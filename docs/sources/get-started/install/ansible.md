---
canonical: https://grafana.com/docs/alloy/latest/get-started/install/ansible/
description: Learn how to install Grafana Alloy with Ansible
menuTitle: Ansible
title: Install Grafana Alloy with Ansible
weight: 550
---

# Install or uninstall {{% param "PRODUCT_NAME" %}} using Ansible

You can use Ansible to install and manage {{< param "PRODUCT_NAME" >}} on Linux hosts.

## Before you begin

- These steps assume you already have a working [Ansible][] setup and a pre-existing inventory.
- You can add the tasks below to any new or existing role.

## Steps

To add {{% param "PRODUCT_NAME" %}} to a host:

1. Create a file named `alloy.yml` and add the following:

    ```yaml
    - name: Install Grafana Alloy
      hosts: all
      become: true
      tasks:
        - name: Install Grafana Alloy
          ansible.builtin.include_role:
            name: grafana.grafana.alloy
          vars:
            # Destination file name
            grafana_alloy_config_filename: config.alloy
            # Local file to copy
            grafana_alloy_provisioned_config_file:  "<path-to-config-file-on-localhost>"
            grafana_alloy_flags_extra:
              server.http.listen-addr: '0.0.0.0:12345'
    ```

   Replace the following:
   - _`<path-to-config-file-on-localhost>`_: The path to the River configuration file on the Ansible Controller (Localhost).

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

[Ansible]: https://www.ansible.com/
[Configure]: ../../../tasks/configure/configure-linux/
