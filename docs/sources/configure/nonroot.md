---
canonical: https://grafana.com/docs/alloy/latest/configure/nonroot/
aliases:
  - ../tasks/nonroot/ # /docs/alloy/latest/tasks/nonroot/
description: Learn how to run the Alloy Docker container as a non-root user in Kubernetes
menuTitle: Non-root user
title: Run Alloy as a non-root user in Kubernetes
weight: 600
---

# Run {{% param "PRODUCT_NAME" %}} as a non-root user in Kubernetes

The [{{< param "PRODUCT_NAME" >}} Docker image][image] contains two users:

* A `root` user.
* A non-root user named `alloy` with uid `473` and gid `473`.

By default, the `alloy` binary runs as `root`. This is because some {{< param "PRODUCT_NAME" >}} components like [beyla.ebpf][] require root permissions.

You can configure a non-root user when you deploy {{< param "PRODUCT_NAME" >}} in Kubernetes.

## Configure {{% param "PRODUCT_NAME" %}} to run as a non-root user in Kubernetes

{{< admonition type="note" >}}
Running {{< param "PRODUCT_NAME" >}} as a non-root user won't work if you are using components like [beyla.ebpf][] that require root rights.

[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf
{{< /admonition >}}

To run {{< param "PRODUCT_NAME" >}} as a non-root user, configure a [security context][] for the {{< param "PRODUCT_NAME" >}} container. If you are using the [Grafana Helm chart][] you can add the following snippet to `values.yaml`:

```yaml
alloy:
  securityContext:
    runAsUser: 473
    runAsGroup: 473
```

This configuration makes the {{< param "PRODUCT_NAME" >}} binary run with UID 473 and GID 473 rather than as root.

## Is the root user a security risk?

Not really. The Linux kernel prevents Docker containers from accessing host resources. For example, Docker containers see a virtual file system rather than the host filesystem, a virtual network rather than the host network, a virtual process namespace rather than the host's processes. Even if the user inside the Docker container is `root` it can't break out of this virtual environment.

However, if there was a bug in the Linux kernel that allowed Docker containers to break out of the virtual environment, it would likely be easier to exploit this bug with a root user than with a non-root user. It's worth noting that the attacker would not only need to find such a Linux kernel bug, but would also need to find a way to make {{< param "PRODUCT_NAME" >}} exploit that bug.

[image]: https://hub.docker.com/r/grafana/alloy
[beyla.ebpf]: ../../reference/components/beyla/beyla.ebpf
[security context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[Grafana Helm chart]: ../../configure/kubernetes/#configure-the-helm-chart