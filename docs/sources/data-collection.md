---
canonical: https://grafana.com/docs/alloy/latest/data-collection/
description: Grafana Alloy data collection
menuTitle: Data collection
title: Grafana Alloy data collection
weight: 900
---

# {{% param "FULL_PRODUCT_NAME" %}} data collection

{{< param "FULL_PRODUCT_NAME" >}} includes a system that optionally and anonymously reports non-sensitive, non-personally identifiable information about {{< param "PRODUCT_NAME" >}} to a remote statistics server.
{{< param "PRODUCT_NAME" >}} maintainers use this anonymous information to learn how the open source community runs {{< param "PRODUCT_NAME" >}}.
This data helps the {{< param "PRODUCT_NAME" >}} team prioritize features and improve documentation.

The anonymous usage statistics reporting is **enabled by default**.
You can opt out by setting the [CLI flag][command line flag] `--disable-reporting` to `true`.

## The statistics server

When usage statistics reporting is enabled, a server that Grafana Labs runs collects the information.
The statistics are collected at `https://stats.grafana.org`.

## Which information is collected

When usage statistics reporting is enabled, {{< param "PRODUCT_NAME" >}} collects the following information:

* A randomly generated, anonymous, unique ID (UUID).
* The timestamp when the UUID was first generated.
* The timestamp when the report was created (by default, every four hours).
* The version of {{< param "PRODUCT_NAME" >}}.
* The operating system where {{< param "PRODUCT_NAME" >}} is running.
* The system architecture where {{< param "PRODUCT_NAME" >}} is running.
* A list of enabled [components][].
* The deployment method for {{< param "PRODUCT_NAME" >}}, such as Docker, Helm, or a Linux package.

{{< admonition type="note" >}}
{{< param "PRODUCT_NAME" >}} maintainers commit to keeping the list of tracked information updated over time.
Any changes are reported in the CHANGELOG.
{{< /admonition >}}

## Disable the anonymous usage statistics reporting

If possible, we ask you to keep the usage reporting feature enabled to help us understand how the open source community runs {{< param "PRODUCT_NAME" >}}.
If you want to opt out of anonymous usage statistics reporting, set the [CLI flag][command line flag] `--disable-reporting` to `true`.

### Example: Opt-out of data collection with Ansible

```yaml
- name: Install Alloy
  hosts: all
  become: true
  tasks:
    - name: Install Alloy
      ansible.builtin.include_role:
        name: grafana.grafana.alloy
      vars:
        alloy_env_file_vars:
          CUSTOM_ARGS: "--disable-reporting"
```

### Example: Opt-out of data collection on Linux

1. Edit the environment file for the service:

   * Debian-based systems: edit `/etc/default/alloy`
   * RedHat or SUSE-based systems: edit `/etc/sysconfig/alloy`

1. Add `--disable-reporting` to the `CUSTOM_ARGS` environment variable.
1. Restart the Alloy service:

   ```shell
   sudo systemctl restart alloy
   ```

[components]: ../get-started/components/
[command line flag]: ../reference/cli/run/
