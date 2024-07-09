---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.rules.kubernetes/
aliases:
  - ../loki.rules.kubernetes/ # /docs/alloy/latest/reference/components/loki.rules.kubernetes/
title: loki.rules.kubernetes
---

# loki.rules.kubernetes

`loki.rules.kubernetes` discovers `PrometheusRule` Kubernetes resources and
loads them into a Loki instance.

* You can specify multiple `loki.rules.kubernetes` components by giving them different labels.
* [Kubernetes label selectors][] can be used to limit the `Namespace` and
  `PrometheusRule` resources considered during reconciliation.
* Compatible with the Ruler APIs of Grafana Loki, Grafana Cloud, and Grafana Enterprise Metrics.
* Compatible with the `PrometheusRule` CRD from the [prometheus-operator][].
* This component accesses the Kubernetes REST API from [within a Pod][].

{{< admonition type="note" >}}
This component requires [Role-based access control (RBAC)][] to be set up
in Kubernetes for {{< param "PRODUCT_NAME" >}} to access it via the Kubernetes REST API.

[Role-based access control (RBAC)]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
{{< /admonition >}}

[Kubernetes label selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
[prometheus-operator]: https://prometheus-operator.dev/
[within a Pod]: https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/

## Usage

```alloy
loki.rules.kubernetes "LABEL" {
  address = LOKI_RULER_URL
}
```

## Arguments

`loki.rules.kubernetes` supports the following arguments:

Name                    | Type       | Description                                                                          | Default | Required
------------------------|------------|--------------------------------------------------------------------------------------|---------|---------
`address`               | `string`   | URL of the Loki ruler.                                                               |         | yes
`tenant_id`             | `string`   | Loki tenant ID.                                                                      |         | no
`use_legacy_routes`     | `bool`     | Whether to use deprecated ruler API endpoints.                                       | false   | no
`sync_interval`         | `duration` | Amount of time between reconciliations with Loki.                                    | "30s"   | no
`loki_namespace_prefix` | `string`   | Prefix used to differentiate multiple {{< param "PRODUCT_NAME" >}} deployments. | "alloy" | no
`bearer_token`          | `secret`   | Bearer token to authenticate with.                                                   |         | no
`bearer_token_file`     | `string`   | File containing a bearer token to authenticate with.                                 |         | no
`proxy_url`             | `string`   | HTTP proxy to proxy requests through.                                                |         | no
`follow_redirects`      | `bool`     | Whether redirects returned by the server should be followed.                         | `true`  | no
`enable_http2`          | `bool`     | Whether HTTP2 is supported for requests.                                             | `true`  | no

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

 [arguments]: #arguments

If no `tenant_id` is provided, the component assumes that the Loki instance at
`address` is running in single-tenant mode and no `X-Scope-OrgID` header is sent.

The `sync_interval` argument determines how often Loki's ruler API is accessed
to reload the current state. Interaction with the Kubernetes API works
differently. Updates are processed as events from the Kubernetes API server
according to the informer pattern.

You can use the `loki_namespace_prefix` argument to separate the rules managed
by multiple {{< param "PRODUCT_NAME" >}} deployments across your infrastructure. You should set the prefix to a
unique value for each deployment.

## Blocks

The following blocks are supported inside the definition of
`loki.rules.kubernetes`:

Hierarchy                                  | Block                  | Description                                              | Required
-------------------------------------------|------------------------|----------------------------------------------------------|---------
rule_namespace_selector                    | [label_selector][]     | Label selector for `Namespace` resources.                | no
rule_namespace_selector > match_expression | [match_expression][]   | Label match expression for `Namespace` resources.        | no
rule_selector                              | [label_selector][]     | Label selector for `PrometheusRule` resources.           | no
rule_selector > match_expression           | [match_expression][]   | Label match expression for `PrometheusRule` resources.   | no
basic_auth                                 | [basic_auth][]         | Configure basic_auth for authenticating to the endpoint. | no
authorization                              | [authorization][]      | Configure generic authorization to the endpoint.         | no
oauth2                                     | [oauth2][]             | Configure OAuth2 for authenticating to the endpoint.     | no
oauth2 > tls_config                        | [tls_config][]         | Configure TLS settings for connecting to the endpoint.   | no
tls_config                                 | [tls_config][]         | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting. For example,
`oauth2 > tls_config` refers to a `tls_config` block defined inside
an `oauth2` block.

[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block
[label_selector]: #label_selector-block
[match_expression]: #match_expression-block

### label_selector block

The `label_selector` block describes a Kubernetes label selector for rule or namespace discovery.

The following arguments are supported:

Name           | Type          | Description                                       | Default | Required
---------------|---------------|---------------------------------------------------|---------|---------
`match_labels` | `map(string)` | Label keys and values used to discover resources. | `{}`    | yes

When the `match_labels` argument is empty, all resources will be matched.

### match_expression block

The `match_expression` block describes a Kubernetes label match expression for rule or namespace discovery.

The following arguments are supported:

Name       | Type           | Description                        | Default | Required
-----------|----------------|------------------------------------|---------|---------
`key`      | `string`       | The label name to match against.   |         | yes
`operator` | `string`       | The operator to use when matching. |         | yes
`values`   | `list(string)` | The values used when matching.     |         | no

The `operator` argument should be one of the following strings:

* `"in"`
* `"notin"`
* `"exists"`

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.rules.kubernetes` does not export any fields.

## Component health

`loki.rules.kubernetes` is reported as unhealthy if given an invalid configuration or an error occurs during reconciliation.

## Debug information

`loki.rules.kubernetes` exposes resource-level debug information.

The following are exposed per discovered `PrometheusRule` resource:
* The Kubernetes namespace.
* The resource name.
* The resource uid.
* The number of rule groups.

The following are exposed per discovered Loki rule namespace resource:
* The namespace name.
* The number of rule groups.

Only resources managed by the component are exposed - regardless of how many
actually exist.

## Debug metrics

Metric Name                                  | Type        | Description
---------------------------------------------|-------------|-------------------------------------------------------------------------
`loki_rules_config_updates_total`            | `counter`   | Number of times the configuration has been updated.
`loki_rules_events_total`                    | `counter`   | Number of events processed, partitioned by event type.
`loki_rules_events_failed_total`             | `counter`   | Number of events that failed to be processed, partitioned by event type.
`loki_rules_events_retried_total`            | `counter`   | Number of events that were retried, partitioned by event type.
`loki_rules_client_request_duration_seconds` | `histogram` | Duration of requests to the Loki API.

## Example

This example creates a `loki.rules.kubernetes` component that loads discovered
rules to a local Loki instance under the `team-a` tenant. Only namespaces and
rules with the `alloy` label set to `yes` are included.

```alloy
loki.rules.kubernetes "local" {
    address = "loki:3100"
    tenant_id = "team-a"

    rule_namespace_selector {
        match_labels = {
            alloy = "yes",
        }
    }

    rule_selector {
        match_labels = {
            alloy = "yes",
        }
    }
}
```

This example creates a `loki.rules.kubernetes` component that loads discovered
rules to Grafana Cloud.

```alloy
loki.rules.kubernetes "default" {
    address = "GRAFANA_CLOUD_URL"
    basic_auth {
        username = "GRAFANA_CLOUD_USER"
        password = "GRAFANA_CLOUD_API_KEY"
        // Alternatively, load the password from a file:
        // password_file = "GRAFANA_CLOUD_API_KEY_PATH"
    }
}
```

The following example is an RBAC configuration for Kubernetes. It authorizes {{< param "PRODUCT_NAME" >}} to query the Kubernetes REST API:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: alloy
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: alloy
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["monitoring.coreos.com"]
  resources: ["prometheusrules"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: alloy
subjects:
- kind: ServiceAccount
  name: alloy
  namespace: default
roleRef:
  kind: ClusterRole
  name: alloy
  apiGroup: rbac.authorization.k8s.io
```
