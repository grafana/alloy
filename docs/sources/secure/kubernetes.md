---
canonical: https://grafana.com/docs/alloy/latest/secure/kubernetes/
aliases:
  - ../configure/nonroot/ # /docs/alloy/latest/configure/nonroot/
  - ../../configure/nonroot/ # /docs/alloy/latest/configure/nonroot/
  - ../tasks/nonroot/ # /docs/alloy/latest/tasks/nonroot/
description: Secure Grafana Alloy on Kubernetes using `securityContext`, non-root users, capability drops, and OpenShift Security Context Constraints
menuTitle: Kubernetes
title: Secure Grafana Alloy on Kubernetes
weight: 200
---

# Secure {{% param "FULL_PRODUCT_NAME" %}} on Kubernetes

Configure `securityContext`, RBAC, and network settings to match the components in your {{< param "PRODUCT_NAME" >}} configuration.
The container image defines a non-root user named `alloy` with UID `473` and GID `473` for this purpose.

For Linux, refer to [Secure {{< param "PRODUCT_NAME" >}} on Linux][linux].
For Windows, refer to [Secure {{< param "PRODUCT_NAME" >}} on Windows][windows].

## Run as a non-root user

The [{{< param "PRODUCT_NAME" >}} Docker image][image] defines two users: `root` and a non-root user named `alloy` with UID `473` and GID `473`.
The container runs the `alloy` binary as `root` by default because some components, such as [beyla.ebpf][], need elevated privileges.

UID `0` inside a container isn't UID `0` on the node.
The container runtime isolates the process, so container `root` can't read host files or processes under normal operation.
Use UID `473` to limit damage on the node if a container escape bug appears in the kernel or runtime.

{{< admonition type="note" >}}
Components like [beyla.ebpf][beyla-ebpf-note] need root or additional Linux capabilities.
Don't set `capabilities.drop: [ALL]` when these components are in your configuration.
Refer to the [beyla.ebpf component reference][beyla-ebpf-note].

[beyla-ebpf-note]: ../../reference/components/beyla/beyla.ebpf/
{{< /admonition >}}

Configure a [security context][security-context] for the {{< param "PRODUCT_NAME" >}} container to run as UID `473`.
If you use the [Grafana Helm chart][], add to `values.yaml`:

```yaml
alloy:
  securityContext:
    runAsUser: 473
    runAsGroup: 473

configReloader:
  securityContext:
    # UID of the "nobody" user that the config reloader image runs as
    runAsUser: 65534
    runAsGroup: 65534
```

The Helm values run the {{< param "PRODUCT_NAME" >}} binary with UID `473` and GID `473`, and run the `configReloader` sidecar as UID `65534`.

## Secure the container

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
- `readOnlyRootFilesystem: true` prevents the process from writing anywhere in the container filesystem except mounted volumes.
- `allowPrivilegeEscalation: false` prevents the process from gaining more privileges than its parent, regardless of file capabilities or `setuid` bits.
- `capabilities.drop: [ALL]` removes all Linux capabilities from the container.

{{< admonition type="note" >}}
If you use components that need elevated host access, such as `beyla.ebpf`, add back the capabilities those components need instead of dropping all capabilities.
Refer to the [beyla.ebpf component reference][beyla-ebpf-cap].

[beyla-ebpf-cap]: ../../reference/components/beyla/beyla.ebpf/
{{< /admonition >}}

## Restrict the HTTP server

The Grafana Helm chart sets `alloy.listenAddr` to `0.0.0.0` by default so other Pods can reach the container on port `12345`.
Set `alloy.listenAddr` to `127.0.0.1` in `values.yaml` or restrict access with a NetworkPolicy when you don't need cross-Pod access to the UI or `/metrics` endpoint.
The standalone binary binds to `127.0.0.1:12345` by default.
Refer to the [`http` block][http-block] for TLS and authentication options.

## Kubernetes RBAC

{{< param "PRODUCT_NAME" >}} needs RBAC permissions to interact with Kubernetes APIs.
The Helm chart creates a `ClusterRole` and `ClusterRoleBinding` with permissions for the default component set.

Remove permissions for resources your configuration doesn't use.
If you don't use Kubernetes service discovery or Pod log collection, review the generated RBAC rules and trim what you don't need.

Review the RBAC resources the Helm chart creates:

```shell
helm template grafana/alloy --show-only templates/rbac.yaml
```

## Deploy on OpenShift

Red Hat OpenShift Container Platform uses Security Context Constraints to control Pod permissions.
The standard Kubernetes `securityContext` settings work on OpenShift, but you must also configure RBAC and Security Context Constraints to satisfy OpenShift security policies.

### Configure RBAC

Download the [rbac.yaml][] configuration file, which defines the OpenShift verbs and permissions for {{< param "PRODUCT_NAME" >}}.
Review and adapt it to your environment before you apply it.
Refer to [Managing Role-based Access Control][rbac] in the OpenShift documentation.

### Apply security context constraints

Configure these Security Context Constraints when you deploy {{< param "PRODUCT_NAME" >}} on OpenShift:

- **`RunAsUser`**: Allow UID `473`.
- **`FSGroup`**: Set a group ID that matches your volume mounts.
- **`Volumes`**: Allow the volume types your deployment uses.
- **`SELinuxContext`**: Set a context that matches your `SELinux` policy when you run as root.
  Skip this constraint for non-root deployments.

### OpenShift DaemonSet

Deploy {{< param "PRODUCT_NAME" >}} as a non-root user on OpenShift with a DaemonSet like the one below:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: alloy-logs
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: alloy-logs
  template:
    metadata:
      labels:
        app: alloy-logs
    spec:
      securityContext:
        runAsUser: 473
        runAsGroup: 473
        fsGroup: 1000
      containers:
        - name: alloy-logs
          image: grafana/alloy:<ALLOY_VERSION>
          ports:
            - containerPort: 12345
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
      volumes:
        - name: log-volume
          emptyDir: {}
```

Replace `<ALLOY_VERSION>` with the version you deploy, for example `v1.5.1`.

{{< admonition type="note" >}}
`emptyDir` volumes don't persist data across node restarts.
Use a persistent storage volume in production.
Refer to [Using volumes to persist container data][ocp-volumes-note] in the OpenShift documentation.

[ocp-volumes-note]: https://docs.openshift.com/container-platform/latest/nodes/containers/nodes-containers-volumes.html
{{< /admonition >}}

### Security Context Constraint on OpenShift

```yaml
kind: SecurityContextConstraints
apiVersion: security.openshift.io/v1
metadata:
  name: scc-alloy
runAsUser:
  type: MustRunAs
  uid: 473
fsGroup:
  type: MustRunAs
  uid: 1000
volumes:
  - configMap
  - downwardAPI
  - emptyDir
  - persistentVolumeClaim
  - secret
users:
  - my-admin-user
groups:
  - my-admin-group
seLinuxContext:
  type: MustRunAs
  user: <SYSTEM_USER>
  role: <SYSTEM_ROLE>
  type: <CONTAINER_TYPE>
  level: <LEVEL>
```

Replace the `<SYSTEM_USER>`, `<SYSTEM_ROLE>`, `<CONTAINER_TYPE>`, and `<LEVEL>` placeholders with values for your `SELinux` context.
Refer to [Security context constraints for pods and containers][selinux-ocp] in the OpenShift documentation.

## Components that require elevated access

Some components need root or additional Linux capabilities.
Refer to [Components that require elevated access][elevated-access] for the full list and the [beyla.ebpf component reference][beyla-ebpf] for capability requirements on Kubernetes.

## Next steps

- [Secure {{< param "PRODUCT_NAME" >}}][secure]
- [Secure {{< param "PRODUCT_NAME" >}} on Linux][linux]
- [Secure {{< param "PRODUCT_NAME" >}} on Windows][windows]

[image]: https://hub.docker.com/r/grafana/alloy
[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf/
[beyla-ebpf]: ../../reference/components/beyla/beyla.ebpf/
[security-context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[Grafana Helm chart]: ../../configure/kubernetes/#configure-the-helm-chart
[http-block]: ../../reference/config-blocks/http/
[rbac.yaml]: https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/templates/rbac.yaml
[rbac]: https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html
[selinux-ocp]: https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html
[elevated-access]: ../#components-that-require-elevated-access
[secure]: ../
[linux]: ../linux/
[windows]: ../windows/
