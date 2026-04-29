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

{{< param "PRODUCT_NAME" >}} reports anonymous usage statistics by default.
To opt out, use the [CLI flag][command line flag] `--disable-reporting`.

## The statistics server

{{< param "PRODUCT_NAME" >}} sends usage statistics to a server that Grafana Labs runs.
It sends data to `https://stats.grafana.org/alloy-usage-report` with an HTTP POST request.
This endpoint only accepts data and isn't available to view in a browser.

## What {{% param "PRODUCT_NAME" %}} collects

{{< param "PRODUCT_NAME" >}} collects the following information:

- A randomly generated anonymous UUID.
- The timestamp when {{< param "PRODUCT_NAME" >}} first created the UUID.
- The scheduled report interval, which defaults to four hours.
- The version of {{< param "PRODUCT_NAME" >}}.
- The operating system where {{< param "PRODUCT_NAME" >}} runs.
- The system architecture where {{< param "PRODUCT_NAME" >}} runs.
- A list of enabled [components][].
- The deployment method, such as `docker`, `helm`, `operator`, `deb`, `rpm`, `brew`, or `binary`.

{{< admonition type="note" >}}
{{< param "PRODUCT_NAME" >}} maintainers update this list of tracked information over time and report any changes in the CHANGELOG.
{{< /admonition >}}

## Disable anonymous usage statistics

If possible, keep this feature enabled.
It helps Grafana Labs understand how the open source community uses {{< param "PRODUCT_NAME" >}}.
To opt out of anonymous usage statistics, use the [CLI flag][command line flag] `--disable-reporting`.

### Example: Opt out of data collection with Ansible

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

### Example: Opt out of data collection on Linux

1. Edit the environment file for the service:

   - Debian-based systems: edit `/etc/default/alloy`
   - RedHat or SUSE-based systems: edit `/etc/sysconfig/alloy`

1. Add `--disable-reporting` to the `CUSTOM_ARGS` environment variable.
1. Restart the Alloy service:

   ```shell
   sudo systemctl restart alloy
   ```

[components]: ../get-started/components/
[command line flag]: ../reference/cli/run/
