---
canonical: https://grafana.com/docs/alloy/latest/access_permissions/
description: Set access and permissions for Grafana Alloy through process identity, network exposure, secrets, and component-level access
menuTitle: Access and permissions
title: Access and permissions for Grafana Alloy
weight: 95
---

# Access and permissions for {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} collects telemetry from hosts, containers, and APIs, then forwards it to observability backends.
That telemetry collection requires read access to logs, process data, and cluster resources, plus credentials for remote write and similar endpoints.
Your configuration determines which permissions {{< param "PRODUCT_NAME" >}} needs, and your deployment platform determines how you enforce them.

The following settings are common permission options.
Not every item applies to every configuration or platform.
Use only what matches your components and environment.

1. When your components allow it, run {{< param "PRODUCT_NAME" >}} as a non-root user on [Linux][linux], [Kubernetes][kubernetes], or a dedicated service account on [Windows][windows].
1. Restrict network exposure: bind the HTTP server and OpenTelemetry receivers to localhost when remote access isn't required, and use TLS and authentication when you expose listeners or connect to remote backends.
   Refer to [Network exposure](#network-exposure).
1. Avoid `insecure_skip_verify = true` in production.
   Refer to the TLS settings in the [component][components] reference, for example [`prometheus.remote_write`][prometheus-remote-write].
1. Store credentials outside configuration files when you can.
   Refer to [Types and values][types-values].
1. On Kubernetes, set RBAC to the permissions your configuration uses.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. For container deployments on Kubernetes, set `readOnlyRootFilesystem: true` and `allowPrivilegeEscalation: false` when your volume mounts and components allow it.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. For container deployments, set `allowPrivilegeEscalation: false` when your components don't need privilege escalation.
   Refer to [Access and permissions on Kubernetes][kubernetes].
1. Use a dedicated {{< param "PRODUCT_NAME" >}} instance for components that require elevated access, for example [`beyla.ebpf`][beyla-ebpf] and [`pyroscope.ebpf`][pyroscope-ebpf].
   Refer to each component reference for required capabilities and privileges.

## Process identity and privilege

Create a dedicated service account or user on your deployment platform:

- [Linux][linux]: systemd service, file permissions, and the `alloy` user
- [Kubernetes][kubernetes]: `securityContext`, non-root UID, and RBAC
- [Windows][windows]: service accounts, Windows security groups, and filesystem ACLs

## Network exposure

{{< param "PRODUCT_NAME" >}} exposes network listeners through its HTTP server and through OpenTelemetry receivers.
Review bind addresses, TLS, and authentication before you expose an endpoint beyond localhost.

### HTTP server

{{< param "PRODUCT_NAME" >}} runs an HTTP server for its UI, API, and `/metrics` endpoint.
The binary binds to `127.0.0.1:12345` by default, which limits exposure to the local machine.
The Grafana Helm chart sets `alloy.listenAddr` to `0.0.0.0` by default so other Pods can reach the container on port `12345`.
Review what you expose before you change either default.
When you must expose the HTTP server beyond localhost, enable TLS with the [`http` block][http-block].

### OpenTelemetry receivers

OpenTelemetry receivers such as [`otelcol.receiver.otlp`][otelcol-receiver-otlp] accept incoming telemetry on network interfaces.
`otelcol.receiver.otlp` binds to `0.0.0.0` by default on ports `4317` and `4318`.
Many other HTTP-based receivers default to `localhost`.
Confirm the `endpoint` default in the component reference before you deploy.

If clients on the same host can reach the receiver, set `endpoint` to `127.0.0.1` or `localhost` instead of `0.0.0.0`.
An unauthenticated receiver that's reachable from other hosts lets anyone push metrics, logs, and traces into your pipeline.

Because {{< param "PRODUCT_NAME" >}} implements these endpoints with OpenTelemetry Collector components, the same binding and authentication recommendations apply.
For the full upstream guidance, refer to [OpenTelemetry Collector configuration best practices][otel-security-best-practices].

### TLS and authentication

When you must expose a listener beyond localhost, enable TLS on the receiver's `http`, `grpc`, or equivalent block, or on an exporter's `client` block.
Set the `auth` attribute on that block and reference an `otelcol.auth.*` component's `handler` export for credentials.
Not every auth component supports server-side authentication.
Refer to the [otelcol component reference][otelcol] for compatible options and component-specific TLS settings.

Don't set `insecure_skip_verify = true` in production on outbound connections.
Refer to the TLS settings in the [component reference][components] for the component you configure, such as [`prometheus.remote_write`][prometheus-remote-write] or [`otelcol.exporter.otlp`][otelcol-exporter-otlp].

## Secrets and credentials

Store authentication tokens, passwords, and similar values outside plain configuration when you can.
The following patterns load secrets at runtime.
Pair them with the `secret` type described in [Types and values][types-values] so credentials don't appear in the UI or component exports.

- [`sys.env()`][sys-env] loads environment variables in configuration.
- [`remote.vault`][remote-vault] loads secrets from HashiCorp Vault.
- [`remote.kubernetes.secret`][remote-k8s-secret] loads secrets from the cluster.
- [`remote.s3`][remote-s3] loads configuration or secrets from AWS S3.

## Next steps

- [Access and permissions on Linux][linux]
- [Access and permissions on Kubernetes][kubernetes]
- [Access and permissions on Windows][windows]
- [OpenTelemetry Collector configuration best practices][otel-security-best-practices]

[linux]: ./linux/
[kubernetes]: ./kubernetes/
[windows]: ./windows/
[network-exposure]: #network-exposure
[http-block]: ../reference/config-blocks/http/
[prometheus-remote-write]: ../reference/components/prometheus/prometheus.remote_write/
[otelcol-exporter-otlp]: ../reference/components/otelcol/otelcol.exporter.otlp/
[otelcol-receiver-otlp]: ../reference/components/otelcol/otelcol.receiver.otlp/
[otelcol]: ../reference/components/otelcol/
[otel-security-best-practices]: https://opentelemetry.io/docs/security/config-best-practices/
[remote-vault]: ../reference/components/remote/remote.vault/
[sys-env]: ../reference/stdlib/sys/#sys.env
[remote-k8s-secret]: ../reference/components/remote/remote.kubernetes.secret/
[remote-s3]: ../reference/components/remote/remote.s3/
[types-values]: ../get-started/expressions/types_and_values/
[components]: ../reference/components/
[beyla-ebpf]: ../reference/components/beyla/beyla.ebpf/
[pyroscope-ebpf]: ../reference/components/pyroscope/pyroscope.ebpf/
