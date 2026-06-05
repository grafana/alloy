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

You can configure a non-root user when you deploy Alloy in Kubernetes.

{{< admonition type="note" >}}
Components like [beyla.ebpf][beyla-ebpf-note] and [pyroscope.ebpf][pyroscope-ebpf-note] need root or additional Linux capabilities.
Don't set `capabilities.drop: [ALL]` when these components are in your configuration.
Refer to the component references for required capabilities and Pod settings.

[beyla-ebpf-note]: ../../reference/components/beyla/beyla.ebpf/
[pyroscope-ebpf-note]: ../../reference/components/pyroscope/pyroscope.ebpf/
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

When you set `readOnlyRootFilesystem: true`, mount a writable volume at that path or change `alloy.storagePath` to a mounted volume.

{{< admonition type="note" >}}
If you use components that need elevated host access, for example [`beyla.ebpf`][beyla-ebpf-cap] or [`pyroscope.ebpf`][pyroscope-ebpf-cap], add the capabilities those components need.
Don't drop all capabilities when these components are in your configuration.
Refer to the component references for required capabilities and volume mounts.

[beyla-ebpf-cap]: ../../reference/components/beyla/beyla.ebpf/
[pyroscope-ebpf-cap]: ../../reference/components/pyroscope/pyroscope.ebpf/
{{< /admonition >}}

## Restrict the HTTP server

The Grafana Helm chart sets `alloy.listenAddr` to `0.0.0.0` by default so other Pods can reach the container on port `12345`.
Set `alloy.listenAddr` to `127.0.0.1` in `values.yaml` or restrict access with a NetworkPolicy when you don't need cross-Pod access to the UI or `/metrics` endpoint.
The container image uses the binary default of `127.0.0.1:12345` when you don't pass `--server.http.listen-addr`.
Refer to the [`http` block][http-block] for TLS and authentication options.

## Kubernetes RBAC

{{< param "PRODUCT_NAME" >}} needs RBAC permissions to interact with Kubernetes APIs.
The Helm chart creates a `ClusterRole` and `ClusterRoleBinding` when `rbac.create` is `true`.
The Helm chart sets `rbac.rules` and `rbac.clusterRules` in `values.yaml`.
Refer to the [Grafana Helm chart][] `values.yaml` or README for the default rule blocks and the components each one supports.

To limit permissions, set `rbac.rules` and `rbac.clusterRules` to only the rule blocks your configuration uses.
Helm replaces each array in full, so copy the defaults and remove the blocks you don't need.
Set `rbac.create` to `false` if you manage RBAC outside the chart.

Review the RBAC resources the Helm chart creates:

```shell
helm template alloy grafana/alloy --show-only templates/rbac.yaml
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}} on Kubernetes][configure-kubernetes]
- [Monitor Kubernetes logs with {{< param "PRODUCT_NAME" >}}][monitor-kubernetes-logs]
- [Collect and forward data][collect]

[image]: https://hub.docker.com/r/grafana/alloy
[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf/
[pyroscope-ebpf]: ../../reference/components/pyroscope/pyroscope.ebpf/
[security-context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[Grafana Helm chart]: ../../configure/kubernetes/#configure-the-helm-chart
[http-block]: ../../reference/config-blocks/http/
[configure-kubernetes]: ../../configure/kubernetes/
[monitor-kubernetes-logs]: ../../monitor/monitor-kubernetes-logs/
[collect]: ../../collect/
