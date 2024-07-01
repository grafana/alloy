---
canonical: https://grafana.com/docs/alloy/latest/tasks/nonroot/
description: Learn how to run the Alloy Docker container as a non-root user in Kubernetes
title: Run Alloy as a non-root user in Kubernetes
weight: 600
---

# Run Alloy as a non-root user in Kubernetes

The [Alloy Docker image] contains two users:

* A `root` user.
* A non-root user named `alloy` with uid `473` and gid `473`.

By default, the `alloy` binary runs as `root`. This is because some Alloy components like [beyla.ebpf] require root permissions.

This page demonstrates how to configure a non-root user when deploying Alloy in Kubernetes.

## Configure Alloy to run as a non-root user in Kubernetes

{{< admonition type="note" >}}
Running Alloy as a non-root user will not work if you are using components like [beyla.ebpf] that require root rights.
{{< /admonition >}}

In order to run Alloy as a non-root user, you need to configure a [security context] for the Alloy container. If you are using the [Grafana Helm chart] you can do this by adding the following snippet to `values.yaml`:

```
alloy:
  securityContext:
    runAsUser: 473
    runAsGroup: 473
```

This will make the Alloy binary to run with uid 473 and gid 473 rather than as root.

## Is the root user a security risk?

Not really. The Linux kernel prevents Docker containers from accessing host resources. For example, Docker containers will see a virtual file system rather than the host filesystem, a virtual network rather than the host network, a virtual process namespace rather than the host's processes. Even if the user inside the Docker container is `root` it cannot break out of this virtual environment.

However, if there was a bug in the Linux kernel that allowed Docker containers to break out of the virtual environment, it would likely be easier to exploit this bug with a root user than with a non-root user. It is worth noting that the attacker would not only need to find such a Linux kernel bug, but would also need to find a way to make Alloy exploit that bug.

[Alloy Docker image]: https://hub.docker.com/r/grafana/alloy
[beyla.ebpf]: ../../reference/components/beyla.ebpf
[security context]: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
[Grafana Helm chart]: https://grafana.com/docs/alloy/latest/tasks/configure/configure-kubernetes/#configure-the-helm-chart