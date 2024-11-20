---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/openshift/
description: Learn how to deploy Grafana Alloy on OpenShift
menuTitle: OpenShift
title: Deploy Grafana Alloy on OpenShift
weight: 250
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}} on OpenShift

You can deploy {{< param "PRODUCT_NAME" >}} on the Red Hat OpenShift Container Platform (OCP).

## Before you begin

* These steps assume you have a working OCP environment.
* You can adapt the suggested policies and configuration to meet your specific needs and [security][] policies.

## Configure Role-Based Access Control

You must configure RBAC to allow secure access to Kubernetes and OCP resources.

1. Download the [rbac.yaml][] configuration file. This configuration file defines the OCP verbs and permissions for {{< param "PRODUCT_NAME" >}}.
1. Review the `rbac.yaml` file and adapt as needed for your local environment. Refer to [Managing Role-based Access Control (RBAC)][rbac] topic in the OCP documentation for more information about updating and managing your RBAC configurations.

## Run {{% param "PRODUCT_NAME" %}} as a non-root user

You must configure {{< param "PRODUCT_NAME" >}} to [run as a non-root user][nonroot].
This ensures that {{< param "PRODUCT_NAME" >}} complies with your OCP security policies.

## Apply Security Context Constraints

OCP uses Security Context Constraints (SCC) to control pod permissions.
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

The following example shows a SCC configuration that deploys {{< param "PRODUCT_NAME" >}} as a non-root user:

```yaml
apiVersion: aapps/v1
kind: DaemonSet
metadata:
  name: alloy-logs
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: alloy-logs
   template:
     metadata:
       lables:
         app: alloy-logs
      spec:
        containers:
        - name: alloy-logs
          image: grafana/alloy:latest
          ports:
          - containerPort: 12345
          securityContext:
            allowPrivilegeEscalation: false
            runAsUser: 473
            runAsGroup: 473
            # fsGroup: 1000
         volumes:
         - name: log-volume
           emptyDir: {}
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[rbac.yaml]: https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/templates/rbac.yaml
[rbac]: https://docs.openshift.com/container-platform/3.11/admin_guide/manage_rbac.html
[security]: https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/troubleshooting/#openshift-support
[nonroot]: ../../../configure/nonroot/
[scc]: https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html
[Configure]: ../../../configure/linux/
