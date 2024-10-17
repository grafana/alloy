---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/openshift/
description: Learn how to deploy Grafana Alloy on OpenShift
menuTitle: OpenShift
title: Deploy Grafana Alloy on OpenShift
weight: 250
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}} on OpenShift

You can deploy {{< param "PRODUCT_NAME" >}} on OpenShift.
It's important to ensure that Alloy conforms to OpenShift [security][] requirements.
This includes correctly configuring permissions and security policies to avoid vulnerabilities.
You can adapt the suggested policies and configuration to meet your specific needs and security policies.

## Before you begin

* These steps assume you have access to OpenShift in your environment.

## Configure Role-Based Access Control

You must configure RBAC to allow access to Kubernetes and OpenShift resources without compromising security.

1. Download the [rbac.yaml][] configuration file. This configuration file defines the OpenShift verbs and permissions for {{< param "PRODUCT_NAME" >}}.
1. Refer to [Managing Role-based Access Control (RBAC)][rbac] topic in the OpenShift documentation for more information about updating and managing your RBAC configurations.

## Run {{% param "FULL_PRODUCT_NAME" %}} as a non-root user

You must configure {{< param "PRODUCT_NAME" >}} to run as a non-root user.
This ensures that {{< param "PRODUCT_NAME" >}} complies with OpenShift security policies.

1. Configure {{< param "PRODUCT_NAME" >}} to [run as a non-root user][nonroot].

## Apply Security Context Constraints

OpenShift uses Security Context Constraints (SCC) to control pod permissions.
Refer to [Managing security context constraints][scc] for more information about how you can define and enforce these permissions.
This ensures that the pods running {{< param "PRODUCT_NAME" >}} comply with OpenShift security policies.

{{< admonition type="note" >}}
The security context is configured only at the container level, not at the container and deployment level.
{{< /admonition >}}

You can apply the following SCCs when you deploy {{< param "PRODUCT_NAME" >}}.

{{< admonition type="note" >}}
Not all of these SCCs are required for each use case.
You can adapt the SCCs to meet your local requirements and needs.
{{< /admonition >}}

* `RunAsUser`: Specifies the user ID under which Alloy will run.
   It must be configured to allow the use of a non-root user ID.
* `SELinuxContext`: Configures the SELinux context for containers.
   The appropriate SELinux context must be allowed for {{< param "PRODUCT_NAME" >}}, ensuring that SELinux policies allow the necessary operations.
   This SCC is generally not required to deploy {{< param "PRODUCT_NAME" >}} as a non-root user.
* `FSGroup`: Allows you to specify the group of file IDs for file system access.
   It must be configured to allow the {{< param "PRODUCT_NAME" >}} group to have proper access to the files it needs.
* `Volumes`: Allow the type of volumes required for {{< param "PRODUCT_NAME" >}}.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[rbac.yaml]: https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/templates/rbac.yaml
[rbac]: https://docs.openshift.com/container-platform/3.11/admin_guide/manage_rbac.html
[security]: https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/troubleshooting/#openshift-support
[nonroot]: ../../../configure/nonroot/
[scc]: https://docs.openshift.com/container-platform/4.6/authentication/managing-security-context-constraints.html
[Configure]: ../../../configure/linux/
