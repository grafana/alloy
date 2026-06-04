---
canonical: https://grafana.com/docs/alloy/latest/access_permissions/kubernetes/
aliases:
  - ../configure/nonroot/ # /docs/alloy/latest/configure/nonroot/
  - ../../configure/nonroot/ # /docs/alloy/latest/configure/nonroot/
  - ../tasks/nonroot/ # /docs/alloy/latest/tasks/nonroot/
description: Set access and permissions for Grafana Alloy on Kubernetes with securityContext, non-root users, and capability drops
menuTitle: Kubernetes
title: Access and permissions for Grafana Alloy on Kubernetes
weight: 200
---

# Access and permissions for {{% param "FULL_PRODUCT_NAME" %}} on Kubernetes

{{< param "PRODUCT_NAME" >}} requires read access to Kubernetes API resources, node and container telemetry, and credentials for observability backends.
The {{< param "PRODUCT_NAME" >}} container image runs as `root` by default and defines a non-root `alloy` user with UID `473` and GID `473`.
Set `securityContext`, RBAC, and network settings to match the components in your configuration.

The [{{< param "PRODUCT_NAME" >}} Docker container image][image] defines two users:

- A `root` user.
- A non-root user named `alloy` with UID `473` and GID `473`.

By default, the `alloy` binary runs as `root` because some components, such as [beyla.ebpf][], need elevated privileges.

You can configure a non-root user when you deploy Alloy in Kubernetes.

{{< admonition type="note" >}}
Components like [beyla.ebpf][beyla-ebpf-note] need root or additional Linux capabilities.
Don't set `capabilities.drop: [ALL]` when these components are in your configuration.
Refer to the [beyla.ebpf component reference][beyla-ebpf-note].

[beyla-ebpf-note]: ../../reference/components/beyla/beyla.ebpf/
{{< /admonition >}}

## Run as a non-root user

To run {{< param "PRODUCT_NAME" >}} as a non-root user, configure a [security context][security-context] for the {{< param "PRODUCT_NAME" >}} container.
If you use the [Grafana Helm chart][], add this to `values.yaml`:

```yaml
alloy:
  securityContext:
    runAsUser: 473
    runAsGroup: 473

global:
  podSecurityContext:
    fsGroup: 473

configReloader:
  securityContext:
    # this is the UID of the "nobody" user that the configReloader image runs as
    runAsUser: 65534
    runAsGroup: 65534
```

This configuration runs the {{< param "PRODUCT_NAME" >}} binary with UID `473` and GID `473` instead of as `root`.
It also runs the `config reloader` sidecar as UID `65534` and GID `65534`.

## Set container permissions

Set `securityContext` at the Pod and container level to limit filesystem writes, privilege escalation, and Linux capabilities:

```yaml
spec:
  securityContext:
    runAsUser: 473
    runAsGroup: 473
    fsGroup: 473
    runAsNonRoot: true
  containers:
    - name: alloy
      securityContext:
        readOnlyRootFilesystem: true
        allowPrivilegeEscalation: false
        capabilities:
          drop:
            - ALL
```

- `runAsNonRoot: true` causes Kubernetes to reject the Pod if the image tries to run as root.
- `readOnlyRootFilesystem: true` blocks writes to the container filesystem except on mounted volumes.
- `allowPrivilegeEscalation: false` blocks privilege escalation beyond the parent process, regardless of file capabilities or `setuid` bits.
- `capabilities.drop: [ALL]` removes all Linux capabilities from the container.

The Helm chart sets `alloy.storagePath` to `/tmp/alloy` by default.
When you set `readOnlyRootFilesystem: true`, mount a writable volume at that path or change `alloy.storagePath` to a mounted volume.

{{< admonition type="note" >}}
If you use components that need elevated host access, such as `beyla.ebpf`, add back the capabilities those components need.
Don't drop all capabilities when these components are in your configuration.
Refer to the [beyla.ebpf component reference][beyla-ebpf-cap].

[beyla-ebpf-cap]: ../../reference/components/beyla/beyla.ebpf/
{{< /admonition >}}

## Restrict the HTTP server

The Grafana Helm chart sets `alloy.listenAddr` to `0.0.0.0` by default so other Pods can reach the container on port `12345`.
Set `alloy.listenAddr` to `127.0.0.1` in `values.yaml` or restrict access with a NetworkPolicy when you don't need cross-Pod access to the UI or `/metrics` endpoint.
The container image uses the binary default of `127.0.0.1:12345` when you don't pass `--server.http.listen-addr`.
Refer to the [`http` block][http-block] for TLS and authentication options.

## Kubernetes RBAC

{{< param "PRODUCT_NAME" >}} needs RBAC permissions to interact with Kubernetes APIs.
The Helm chart creates a `ClusterRole` and `ClusterRoleBinding` with a fixed rule set in `values.yaml` when `rbac.create` is `true`.

Remove permissions for resources your configuration doesn't use.
If you don't use Kubernetes service discovery or Pod log collection, review the generated RBAC rules and trim what you don't need.

Review the RBAC resources the Helm chart creates:

```shell
helm template alloy grafana/alloy --show-only templates/rbac.yaml
```

## Components that require elevated access

Some components need root or additional Linux capabilities.
Refer to [Components that require elevated access][elevated-access] for the full list and the [beyla.ebpf component reference][beyla-ebpf] for capability requirements on Kubernetes.

## Next steps

- [Access and permissions for {{< param "PRODUCT_NAME" >}}][access]
- [Access and permissions on Linux][linux]
- [Access and permissions on Windows][windows]

[image]: https://hub.docker.com/r/grafana/alloy
[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf/
[beyla-ebpf]: ../../reference/components/beyla/beyla.ebpf/
[security-context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[Grafana Helm chart]: ../../configure/kubernetes/#configure-the-helm-chart
[http-block]: ../../reference/config-blocks/http/
[elevated-access]: ../#components-that-require-elevated-access
[access]: ../
[linux]: ../linux/
[windows]: ../windows/
