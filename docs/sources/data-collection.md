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
To opt out of anonymous usage statistics, use the [CLI flag][command line flag] `--disable-reporting` with the method that matches your install type.

### Linux

1. Edit the environment file for the service:
   - Debian-based systems: edit `/etc/default/alloy`
   - RedHat or SUSE-based systems: edit `/etc/sysconfig/alloy`

1. Add `--disable-reporting` to the `CUSTOM_ARGS` environment variable.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   sudo systemctl restart alloy
   ```

### Windows

{{< param "PRODUCT_NAME" >}} runs as a Windows service and reads its command-line arguments from the registry.

1. Open the Registry Editor:
   1. Right-click the **Start** menu and select **Run**.
   1. Type `regedit` and click **OK**.

1. Navigate to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.

1. Double-click the **Arguments** value.

1. Add `--disable-reporting` on a separate line at the end of the arguments list.

1. Click **OK**.

1. Restart the {{< param "PRODUCT_NAME" >}} service:
   1. Right-click the **Start** menu and select **Run**.
   1. Type `services.msc` and click **OK**.
   1. Right-click the **{{< param "PRODUCT_NAME" >}}** service and select **All Tasks > Restart**.

### macOS

{{< param "PRODUCT_NAME" >}} reads extra command-line flags from a dedicated file when you install it with Homebrew.

1. Edit `$(brew --prefix)/etc/alloy/extra-args.txt`.

1. Add `--disable-reporting` on a separate line.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   brew services restart grafana/grafana/alloy
   ```

### Docker

Add `--disable-reporting` to the `alloy run` arguments in your `docker run` command:

```shell
docker run \
  -v <CONFIG_FILE_PATH>:/etc/alloy/config.alloy \
  -p 12345:12345 \
  grafana/alloy:latest \
    run --server.http.listen-addr=0.0.0.0:12345 \
    --storage.path=/var/lib/alloy/data \
    --disable-reporting \
    /etc/alloy/config.alloy
```

Replace the following:

- _`<CONFIG_FILE_PATH>`_: The path to your configuration file on the host system.

### Helm

The Grafana Alloy Helm chart includes a dedicated value to disable usage statistics. Set `alloy.enableReporting` to `false` in your `values.yaml`:

```yaml
alloy:
  enableReporting: false
```

Then apply the change:

```shell
helm upgrade --namespace <NAMESPACE> <RELEASE_NAME> grafana/alloy -f <VALUES_PATH>
```

Replace the following:

- _`<NAMESPACE>`_: The namespace for your {{< param "PRODUCT_NAME" >}} installation.
- _`<RELEASE_NAME>`_: The name of your {{< param "PRODUCT_NAME" >}} Helm release.
- _`<VALUES_PATH>`_: The path to your `values.yaml` file.

### Ansible

Set `CUSTOM_ARGS` in your playbook using the Grafana Ansible collection:

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

### Binary

Add `--disable-reporting` to the `alloy run` command:

```shell
<BINARY_PATH> run --disable-reporting <CONFIG_PATH>
```

Replace the following:

- _`<BINARY_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary.
- _`<CONFIG_PATH>`_: The path to your {{< param "PRODUCT_NAME" >}} configuration file.

[components]: ../get-started/components/
[command line flag]: ../reference/cli/run/
