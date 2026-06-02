---
canonical: https://grafana.com/docs/alloy/latest/secure/
description: Secure Grafana Alloy for production through process identity, network exposure, secrets, and component-level access
menuTitle: Secure Alloy
title: Secure Grafana Alloy
weight: 110
---

# Secure {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} collects metrics, logs, traces, and profiles from your infrastructure.
A typical configuration reads host logs, accesses `/proc`, queries Kubernetes APIs, and stores credentials for observability backends.
Review these areas before you deploy to production.

## Process identity and privilege

Run {{< param "PRODUCT_NAME" >}} as a dedicated, unprivileged user wherever possible.
OS-level restrictions limit what a compromised process can access on the host.

- [Secure {{< param "PRODUCT_NAME" >}} on Linux][linux]: systemd service, file permissions, and the `alloy` user
- [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes]: `securityContext`, non-root UID, and OpenShift Security Context Constraints
- [Secure {{< param "PRODUCT_NAME" >}} on Windows][windows]: service accounts, Windows security groups, and filesystem ACLs

## Network exposure

{{< param "PRODUCT_NAME" >}} runs an HTTP server for its UI, API, and `/metrics` endpoint.
The binary binds to `127.0.0.1:12345` by default, which limits exposure to the local machine.
The Grafana Helm chart binds to `0.0.0.0:12345` by default so other Pods can reach the container.
Understand what you expose before you change either default.

- Configure TLS for the HTTP server in the [`http` block][http-block].
- Configure TLS for outbound connections in the [`tls` block][tls-block] in component references.
- Restrict or authenticate the HTTP server in the [`auth` block][http-block] inside the [`http` block][http-block].

## Secrets and credentials

Store credentials outside your configuration files.
{{< param "PRODUCT_NAME" >}} loads secrets at runtime through several patterns:

- `sys.env()` in configuration to reference environment variables
- [`remote.vault`][remote-vault] to load secrets from HashiCorp Vault
- Secrets from the cluster: [remote.kubernetes.secret][remote-k8s-secret]
- [`remote.s3`][remote-s3] to load configuration or secrets from AWS S3

For `secret`-typed values at runtime and protection from exposure in the UI and component exports, refer to [Types and values][types-values].

## Components that require elevated access

Some components need root, extra capabilities, or group membership.

| Component                  | Requirement                        | Notes                                                        |
| -------------------------- | ---------------------------------- | ------------------------------------------------------------ |
| `beyla.ebpf`               | Root or Linux capabilities         | Kernel-level eBPF. Incompatible with strict non-root setups. |
| `prometheus.exporter.unix` | Read access to `/proc`, `/sys`     | Usually satisfied by the `alloy` user on Linux.              |
| `loki.source.journal`      | `adm` and `systemd-journal` groups | The package installer adds the `alloy` user to both groups.  |
| `loki.source.file`         | Read access to target log paths    | Grant per-path ACLs instead of broad permissions.            |
| `pyroscope.ebpf`           | Root or Linux capabilities         | Same constraint as `beyla.ebpf`.                             |

## Pre-production checklist

Before you deploy {{< param "PRODUCT_NAME" >}} to production:

1. Run {{< param "PRODUCT_NAME" >}} as a non-root user on [Linux][linux], [Kubernetes][kubernetes], or [Windows][windows].
1. Restrict the HTTP server to `127.0.0.1` or a private network address with the [`http` block][http-block].
1. Enable TLS for the HTTP server when you expose it beyond localhost with the [`http` block][http-block].
1. Use TLS for all outbound remote write and OTLP connections with the [`tls` block][tls-block].
1. Never set `insecure_skip_verify = true` in production. Refer to the [`tls` block][tls-block].
1. Store credentials outside configuration files. Refer to [Types and values][types-values].
1. Scope Kubernetes RBAC to the permissions your configuration uses. Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes].
1. Set `readOnlyRootFilesystem: true` for container deployments. Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes].
1. Disable privilege escalation with `allowPrivilegeEscalation: false` for container deployments. Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes].
1. Avoid eBPF components unless your use case requires them. Refer to [Components that require elevated access](#components-that-require-elevated-access).

[linux]: ./linux/
[kubernetes]: ./kubernetes/
[windows]: ./windows/
[http-block]: ../reference/config-blocks/http/
[tls-block]: ../shared/reference/components/tls-config-block/
[remote-vault]: ../reference/components/remote/remote.vault/
[remote-k8s-secret]: ../reference/components/remote/remote.kubernetes.secret/
[remote-s3]: ../reference/components/remote/remote.s3/
[types-values]: ../get-started/expressions/types_and_values/
