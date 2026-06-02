---
canonical: https://grafana.com/docs/alloy/latest/secure/
description: Secure Grafana Alloy for production by controlling process identity, network exposure, secrets, and component-level access
menuTitle: Secure Alloy
title: Secure Grafana Alloy
weight: 110
---

# Secure {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} collects metrics, logs, traces, and profiles from your infrastructure.
Depending on how you configure it, {{< param "PRODUCT_NAME" >}} reads host logs, accesses `/proc`, queries Kubernetes APIs, and holds credentials for your observability backends.

## Security areas

### Process identity and privilege

Run {{< param "PRODUCT_NAME" >}} as a dedicated, unprivileged user wherever possible.
OS-level restrictions limit what a compromised process can access on the host.

- Refer to [Secure {{< param "PRODUCT_NAME" >}} on Linux][linux] for systemd service, file permissions, and the `alloy` user.
- Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes] for `securityContext`, non-root UID, and OpenShift Security Context Constraints.
- Refer to [Secure {{< param "PRODUCT_NAME" >}} on Windows][windows] for service accounts, Windows security groups, and filesystem ACLs.

### Network exposure

{{< param "PRODUCT_NAME" >}} runs an HTTP server for its UI, API, and `/metrics` endpoint.
The binary binds to `127.0.0.1:12345` by default, which limits exposure to the local machine.
The Grafana Helm chart binds to `0.0.0.0:12345` by default so other Pods can reach the container.
Before you change either default, understand what you expose and apply appropriate controls.

- Configure TLS for the HTTP server in the [`http` block][http-block].
- Configure TLS for outbound connections in the [`tls` block][tls-block] in component references.
- Restrict or authenticate the HTTP server in the [`auth` block][http-block] inside the [`http` block][http-block].

### Secrets and credentials

Store credentials outside your configuration files.
{{< param "PRODUCT_NAME" >}} supports several patterns for loading secrets at runtime without embedding them in configuration.

- Use `sys.env()` in configuration to reference environment variables.
- Use [`remote.vault`][remote-vault] to load secrets from HashiCorp Vault.
- Use [`remote.kubernetes.secret`][remote-k8s-secret] to load secrets from Kubernetes Secrets.
- Use [`remote.s3`][remote-s3] to load configuration or secrets from AWS S3.

For how {{< param "PRODUCT_NAME" >}} handles `secret`-typed values at runtime and protects them from exposure in the UI and component exports, refer to [Types and values][types-values].

### Components that require elevated access

Some components need root, extra capabilities, or group membership.

| Component                  | Requirement                        | Notes                                                       |
| -------------------------- | ---------------------------------- | ----------------------------------------------------------- |
| `beyla.ebpf`               | Root or `CAP_SYS_ADMIN`            | Kernel-level eBPF; incompatible with strict non-root setups |
| `prometheus.exporter.unix` | Read access to `/proc`, `/sys`     | Usually satisfied by the `alloy` user on Linux              |
| `loki.source.journal`      | `adm` and `systemd-journal` groups | The package installer adds the `alloy` user to both groups  |
| `loki.source.file`         | Read access to target log paths    | Grant per-path ACLs rather than broad permissions           |
| `pyroscope.ebpf`           | Root or `CAP_SYS_ADMIN`            | Same constraint as `beyla.ebpf`                             |

## Pre-production checklist

Complete these steps before you deploy {{< param "PRODUCT_NAME" >}} to production.

1. Run {{< param "PRODUCT_NAME" >}} as a non-root user.
   Refer to [Linux][linux], [Kubernetes][kubernetes], or [Windows][windows] guidance.
1. Restrict the HTTP server to `127.0.0.1` or a private network address.
   Refer to the [`http` block][http-block].
1. Enable TLS for the HTTP server if you expose it beyond localhost.
   Refer to the [`http` block][http-block].
1. Use TLS for all outbound remote write and OTLP connections.
   Refer to the [`tls` block][tls-block].
1. Never set `insecure_skip_verify = true` in production.
   Refer to the [`tls` block][tls-block].
1. Store credentials outside configuration files.
   Refer to [Types and values][types-values].
1. Scope Kubernetes RBAC to the permissions your configuration uses.
   Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes].
1. Set `readOnlyRootFilesystem: true` for container deployments.
   Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes].
1. Disable privilege escalation with `allowPrivilegeEscalation: false` for container deployments.
   Refer to [Secure {{< param "PRODUCT_NAME" >}} on Kubernetes][kubernetes].
1. Avoid eBPF components unless your use case requires them.
   Refer to [Components that require elevated access](#components-that-require-elevated-access).

[linux]: ./linux/
[kubernetes]: ./kubernetes/
[windows]: ./windows/
[http-block]: ../reference/config-blocks/http/
[tls-block]: ../shared/reference/components/tls-config-block/
[remote-vault]: ../reference/components/remote/remote.vault/
[remote-k8s-secret]: ../reference/components/remote/remote.kubernetes.secret/
[remote-s3]: ../reference/components/remote/remote.s3/
[types-values]: ../get-started/expressions/types_and_values/
