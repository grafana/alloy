---
canonical: https://grafana.com/docs/alloy/latest/secure/harden-linux/
description: Harden a standalone Grafana Alloy installation on Linux using the alloy system user, file permissions, and systemd service hardening options
menuTitle: Harden on Linux
title: Harden Grafana Alloy on Linux
weight: 100
---

# Harden {{% param "FULL_PRODUCT_NAME" %}} on Linux

This page covers hardening measures for {{< param "PRODUCT_NAME" >}} installed as a system service on Linux using the deb or rpm package.
If you run {{< param "PRODUCT_NAME" >}} on Kubernetes, refer to [Harden {{< param "PRODUCT_NAME" >}} on Kubernetes][harden-kubernetes] instead.

{{< admonition type="note" >}}
These steps assume you installed {{< param "PRODUCT_NAME" >}} using the official deb or rpm package.
The package creates the `alloy` system user and the systemd unit file automatically.
If you installed via binary, adapt the steps to match your installation paths.
{{< /admonition >}}

## Run as the alloy user

The {{< param "PRODUCT_NAME" >}} package creates a dedicated system user named `alloy` at install time.
The systemd unit runs the process as this user by default.

To verify the service runs as the `alloy` user:

```shell
ps aux | grep alloy
```

The output should show `alloy` in the user column, not `root`.

If the process runs as `root`, check the `User=` directive in the unit file:

```shell
systemctl cat alloy | grep User
```

If `User=alloy` isn't set, override it using a drop-in file rather than editing the unit directly:

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
The following table shows tighter permissions you can apply for production hardening.

| Path                      | Owner         | Permissions | Notes                                         |
| ------------------------- | ------------- | ----------- | --------------------------------------------- |
| `/etc/alloy/config.alloy` | `root:alloy`  | `640`       | Group-readable by `alloy`, not world-readable |
| `/etc/alloy/`             | `root:alloy`  | `750`       | `alloy` can read directory contents           |
| `/var/lib/alloy/`         | `alloy:alloy` | `750`       | Write-ahead log and data storage              |

Apply these permissions after installation:

```shell
sudo chown -R root:alloy /etc/alloy
sudo chmod 750 /etc/alloy
sudo chmod 640 /etc/alloy/config.alloy
sudo chown -R alloy:alloy /var/lib/alloy
sudo chmod 750 /var/lib/alloy
```

If your configuration file contains credentials, confirm it isn't world-readable:

```shell
stat /etc/alloy/config.alloy
```

## Harden the systemd service

The systemd unit shipped with the {{< param "PRODUCT_NAME" >}} package doesn't include security hardening directives by default.
You can add them using a drop-in file without modifying the upstream unit, so they survive package upgrades.

Create a drop-in file:

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

Reload and restart the service to apply the changes:

```shell
sudo systemctl daemon-reload
sudo systemctl restart alloy
```

Verify the service starts cleanly and check for permission errors in the logs:

```shell
sudo journalctl -u alloy -n 50
```

{{< admonition type="note" >}}
If you use `loki.source.journal` to collect systemd journal logs, add `/run/log/journal` and `/var/log/journal` to `ReadOnlyPaths` so the `alloy` user can read them.
The `alloy` user also needs membership in the `adm` and `systemd-journal` groups.
Refer to [Grant access to the systemd journal](#grant-access-to-the-systemd-journal).
{{< /admonition >}}

## Grant access to the systemd journal

If you use `loki.source.journal`, the `alloy` user needs membership in the `adm` and `systemd-journal` groups.
The package installer adds the `alloy` user to both groups when they exist on the system.
If you installed via binary or removed the user from either group, add them back:

```shell
sudo usermod -aG adm,systemd-journal alloy
sudo systemctl restart alloy
```

## Grant access to application log files

If you use `loki.source.file` to tail log files owned by other users or services, grant read access using ACLs rather than broadening the `alloy` user's group membership:

```shell
sudo setfacl -R -m u:alloy:r /var/log/myapp
sudo setfacl -R -d -m u:alloy:r /var/log/myapp
```

The `-d` flag sets a default ACL so files created in the directory inherit the permission.

## Restrict the HTTP server

By default, {{< param "PRODUCT_NAME" >}} binds its HTTP server to `127.0.0.1:12345`, which is only reachable from the local machine.
Don't change this unless you have a specific operational need to expose the UI or metrics endpoint to other machines.

If you need to expose the `/metrics` endpoint for Prometheus scraping without exposing the UI, place a reverse proxy in front of {{< param "PRODUCT_NAME" >}} and restrict access at the proxy level.

For configuration options, refer to the [`http` block][http-block].

## Components that require elevated access

Some components can't run as the unprivileged `alloy` user.

**`beyla.ebpf`** and **`pyroscope.ebpf`** require root or `CAP_SYS_ADMIN` for kernel-level eBPF access.
If you need eBPF-based instrumentation, grant the necessary capability or run as root.
This is incompatible with `NoNewPrivileges=yes`.
Evaluate whether the operational benefit justifies relaxing the hardening.

**`prometheus.exporter.unix`** reads from `/proc` and `/sys`.
The `alloy` user can read most of these paths without elevated privileges on a typical Linux system.
If you see permission errors, check the specific metric collector that causes the issue rather than running as root.

## Next steps

- [Secure {{< param "PRODUCT_NAME" >}}][secure]: overview of all security areas
- [Harden {{< param "PRODUCT_NAME" >}} on Kubernetes][harden-kubernetes]
- [Harden {{< param "PRODUCT_NAME" >}} on Windows][harden-windows]
- [`http` block][http-block]: TLS and authentication for the HTTP server

[harden-kubernetes]: ../harden-kubernetes/
[harden-windows]: ../harden-windows/
[secure]: ../
[http-block]: ../../reference/config-blocks/http/
