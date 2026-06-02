---
canonical: https://grafana.com/docs/alloy/latest/secure/kubernetes/
aliases:
  - ../configure/nonroot/ # /docs/alloy/latest/configure/nonroot/
  - ../../configure/nonroot/ # /docs/alloy/latest/configure/nonroot/
  - ../tasks/nonroot/ # /docs/alloy/latest/tasks/nonroot/
  - ../harden-kubernetes/ # /docs/alloy/latest/secure/harden-kubernetes/
description: Secure Grafana Alloy on Kubernetes using `securityContext`, non-root users, capability drops, and OpenShift Security Context Constraints
menuTitle: Secure Kubernetes
title: Secure Grafana Alloy on Kubernetes
weight: 200
---

# Secure {{% param "FULL_PRODUCT_NAME" %}} on Kubernetes

You can run {{< param "PRODUCT_NAME" >}} on Kubernetes with a non-root configuration, `securityContext`, capability restrictions, and OpenShift Security Context Constraints.

## Run as a non-root user

The [{{< param "PRODUCT_NAME" >}} Docker image][image] contains two users: `root` and a non-root user named `alloy` with UID `473` and GID `473`.
By default, the container runs the `alloy` binary as `root` because some components, such as [beyla.ebpf][], require root permissions.

{{< admonition type="note" >}}
Running {{< param "PRODUCT_NAME" >}} as a non-root user doesn't work if you use components like [beyla.ebpf][] that require root access.
{{< /admonition >}}

To run {{< param "PRODUCT_NAME" >}} as a non-root user, configure a [security context][security-context] for the {{< param "PRODUCT_NAME" >}} container.
If you use the [Grafana Helm chart][], add the following to `values.yaml`:

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

This configuration runs the {{< param "PRODUCT_NAME" >}} binary with UID `473` and GID `473` rather than root, and runs the `configReloader` sidecar as UID `65534`.

## Apply a full security context

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

- `runAsNonRoot: true` causes Kubernetes to reject the Pod if the image tries to run as root, which provides a safety net.
- `readOnlyRootFilesystem: true` prevents the process from writing anywhere in the container filesystem except explicitly mounted volumes.
- `allowPrivilegeEscalation: false` prevents the process from gaining more privileges than its parent, regardless of file capabilities or `setuid` bits.
- `capabilities.drop: [ALL]` removes all Linux capabilities from the container.

{{< admonition type="note" >}}
If you use components that require elevated host access, such as `beyla.ebpf`, add back the capabilities those components need instead of dropping all capabilities.
Refer to the [beyla.ebpf][] component reference.

[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf/
{{< /admonition >}}

## Restrict the HTTP server

The Grafana Helm chart sets `alloy.listenAddr` to `0.0.0.0` by default so other Pods can reach the container on port `12345`.
If you don't need cross-Pod access to the UI or `/metrics` endpoint, set `alloy.listenAddr` to `127.0.0.1` in `values.yaml` or restrict access with a NetworkPolicy.
The standalone binary binds to `127.0.0.1:12345` by default.

For configuration options, refer to the [`http` block][http-block].

## Container root and host root

UID `0` inside a container isn't UID `0` on the node.
The container runtime runs the process in an isolated namespace, so container `root` can't read host files or processes under normal operation.

Run as UID `473` anyway.
A non-root UID limits damage on the node if a container escape bug appears in the kernel or runtime.

## Kubernetes RBAC

{{< param "PRODUCT_NAME" >}} requires RBAC permissions to interact with Kubernetes APIs.
The Helm chart creates a `ClusterRole` and `ClusterRoleBinding` with permissions for the default component set.

Scope these permissions to what your specific configuration actually uses.
If you aren't using Kubernetes service discovery or Pod log collection, review the generated RBAC rules and remove permissions for resources you don't need.

To review the RBAC resources the Helm chart creates:

```shell
helm template grafana/alloy --show-only templates/rbac.yaml
```

## Deploy on OpenShift

Red Hat OpenShift Container Platform uses Security Context Constraints to control Pod permissions.
The standard Kubernetes `securityContext` settings work on OpenShift, but you must also configure RBAC and Security Context Constraints to satisfy OpenShift security policies.

### Configure RBAC

Download the [rbac.yaml][] configuration file, which defines the OpenShift verbs and permissions for {{< param "PRODUCT_NAME" >}}.
Review it and adapt it to your environment before you apply it.

Refer to [Managing Role-based Access Control][rbac] in the OpenShift documentation for more information.

### Apply security context constraints

You can apply the following Security Context Constraints when you deploy {{< param "PRODUCT_NAME" >}}:

- **`RunAsUser`**: Configure this to allow the non-root UID `473`.
- **`FSGroup`**: Configure this to give {{< param "PRODUCT_NAME" >}} group access to its required files.
- **`Volumes`**: Configure this to allow access to the volumes {{< param "PRODUCT_NAME" >}} needs.
- **`SELinuxContext`**: Configure this if you run as root and `SELinux` policies would otherwise block {{< param "PRODUCT_NAME" >}}.
  Generally not required for non-root deployments.

{{< admonition type="note" >}}
Not all Security Context Constraints are required for every use case.
Adapt them to your local requirements.
{{< /admonition >}}

### Example DaemonSet configuration

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

Replace `<ALLOY_VERSION>` with the specific version you deploy, for example `v1.5.1`.

{{< admonition type="note" >}}
`emptyDir` volumes don't persist data across node restarts.
In production, use a persistent storage volume so data survives node restarts.
Refer to [Using volumes to persist container data][ocp-volumes] in the OpenShift documentation.
{{< /admonition >}}

### Example Security Context Constraint definition

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

Replace the `<SYSTEM_USER>`, `<SYSTEM_ROLE>`, `<CONTAINER_TYPE>`, and `<LEVEL>` placeholders with values appropriate for your `SELinux` context.
Refer to [`SELinux` contexts][selinux] in the Red Hat documentation for more information.

## Next steps

- [Secure {{< param "PRODUCT_NAME" >}}][secure]: overview of all security areas
- [Secure {{< param "PRODUCT_NAME" >}} on Linux][linux]
- [Secure {{< param "PRODUCT_NAME" >}} on Windows][windows]

[image]: https://hub.docker.com/r/grafana/alloy
[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf/
[security-context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[Grafana Helm chart]: ../../configure/kubernetes/#configure-the-helm-chart
[http-block]: ../../reference/config-blocks/http/
[rbac.yaml]: https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/templates/rbac.yaml
[rbac]: https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html
[ocp-volumes]: https://docs.openshift.com/container-platform/latest/nodes/containers/nodes-containers-volumes.html
[selinux]: https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/6/html/security-enhanced_linux/chap-security-enhanced_linux-selinux_contexts
[secure]: ../
[linux]: ../linux/
[windows]: ../windows/
