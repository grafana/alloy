---
canonical: https://grafana.com/docs/alloy/latest/secure/linux/
description: Secure a Grafana Alloy installation on Linux using the alloy system user, file permissions, and systemd service security options
menuTitle: Linux
title: Secure Grafana Alloy on Linux
weight: 100
---

# Secure {{% param "FULL_PRODUCT_NAME" %}} on Linux

{{< param "PRODUCT_NAME" >}} requires read access to `/proc`, `/sys`, the systemd journal, application log files, and credentials for observability backends.
DEB and RPM packages for {{< param "PRODUCT_NAME" >}} provide a dedicated `alloy` user and systemd unit file.
You can configure filesystem permissions, systemd security directives, and read access for the components in your configuration.

{{< admonition type="note" >}}
If you installed from a binary instead of a package, create the `alloy` user and systemd unit yourself.
Refer to [Install {{< param "PRODUCT_NAME" >}} on Linux][install-linux] for setup steps, and adapt paths to match your layout.

[install-linux]: ../../set-up/install/linux/
{{< /admonition >}}

## Run as the `alloy` user

Verify the service runs as the `alloy` user:

```shell
ps aux | grep alloy
```

The output should show `alloy` in the user column, not `root`.

If the process runs as `root`, check the `User=` directive in the unit file:

```shell
systemctl cat alloy | grep User
```

If `User=alloy` isn't set, override it with a drop-in file instead of editing the unit directly:

```shell
sudo systemctl edit alloy
```

Add the following:

```ini
[Service]
User=alloy
Group=alloy
```

## Restrict file and directory permissions

The `alloy` user needs read access to the configuration file and read/write access to the data directory.
It shouldn't have access to anything else.

The package sets `/etc/alloy` and `/var/lib/alloy` to mode `770` at install time.
Tighter permissions for production:

| Path                      | Owner         | Permissions | Notes                                         |
| ------------------------- | ------------- | ----------- | --------------------------------------------- |
| `/etc/alloy/config.alloy` | `root:alloy`  | `640`       | Group-readable by `alloy`, not world-readable |
| `/etc/alloy/`             | `root:alloy`  | `750`       | `alloy` can read directory contents           |
| `/var/lib/alloy/`         | `alloy:alloy` | `750`       | Write-ahead log and data storage              |

Apply the permissions after installation:

```shell
sudo chown -R root:alloy /etc/alloy
sudo chmod 750 /etc/alloy
sudo chmod 640 /etc/alloy/config.alloy
sudo chown -R alloy:alloy /var/lib/alloy
sudo chmod 750 /var/lib/alloy
```

If the configuration file contains credentials, confirm it isn't world-readable:

```shell
stat /etc/alloy/config.alloy
```

## Secure the systemd service

The systemd unit in the package doesn't include security directives by default.
Add them with a drop-in file so they survive package upgrades:

```shell
sudo systemctl edit alloy
```

Add the following directives:

```ini
[Service]
# Prevent the process from gaining new privileges via setuid or capabilities
NoNewPrivileges=yes

# Make the entire filesystem read-only except for explicitly allowed paths
ProtectSystem=strict

# Prevent access to /home, /root, and /run/user
ProtectHome=yes

# Give the service a private /tmp, isolated from other services
PrivateTmp=yes

# Prevent the service from writing to kernel variables in /proc/sys and /sys
ProtectKernelTunables=yes

# Prevent loading kernel modules
ProtectKernelModules=yes

# Protect kernel log from the service
ProtectKernelLogs=yes

# Give the service its own network namespace if it only needs to reach specific hosts
# Remove this line if Alloy needs to listen on a host network interface
# PrivateNetwork=yes

# Allow write access to the data directory only
ReadWritePaths=/var/lib/alloy
```

Reload and restart the service:

```shell
sudo systemctl daemon-reload
sudo systemctl restart alloy
```

Make sure the service starts cleanly and review the logs for permission errors:

```shell
sudo journalctl -u alloy -n 50
```

## Grant access to the systemd journal

If you use [`loki.source.journal`][loki-source-journal], the `alloy` user needs membership in the `adm` and `systemd-journal` groups.
The package installer adds the user to both groups when they exist on the system.
If you installed via binary or removed the user from either group, add them back:

```shell
sudo usermod -aG adm,systemd-journal alloy
sudo systemctl restart alloy
```

When you use `ProtectSystem=strict`, add journal paths to `ReadOnlyPaths` in the systemd drop-in:

```ini
ReadOnlyPaths=/var/log/journal
ReadOnlyPaths=/run/log/journal
```

## Grant access to application log files

If you use [`loki.source.file`][loki-source-file] to read log files owned by other users or services, grant read access with ACLs instead of broadening the `alloy` user's group membership:

```shell
sudo setfacl -R -m u:alloy:r /var/log/myapp
sudo setfacl -R -d -m u:alloy:r /var/log/myapp
```

The `-d` flag sets a default ACL so new files in the directory inherit the permission.

## Restrict the HTTP server

By default, {{< param "PRODUCT_NAME" >}} binds its HTTP server to `127.0.0.1:12345`.
Only change the bind address when you need to expose the UI or metrics endpoint to other machines.

To expose `/metrics` for Prometheus scraping without exposing the UI, put a reverse proxy in front of {{< param "PRODUCT_NAME" >}} and restrict access at the proxy.
Refer to the [`http` block][http-block] for TLS and authentication options.

## Components that require elevated access

Some components can't run as the unprivileged `alloy` user.
Refer to [Components that require elevated access][elevated-access] for the full list.

**`beyla.ebpf`** and **`pyroscope.ebpf`** need root or additional Linux capabilities for kernel-level eBPF access.
Grant the required capabilities or run as root, and remove `NoNewPrivileges=yes` from the systemd drop-in when you grant capabilities to the `alloy` user.
Refer to the [beyla.ebpf component reference][beyla-ebpf].

**`prometheus.exporter.unix`** reads from `/proc` and `/sys`.
The `alloy` user can read most of these paths on a typical Linux system without elevated privileges.
If you see permission errors, check the metric collector that causes the issue instead of running as root.

## Next steps

- [Secure {{< param "PRODUCT_NAME" >}}][secure]
- [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes]
- [Secure {{< param "PRODUCT_NAME" >}} on Windows][windows]

[kubernetes]: ../kubernetes/
[windows]: ../windows/
[secure]: ../
[elevated-access]: ../#components-that-require-elevated-access
[http-block]: ../../reference/config-blocks/http/
[loki-source-journal]: ../../reference/components/loki/loki.source.journal/
[loki-source-file]: ../../reference/components/loki/loki.source.file/
[beyla-ebpf]: ../../reference/components/beyla/beyla.ebpf/
