---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/openshift/
description: Learn how to deploy Grafana Alloy on OpenShift
menuTitle: OpenShift
title: Deploy Grafana Alloy on OpenShift
weight: 650
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}} on OpenShift

You can deploy {{< param "PRODUCT_NAME" >}} on the Red Hat OpenShift Container Platform (OCP).

## Before you begin

* These steps assume you have a working OCP environment.
* You can adapt the suggested policies and configuration to meet your specific needs and security policies.

## Configure RBAC

You must configure Role-Based Access Control (RBAC) to allow secure access to Kubernetes and OCP resources.

1. Download the [rbac.yaml][] configuration file. This configuration file defines the OCP verbs and permissions for {{< param "PRODUCT_NAME" >}}.
1. Review the `rbac.yaml` file and adapt as needed for your local environment. Refer to [Managing Role-based Access Control (RBAC)][rbac] topic in the OCP documentation for more information about updating and managing your RBAC configurations.

## Run {{% param "PRODUCT_NAME" %}} as a non-root user

You must configure {{< param "PRODUCT_NAME" >}} to [run as a non-root user][nonroot].
This ensures that {{< param "PRODUCT_NAME" >}} complies with your OCP security policies.

## Apply security context constraints

OCP uses Security Context Constraints (SCC) to control Pod permissions.
Refer to [Managing security context constraints][scc] for more information about how you can define and enforce these permissions.
This ensures that the pods running {{< param "PRODUCT_NAME" >}} comply with OCP security policies.

{{< admonition type="note" >}}
The security context is only configured at the container level, not at the container and deployment level.
{{< /admonition >}}

You can apply the following SCCs when you deploy {{< param "PRODUCT_NAME" >}}.

{{< admonition type="note" >}}
Not all of these SCCs are required for each use case.
You can adapt the SCCs to meet your local requirements and needs.
{{< /admonition >}}

* `RunAsUser`: Specifies the user ID under which {{< param "PRODUCT_NAME" >}} runs.
  You must configure this constraint to allow a non-root user ID.
* `SELinuxContext`: Configures the SELinux context for containers.
  If you run {{< param "PRODUCT_NAME" >}} as root, you must configure this constraint to make sure that SELinux policies don't block {{< param "PRODUCT_NAME" >}}.
  This SCC is generally not required to deploy {{< param "PRODUCT_NAME" >}} as a non-root user.
* `FSGroup`: Specifies the fsGroup IDs for file system access.
  You must configure this constraint to give {{< param "PRODUCT_NAME" >}} group access to the files it needs.
* `Volumes`: Specifies the persistent volumes used for storage.
  You must configure this constraint to give {{< param "PRODUCT_NAME" >}} access to the volumes it needs.

## Example DaemonSet configuration

The following example shows a DaemonSet configuration that deploys {{< param "PRODUCT_NAME" >}} as a non-root user:

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

Replace the following:

* _`<ALLOY_VERSION>`_: Set to the specific {{< param "PRODUCT_NAME" >}} version you are deploying. For example, `1.5.1`.

{{< admonition type="note" >}}
This example uses the simplest volume type, `emptyDir`. In this example configuration, if your node restarts, your data will be lost. Make sure you set the volume type to a persistent storage location for production environments. Refer to [Using volumes to persist container data](https://docs.openshift.com/container-platform/latest/nodes/containers/nodes-containers-volumes.html) in the OpenShift documentation for more information.
{{< /admonition >}}

## Example SCC definition

The following example shows an SCC definition that deploys {{< param "PRODUCT_NAME" >}} as a non-root user:

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
- '*'
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

Replace the following:

* _`<SYSTEM_USER>`_: The user for your SELinux context.
* _`<SYSTEM_ROLE>`_: The role for your SELinux context.
* _`<CONTAINER_TYPE>`_: The container type for your SELinux context.
* _`<LEVEL>`_: The level  for your SELinux context.

Refer to [SELinux Contexts][selinux] in the RedHat documentation for more information on the SELinux context configuration.

{{< admonition type="note" >}}
This example sets `volumes:` to `*`. In a production environment, you should set `volumes:` to only the volumes that are necessary for the deployment. For example:

```yaml
volumes:
  - configMap
  - downwardAPI
  - emptyDir
  - persistentVolumeClaim
  - secret 
```

{{< /admonition >}}

Refer to [Deploy {{< param "FULL_PRODUCT_NAME" >}}][deploy] for more information about deploying {{< param "PRODUCT_NAME" >}} in your environment.

## Next steps

* [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[rbac.yaml]: https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/templates/rbac.yaml
[rbac]: https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html
[nonroot]: ../../../configure/nonroot/
[scc]: https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html
[Configure]: ../../../configure/linux/
[deploy]: ../../deploy/
[selinux]: https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/6/html/security-enhanced_linux/chap-security-enhanced_linux-selinux_contexts#chap-Security-Enhanced_Linux-SELinux_Contexts
