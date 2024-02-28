---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/mimir.rules.kubernetes/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/mimir.rules.kubernetes/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/mimir.rules.kubernetes/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/mimir.rules.kubernetes/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/mimir.rules.kubernetes/
description: Learn about mimir.rules.kubernetes
labels:
  stage: beta
title: mimir.rules.kubernetes
---

# mimir.rules.kubernetes

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

`mimir.rules.kubernetes` discovers `PrometheusRule` Kubernetes resources and
loads them into a Mimir instance.

* Multiple `mimir.rules.kubernetes` components can be specified by giving them
  different labels.
* [Kubernetes label selectors][] can be used to limit the `Namespace` and
  `PrometheusRule` resources considered during reconciliation.
* Compatible with the Ruler APIs of Grafana Mimir, Grafana Cloud, and Grafana Enterprise Metrics.
* Compatible with the `PrometheusRule` CRD from the [prometheus-operator][].
* This component accesses the Kubernetes REST API from [within a Pod][].

> **NOTE**: This component requires [Role-based access control (RBAC)][] to be setup
> in Kubernetes in order for the Agent to access it via the Kubernetes REST API.
> For an example RBAC configuration please click [here](#example).

[Kubernetes label selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
[prometheus-operator]: https://prometheus-operator.dev/
[within a Pod]: https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/
[Role-based access control (RBAC)]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/

## Usage

```river
mimir.rules.kubernetes "LABEL" {
  address = MIMIR_RULER_URL
}
```

## Arguments

`mimir.rules.kubernetes` supports the following arguments:

Name                     | Type                | Description                                                     | Default       | Required
------------------------ | ------------------- | --------------------------------------------------------------- | ------------- | --------
`address`                | `string`            | URL of the Mimir ruler.                                         |               | yes
`tenant_id`              | `string`            | Mimir tenant ID.                                                |               | no
`use_legacy_routes`      | `bool`              | Whether to use [deprecated][gem-2_2] ruler API endpoints.       | false         | no
`prometheus_http_prefix` | `string`            | Path prefix for [Mimir's Prometheus endpoint][gem-path-prefix]. | `/prometheus` | no
`sync_interval`          | `duration`          | Amount of time between reconciliations with Mimir.              | "30s"         | no
`mimir_namespace_prefix` | `string`            | Prefix used to differentiate multiple {{< param "PRODUCT_NAME" >}} deployments. | "agent" | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.            |               | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                              |               | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                        | `true`        | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.    | `true`        | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                            |               | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. | | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.         | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests. |         | no

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

 [arguments]: #arguments

{{< docs/shared lookup="flow/reference/components/http-client-proxy-config-description.md" source="agent" version="<AGENT_VERSION>" >}}

If no `tenant_id` is provided, the component assumes that the Mimir instance at
`address` is running in single-tenant mode and no `X-Scope-OrgID` header is sent.

The `sync_interval` argument determines how often Mimir's ruler API is accessed
to reload the current state of rules. Interaction with the Kubernetes API works
differently. Updates are processed as events from the Kubernetes API server
according to the informer pattern.

The `mimir_namespace_prefix` argument can be used to separate the rules managed
by multiple {{< param "PRODUCT_NAME" >}} deployments across your infrastructure. It should be set to a
unique value for each deployment.

If `use_legacy_routes` is set to `true`, `mimir.rules.kubernetes` contacts Mimir on a `/api/v1/rules` endpoint.

If `prometheus_http_prefix` is set to `/mimir`, `mimir.rules.kubernetes` contacts Mimir on a `/mimir/config/v1/rules` endpoint. 
This is useful if you configure Mimir to use a different [prefix][gem-path-prefix] for its Prometheus endpoints than the default one.

`prometheus_http_prefix` is ignored if `use_legacy_routes` is set to `true`.

## Blocks

The following blocks are supported inside the definition of
`mimir.rules.kubernetes`:

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

Name           | Type          | Description                                       | Default                     | Required
---------------|---------------|---------------------------------------------------|-----------------------------|---------
`match_labels` | `map(string)` | Label keys and values used to discover resources. | `{}` | yes

When the `match_labels` argument is empty, all resources will be matched.

### match_expression block

The `match_expression` block describes a Kubernetes label match expression for rule or namespace discovery.

The following arguments are supported:

Name       | Type           | Description                                        | Default | Required
-----------|----------------|----------------------------------------------------|---------|---------
`key`      | `string`       | The label name to match against.                   |         | yes
`operator` | `string`       | The operator to use when matching.                 |         | yes
`values`   | `list(string)` | The values used when matching.                     |         | no

The `operator` argument should be one of the following strings:

* `"In"`
* `"NotIn"`
* `"Exists"`
* `"DoesNotExist"`

The `values` argument must not be provided when `operator` is set to `"Exists"` or `"DoesNotExist"`.

### basic_auth block

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### authorization block

{{< docs/shared lookup="flow/reference/components/authorization-block.md" source="agent" version="<AGENT_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="flow/reference/components/oauth2-block.md" source="agent" version="<AGENT_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="flow/reference/components/tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

## Exported fields

`mimir.rules.kubernetes` does not export any fields.

## Component health

`mimir.rules.kubernetes` is reported as unhealthy if given an invalid configuration or an error occurs during reconciliation.

## Debug information

`mimir.rules.kubernetes` exposes resource-level debug information.

The following are exposed per discovered `PrometheusRule` resource:
* The Kubernetes namespace.
* The resource name.
* The resource uid.
* The number of rule groups.

The following are exposed per discovered Mimir rule namespace resource:
* The namespace name.
* The number of rule groups.

Only resources managed by the component are exposed - regardless of how many
actually exist.

## Debug metrics

Metric Name                                   | Type        | Description
----------------------------------------------|-------------|-------------------------------------------------------------------------
`mimir_rules_config_updates_total`            | `counter`   | Number of times the configuration has been updated.
`mimir_rules_events_total`                    | `counter`   | Number of events processed, partitioned by event type.
`mimir_rules_events_failed_total`             | `counter`   | Number of events that failed to be processed, partitioned by event type.
`mimir_rules_events_retried_total`            | `counter`   | Number of events that were retried, partitioned by event type.
`mimir_rules_client_request_duration_seconds` | `histogram` | Duration of requests to the Mimir API.

## Example

This example creates a `mimir.rules.kubernetes` component that loads discovered
rules to a local Mimir instance under the `team-a` tenant. Only namespaces and
rules with the `agent` label set to `yes` are included.

```river
mimir.rules.kubernetes "local" {
    address = "mimir:8080"
    tenant_id = "team-a"

    rule_namespace_selector {
        match_labels = {
            agent = "yes",
        }
    }

    rule_selector {
        match_labels = {
            agent = "yes",
        }
    }
}
```

This example creates a `mimir.rules.kubernetes` component that loads discovered
rules to Grafana Cloud.

```river
mimir.rules.kubernetes "default" {
    address = "GRAFANA_CLOUD_METRICS_URL"
    basic_auth {
        username = "GRAFANA_CLOUD_USER"
        password = "GRAFANA_CLOUD_API_KEY"
        // Alternatively, load the password from a file:
        // password_file = "GRAFANA_CLOUD_API_KEY_PATH"
    }
}
```

The following example is an RBAC configuration for Kubernetes. It authorizes the Agent to query the Kubernetes REST API:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: grafana-agent
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: grafana-agent
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
  name: grafana-agent
subjects:
- kind: ServiceAccount
  name: grafana-agent
  namespace: default
roleRef:
  kind: ClusterRole
  name: grafana-agent
  apiGroup: rbac.authorization.k8s.io
```
