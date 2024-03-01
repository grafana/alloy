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

1. Create a file named `grafana-agent.yml` and add the following:

    ```yaml
    - name: Install Grafana Agent Flow
      hosts: all
      become: true
      tasks:
        - name: Install Grafana Agent Flow
          ansible.builtin.include_role:
            name: grafana.grafana.grafana_agent
          vars:
            grafana_agent_mode: flow
            # Destination file name
            grafana_agent_config_filename: config.river
            # Local file to copy
            grafana_agent_provisioned_config_file:  "<path-to-config-file-on-localhost>"
            grafana_agent_flags_extra:
              server.http.listen-addr: '0.0.0.0:12345'
    ```

   Replace the following:
   - _`<path-to-config-file-on-localhost>`_: The path to the River configuration file on the Ansible Controller (Localhost).

1. Run the Ansible playbook. Open a terminal window and run the following command from the Ansible playbook directory.

   ```shell
   ansible-playbook grafana-agent.yml
   ```

## Validate

To verify that the {{< param "PRODUCT_NAME" >}} service on the target machine is `active` and `running`, open a terminal window and run the following command:

```shell
$ sudo systemctl status grafana-agent.service
```

If the service is `active` and `running`, the output should look similar to this:

```
grafana-agent.service - Grafana Agent
  Loaded: loaded (/etc/systemd/system/grafana-agent.service; enabled; vendor preset: enabled)
  Active: active (running) since Wed 2022-07-20 09:56:15 UTC; 36s ago
Main PID: 3176 (agent-linux-amd)
  Tasks: 8 (limit: 515)
  Memory: 92.5M
    CPU: 380ms
  CGroup: /system.slice/grafana-agent.service
    └─3176 /usr/local/bin/agent-linux-amd64 --config.file=/etc/grafana-cloud/agent-config.yaml
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Ansible]: https://www.ansible.com/
[Configure]: ../../../tasks/configure/configure-linux/
