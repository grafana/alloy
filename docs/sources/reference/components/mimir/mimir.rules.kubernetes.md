---
canonical: https://grafana.com/docs/alloy/latest/reference/components/mimir/mimir.rules.kubernetes/
aliases:
  - ../mimir.rules.kubernetes/ # /docs/alloy/latest/reference/components/mimir.rules.kubernetes/
description: Learn about mimir.rules.kubernetes
labels:
  stage: general-availability
  products:
    - oss
title: mimir.rules.kubernetes
---

# `mimir.rules.kubernetes`

`mimir.rules.kubernetes` discovers `PrometheusRule` Kubernetes resources and loads them into a Mimir instance.

It can also [federate rules](https://grafana.com/docs/mimir/latest/references/architecture/components/ruler/#federated-rule-groups) from different source tenants into a target Mimir tenant by interpreting the `monitoring.grafana.com/source_tenants` annotation on `PrometheusRule` resources.

* You can specify multiple `mimir.rules.kubernetes` components by giving them different labels.
* [Kubernetes label selectors][] can be used to limit the `Namespace` and `PrometheusRule` resources considered during reconciliation.
* Compatible with the Ruler APIs of Grafana Mimir, Grafana Cloud, and Grafana Enterprise Metrics.
* Compatible with the `PrometheusRule` CRD from the [`prometheus-operator`][prometheus-operator].
* This component accesses the Kubernetes REST API from [within a Pod][].

{{< admonition type="note" >}}
This component requires [Role-based access control (RBAC)][] to be set up in Kubernetes in order for {{< param "PRODUCT_NAME" >}} to access it via the Kubernetes REST API.

[Role-based access control (RBAC)]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
{{< /admonition >}}

{{< admonition type="note" >}}
{{< param "PRODUCT_NAME" >}} version 1.1 and higher supports [clustered mode][] in this component.
When you use this component as part of a cluster of {{< param "PRODUCT_NAME" >}} instances, only a single instance from the cluster updates rules using the Mimir API.

[clustered mode]: ../../../../get-started/clustering/
{{< /admonition >}}

[Kubernetes label selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
[prometheus-operator]: https://prometheus-operator.dev/
[within a Pod]: https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/

## Usage

```alloy
mimir.rules.kubernetes "<LABEL>" {
  address = "<MIMIR_RULER_URL>"
}
```

## Arguments

You can use the following arguments with `mimir.rules.kubernetes`:

| Name                     | Type                | Description                                                                                      | Default         | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | --------------- | -------- |
| `address`                | `string`            | URL of the Mimir ruler.                                                                          |                 | yes      |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |                 | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |                 | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`          | no       |
| `external_labels`        | `map(string)`       | Labels to add to each rule.                                                                      | `{}`            | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`          | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |                 | no       |
| `mimir_namespace_prefix` | `string`            | Prefix used to differentiate multiple {{< param "PRODUCT_NAME" >}} deployments.                  | `"alloy"`       | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                 | no       |
| `prometheus_http_prefix` | `string`            | Path prefix for the [Mimir Prometheus endpoint][gem-path-prefix].                                | `"/prometheus"` | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                 | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`         | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |                 | no       |
| `sync_interval`          | `duration`          | Amount of time between reconciliations with Mimir.                                               | `"5m"`          | no       |
| `tenant_id`              | `string`            | Mimir tenant ID.                                                                                 |                 | no       |
| `use_legacy_routes`      | `bool`              | Whether to use deprecated ruler API endpoints.                                                   | `false`         | no       |

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments]argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

 [arguments]: #arguments
 [gem-path-prefix]: https://grafana.com/docs/mimir/latest/references/http-api/

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

If no `tenant_id` is provided, the component assumes that the Mimir instance at `address` is running in single-tenant mode and no `X-Scope-OrgID` header is sent.

The `sync_interval` argument determines how often the Mimir ruler API is accessed to reload the current state of rules.
Interaction with the Kubernetes API works differently.
Updates are processed as events from the Kubernetes API server according to the informer pattern.

The `mimir_namespace_prefix` argument can be used to separate the rules managed by multiple {{< param "PRODUCT_NAME" >}} deployments across your infrastructure.
It should be set to a unique value for each deployment.

If `use_legacy_routes` is set to `true`, `mimir.rules.kubernetes` contacts Mimir on a `/api/v1/rules` endpoint.

If `prometheus_http_prefix` is set to `/mimir`, `mimir.rules.kubernetes` contacts Mimir on a `/mimir/config/v1/rules` endpoint.
This is useful if you configure Mimir to use a different [prefix][gem-path-prefix] for its Prometheus endpoints than the default one.

`prometheus_http_prefix` is ignored if `use_legacy_routes` is set to `true`.

`external_labels` overrides label values if labels with the same names already exist inside the rule.

## Blocks

You can use the following blocks with `mimir.rules.kubernetes`:

| Block                                                              | Description                                                | Required |
| ------------------------------------------------------------------ | ---------------------------------------------------------- | -------- |
| [`authorization`][authorization]                                   | Configure generic authorization to the endpoint.           | no       |
| [`basic_auth`][basic_auth]                                         | Configure `basic_auth` for authenticating to the endpoint. | no       |
| [`extra_query_matchers`][extra_query_matchers]                     | Additional label matchers to add to each query.            | no       |
| `extra_query_matchers` > [`matcher`][matcher]                      | A label matcher to add to query.                           | no       |
| [`oauth2`][oauth2]                                                 | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `oauth2` > [`tls_config`][tls_config]                              | Configure TLS settings for connecting to the endpoint.     | no       |
| [`rule_namespace_selector`][label_selector]                        | Label selector for `Namespace` resources.                  | no       |
| `rule_namespace_selector` > [`match_expression`][match_expression] | Label match expression for `Namespace` resources.          | no       |
| [`rule_selector`][label_selector]                                  | Label selector for `PrometheusRule` resources.             | no       |
| `rule_selector` > [`match_expression`][match_expression]           | Label match expression for `PrometheusRule` resources.     | no       |
| [`tls_config`][tls_config]                                         | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[authorization]: #authorization
[basic_auth]: #basic_auth
[extra_query_matchers]: #extra_query_matchers
[label_selector]: #rule_selector-and-rule_namespace_selector
[match_expression]: #match_expression
[matcher]: #matcher
[oauth2]: #oauth2
[tls_config]: #tls_config

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `extra_query_matchers`

The `extra_query_matchers` block has no attributes.
It contains zero or more [matcher][] blocks.
These blocks allow you to add extra label matchers to all queries that are discovered by `mimir.rules.kubernetes` component.
The algorithm of adding the label matchers to queries is the same as the one provided by the [`promtool promql label-matchers set` command](https://prometheus.io/docs/prometheus/latest/command-line/promtool/#promtool-promql).

### `matcher`

The `matcher` block describes a label matcher that's added to each query found in `PrometheusRule` CRDs.

The following arguments are supported:

| Name               | Type     | Description                                              | Default | Required |
| ------------------ | -------- | -------------------------------------------------------- | ------- | -------- |
| `match_type`       | `string` | The type of match. One of `=`, `!=`, `=~` and `!~`.      |         | yes      |
| `name`             | `string` | Name of the label to match.                              |         | yes      |
| `value`            | `string` | Value of the label to match.                             |         | no       |
| `value_from_label` | `string` | Value of the Kubernetes `PrometheusRule` label to match. |         | no       |

Only one of `value` or `value_from_label` can be used. 

### `rule_selector` and `rule_namespace_selector`

The `rule_selector` and `rule_namespace_selector` blocks describe a Kubernetes label selector for rule or namespace discovery.

The following arguments are supported:

| Name           | Type          | Description                                       | Default | Required |
| -------------- | ------------- | ------------------------------------------------- | ------- | -------- |
| `match_labels` | `map(string)` | Label keys and values used to discover resources. | `{}`    | yes      |

When the `match_labels` argument is empty, all resources are matched.

### `match_expression`

The `match_expression` block describes a Kubernetes label match expression for rule or namespace discovery.

The following arguments are supported:

| Name       | Type           | Description                        | Default | Required |
| ---------- | -------------- | ---------------------------------- | ------- | -------- |
| `key`      | `string`       | The label name to match against.   |         | yes      |
| `operator` | `string`       | The operator to use when matching. |         | yes      |
| `values`   | `list(string)` | The values used when matching.     |         | no       |

The `operator` argument should be one of the following strings:

* `"In"`
* `"NotIn"`
* `"Exists"`
* `"DoesNotExist"`

The `values` argument must not be provided when `operator` is set to `"Exists"` or `"DoesNotExist"`.

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`mimir.rules.kubernetes` doesn't export any fields.

## Component health

`mimir.rules.kubernetes` is reported as unhealthy if given an invalid configuration or an error occurs during reconciliation.

## Debug information

`mimir.rules.kubernetes` exposes resource-level debug information.

The following are exposed per discovered `PrometheusRule` resource:

* The Kubernetes namespace.
* The resource name.
* The resource UID.
* The number of rule groups.

The following are exposed per discovered Mimir rule namespace resource:

* The namespace name.
* The number of rule groups.

Only resources managed by the component are exposed, regardless of how many actually exist.

## Debug metrics

| Metric Name                                         | Type        | Description                                                              |
| --------------------------------------------------- | ----------- | ------------------------------------------------------------------------ |
| `mimir_rules_mimir_client_request_duration_seconds` | `histogram` | Duration of requests to the Mimir API.                                   |
| `mimir_rules_config_updates_total`                  | `counter`   | Number of times the configuration has been updated.                      |
| `mimir_rules_events_failed_total`                   | `counter`   | Number of events that failed to be processed, partitioned by event type. |
| `mimir_rules_events_retried_total`                  | `counter`   | Number of events that were retried, partitioned by event type.           |
| `mimir_rules_events_total`                          | `counter`   | Number of events processed, partitioned by event type.                   |

## Examples

This example creates a `mimir.rules.kubernetes` component that loads discovered rules to a local Mimir instance under the `team-a` tenant.
Only namespaces and rules with the `alloy` label set to `yes` are included.

```alloy
mimir.rules.kubernetes "local" {
    address = "mimir:8080"
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

This example creates a `mimir.rules.kubernetes` component that loads discovered rules to Grafana Cloud.
It also adds a `"label1"` label to each rule. If that label already exists, it's overwritten with `"value1"`.

```alloy
mimir.rules.kubernetes "default" {
    address = ">GRAFANA_CLOUD_METRICS_URL>"
    basic_auth {
        username = "<GRAFANA_CLOUD_USER>"
        password = "<GRAFANA_CLOUD_API_KEY>"
        // Alternatively, load the password from a file:
        // password_file = "<GRAFANA_CLOUD_API_KEY_PATH>"
    }
    external_labels = {"label1" = "value1"}
}
```

This example adds label matcher `{cluster=~"prod-.*"}` to all the queries discovered by `mimir.rules.kubernetes`.

```alloy
mimir.rules.kubernetes "default" {
    address = "<GRAFANA_CLOUD_METRICS_URL>"
    extra_query_matchers {
        matcher {
            name = "cluster"
            match_type = "=~"
            value = "prod-.*"
        }
    }
}
```

If a query in the form of `up != 1` is found in `PrometheusRule` CRDs, it's' modified to `up{cluster=~"prod-.*"} != 1` before sending it to Mimir.

This example shows a `PrometheusRule` with a label set to `application.kubernetes.io/name=my-app`.

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
  metadata:
    labels:
      application.kubernetes.io/name: my-app
```

The following configuration extracts the value from the PrometheusRules CR label `application.kubernetes.io/name` and adds a label matcher with the value `{app=~"extracted_value"}` to all the queries discovered by the `mimir.rules.kubernetes` component.

```alloy
mimir.rules.kubernetes "default" {
    address = "<GRAFANA_CLOUD_METRICS_URL>"
    extra_query_matchers {
        matcher {
            name = "app"
            match_type = "="
            value_from_label = "application.kubernetes.io/name"
        }
    }
}
```

If a query in the form of `up != 1` is found in `PrometheusRule` CRDs, it's modified to `up{app="my-app"} != 1` before sending it to Mimir.

This example demonstrates tenant federation. Rules defined in the `PrometheusRule` CR are considered to originate from `source-tenant-alpha` and `source-tenant-beta`.
The `mimir.rules.kubernetes` component syncs these rules to the `target-tenant-main` in the configured Mimir instance.

```alloy
// config.alloy
mimir.rules.kubernetes "federated_rules" {
    address   = "mimir-ruler.example.com:8080"
    tenant_id = "target-tenant-main" // Target tenant in Mimir

    rule_selector {
        match_labels = {
            "federation-group" = "group1"
        }
    }
}
```

```yaml
# prometheus-rule-federated.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: federated-app-rules
  namespace: my-apps
  labels:
    federation-group: "group1"
  annotations:
    monitoring.grafana.com/source_tenants: "source-tenant-alpha,source-tenant-beta"
spec:
  groups:
  - name: my.app.rules
    rules:
    - alert: HighErrorRateFederated
      expr: job:request_latency_seconds:mean5m{job="my-federated-app"} > 0.5
      for: 10m
      labels:
        severity: critical
      annotations:
        summary: High request error rate for my-federated-app
```

The following example is an RBAC configuration for Kubernetes.
It authorizes {{< param "PRODUCT_NAME" >}} to query the Kubernetes REST API:

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
