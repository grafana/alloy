---
canonical: https://grafana.com/docs/alloy/latest/access_permissions/
description: Set access and permissions for Grafana Alloy through process identity, network exposure, secrets, and component-level access
menuTitle: Access and permissions
title: Access and permissions for Grafana Alloy
weight: 110
---

# Access and permissions for {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} collects telemetry from hosts, containers, and APIs, then forwards it to observability backends.
That work requires read access to logs, process data, and cluster resources, plus credentials for remote write and similar endpoints.
Your configuration determines which permissions {{< param "PRODUCT_NAME" >}} needs, and your deployment platform determines how you enforce them.

## Access and permissions

1. Run {{< param "PRODUCT_NAME" >}} as a non-root user on [Linux][linux], [Kubernetes][kubernetes], or a dedicated service account on [Windows][windows].
1. Restrict the HTTP server to `127.0.0.1` or a private network address with the [`http` block][http-block].
1. Enable TLS for the HTTP server when you expose it beyond localhost with the [`http` block][http-block].
1. Use TLS for all outbound remote write and OTLP connections with the [`tls` block][tls-block].
1. Never set `insecure_skip_verify = true` in production.
   Refer to the [`tls` block][tls-block].
1. Store credentials outside configuration files.
   Refer to [Types and values][types-values].
1. Scope Kubernetes RBAC to the permissions your configuration uses.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. Set `readOnlyRootFilesystem: true` for container deployments.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. Set `allowPrivilegeEscalation: false` for container deployments.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. Don't use eBPF components unless your use case requires them.
   Refer to [Components that require elevated access](#components-that-require-elevated-access).

## Process identity and privilege

Create a dedicated service account or user on your deployment platform:

- [Access and permissions on Linux][linux]: systemd service, file permissions, and the `alloy` user
- [Access and permissions on Kubernetes][kubernetes]: `securityContext`, non-root UID, and RBAC
- [Access and permissions on Windows][windows]: service accounts, Windows security groups, and filesystem ACLs

## Network exposure

{{< param "PRODUCT_NAME" >}} runs an HTTP server for its UI, API, and `/metrics` endpoint.
The binary binds to `127.0.0.1:12345` by default, which limits exposure to the local machine.
The Grafana Helm chart sets `alloy.listenAddr` to `0.0.0.0` by default so other Pods can reach the container on port `12345`.
Review what you expose before you change either default.

## Secrets and credentials

You can load secrets at runtime through several patterns:

- `sys.env()` in configuration to reference environment variables
- [`remote.vault`][remote-vault] to load secrets from HashiCorp Vault
- Secrets from the cluster: [remote.kubernetes.secret][remote-k8s-secret]
- [`remote.s3`][remote-s3] to load configuration or secrets from AWS S3

For `secret`-typed values at runtime and protection from exposure in the UI and component exports, refer to [Types and values][types-values].

## Components that require elevated access

Some components need root, extra capabilities, or group membership.
Refer to [Components][components] for details.

[linux]: ./linux/
[kubernetes]: ./kubernetes/
[windows]: ./windows/
[http-block]: ../reference/config-blocks/http/
[tls-block]: ../shared/reference/components/tls-config-block/
[remote-vault]: ../reference/components/remote/remote.vault/
[remote-k8s-secret]: ../reference/components/remote/remote.kubernetes.secret/
[remote-s3]: ../reference/components/remote/remote.s3/
[types-values]: ../get-started/expressions/types_and_values/
[components]: ../reference/components/
