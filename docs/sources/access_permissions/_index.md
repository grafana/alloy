---
canonical: https://grafana.com/docs/alloy/latest/access_permissions/
description: Set access and permissions for Grafana Alloy through process identity, network exposure, secrets, and component-level access
menuTitle: Access and permissions
title: Access and permissions for Grafana Alloy
weight: 95
---

# Access and permissions for {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} collects telemetry from hosts, containers, and APIs, then forwards it to observability backends.
That work requires read access to logs, process data, and cluster resources, plus credentials for remote write and similar endpoints.
Your configuration determines which permissions {{< param "PRODUCT_NAME" >}} needs, and your deployment platform determines how you enforce them.

The following settings are common permission options.
Not every item applies to every configuration or platform.
Use only what matches your components and environment.

1. When your components allow it, run {{< param "PRODUCT_NAME" >}} as a non-root user on [Linux][linux], [Kubernetes][kubernetes], or a dedicated service account on [Windows][windows].
1. If you don't need remote access to the UI or `/metrics`, restrict the HTTP server to `127.0.0.1` or a private network address with the [`http` block][http-block].
1. When you expose the HTTP server beyond localhost, enable TLS with the [`http` block][http-block].
1. Use TLS for outbound connections.
   Refer to the component you're configuring, for example [`prometheus.remote_write`][prometheus-remote-write] for remote write and [`otelcol.exporter.otlp`][otelcol-exporter-otlp] for OTLP.
1. Avoid `insecure_skip_verify = true` in production.
   Refer to the TLS settings in that component's reference, for example [`prometheus.remote_write`][prometheus-remote-write].
1. Store credentials outside configuration files when you can.
   Refer to [Types and values][types-values].
1. On Kubernetes, set RBAC to the permissions your configuration uses.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. For container deployments, set `readOnlyRootFilesystem: true` when your volume mounts and components allow it.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. For container deployments, set `allowPrivilegeEscalation: false` when your components don't need privilege escalation.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. Use a dedicated {{< param "PRODUCT_NAME" >}} instance for components that require elevated access.
   Refer to [Components that require elevated access](#components-that-require-elevated-access).

## Process identity and privilege

Create a dedicated service account or user on your deployment platform:

- [Linux][linux]: systemd service, file permissions, and the `alloy` user
- [Kubernetes][kubernetes]: `securityContext`, non-root UID, and RBAC
- [Windows][windows]: service accounts, Windows security groups, and filesystem ACLs

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
[prometheus-remote-write]: ../reference/components/prometheus/prometheus.remote_write/
[otelcol-exporter-otlp]: ../reference/components/otelcol/otelcol.exporter.otlp/
[remote-vault]: ../reference/components/remote/remote.vault/
[remote-k8s-secret]: ../reference/components/remote/remote.kubernetes.secret/
[remote-s3]: ../reference/components/remote/remote.s3/
[types-values]: ../get-started/expressions/types_and_values/
[components]: ../reference/components/
