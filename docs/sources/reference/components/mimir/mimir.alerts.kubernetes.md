---
canonical: https://grafana.com/docs/alloy/latest/reference/components/mimir/mimir.alerts.kubernetes/
aliases:
  - ../mimir.alerts.kubernetes/ # /docs/alloy/latest/reference/components/mimir.alerts.kubernetes/
description: Learn about mimir.alerts.kubernetes
labels:
  stage: experimental
  products:
    - oss
title: mimir.alerts.kubernetes
---

# `mimir.alerts.kubernetes`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`mimir.alerts.kubernetes` discovers `AlertmanagerConfig` Kubernetes resources and loads them into a Mimir instance.

* You can specify multiple `mimir.alerts.kubernetes` components by giving them different labels.
* You can use [Kubernetes label selectors][] to limit the `Namespace` and `AlertmanagerConfig` resources considered during reconciliation.
* Compatible with the Alertmanager APIs of Grafana Mimir, Grafana Cloud, and Grafana Enterprise Metrics.
* Compatible with the `AlertmanagerConfig` CRD from the [`prometheus-operator`][prometheus-operator].
* This component accesses the Kubernetes REST API from [within a Pod][].

{{< admonition type="note" >}}
This component requires [Role-based access control (RBAC)][] to be set up in Kubernetes in order for {{< param "PRODUCT_NAME" >}} to access it via the Kubernetes REST API.

[Role-based access control (RBAC)]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
{{< /admonition >}}

`mimir.alerts.kubernetes` doesn't support [clustering][clustered mode].

[clustered mode]: ../../../../get-started/clustering/
[Kubernetes label selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
[prometheus-operator]: https://prometheus-operator.dev/
[within a Pod]: https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/

## Usage

```alloy
mimir.alerts.kubernetes "<LABEL>" {
  address       = "<MIMIR_URL>"
  global_config = "..."
}
```

## Arguments

You can use the following arguments with `mimir.alerts.kubernetes`:

| Name                                  | Type                | Description                                                                                      | Default         | Required |
| ------------------------------------- | ------------------- | ------------------------------------------------------------------------------------------------ | --------------- | -------- |
| `address`                             | `string`            | URL of the Mimir Alertmanager.                                                                   |                 | yes      |
| `global_config`                       | `secret`            | [Alertmanager configuration][global-cfg] to be merged with AlertmanagerConfig CRDs.              |                 | yes      |
| `bearer_token_file`                   | `string`            | File containing a bearer token to authenticate with.                                             |                 | no       |
| `bearer_token`                        | `secret`            | Bearer token to authenticate with.                                                               |                 | no       |
| `enable_http2`                        | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`          | no       |
| `follow_redirects`                    | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`          | no       |
| `http_headers`                        | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |                 | no       |
| `no_proxy`                            | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                 | no       |
| `proxy_connect_header`                | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                 | no       |
| `proxy_from_environment`              | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`         | no       |
| `proxy_url`                           | `string`            | HTTP proxy to send requests through.                                                             |                 | no       |
| `sync_interval`                       | `duration`          | Amount of time between reconciliations with Mimir.                                               | `"5m"`          | no       |
| `template_files`                      | `map(string)`       | A map of Alertmanager [template files][mimir-api-set-alertmgr-cfg].                              |  `{}`           | no       |

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments]argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

 [arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

[global-cfg]: https://prometheus.io/docs/alerting/latest/configuration/
[mimir-api-set-alertmgr-cfg]: https://grafana.com/docs/mimir/latest/references/http-api/#set-alertmanager-configuration

## Blocks

The following blocks are supported inside the definition of
`mimir.alerts.kubernetes`:

| Block                                                                            | Description                                                 | Required |
| -------------------------------------------------------------------------------- | ----------------------------------------------------------- | -------- |
| [`authorization`][authorization]                                                 | Configure generic authorization to the endpoint.            | no       |
| [`basic_auth`][basic_auth]                                                       | Configure `basic_auth` for authenticating to the endpoint.  | no       |
| [`oauth2`][oauth2]                                                               | Configure OAuth 2.0 for authenticating to the endpoint.     | no       |
| `oauth2` > [`tls_config`][tls_config]                                            | Configure TLS settings for connecting to the endpoint.      | no       |
| [`alertmanagerconfig_namespace_selector`][label_selector]                        | Label selector for `Namespace` resources.                   | no       |
| `alertmanagerconfig_namespace_selector` > [`match_expression`][match_expression] | Label match expression for `Namespace` resources.           | no       |
| [`alertmanagerconfig_selector`][label_selector]                                  | Label selector for `AlertmanagerConfig` resources.          | no       |
| `alertmanagerconfig_selector` > [`match_expression`][match_expression]           | Label match expression for `AlertmanagerConfig` resources.  | no       |
| `alertmanagerconfig_matcher`                                                     | Strategy to match alerts to `AlertmanagerConfig` resources. | no       |
| [`tls_config`][tls_config]                                                       | Configure TLS settings for connecting to the endpoint.      | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[authorization]: #authorization
[basic_auth]: #basic_auth
[label_selector]: #alertmanagerconfig_selector-and-alertmanagerconfig_namespace_selector
[match_expression]: #match_expression
[oauth2]: #oauth2
[tls_config]: #tls_config

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `alertmanagerconfig_selector` and `alertmanagerconfig_namespace_selector`

The `alertmanagerconfig_selector` and `alertmanagerconfig_namespace_selector` blocks describe a Kubernetes label selector for AlertmanagerConfig CRDs or namespace discovery.

The following arguments are supported:

| Name           | Type          | Description                                       | Default | Required |
| -------------- | ------------- | ------------------------------------------------- | ------- | -------- |
| `match_labels` | `map(string)` | Label keys and values used to discover resources. | `{}`    | yes      |

When the `match_labels` argument is empty, all resources are matched.

### `match_expression`

The `match_expression` block describes a Kubernetes label match expression for AlertmanagerConfig CRDs or namespace discovery.

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

### `alertmanagerconfig_matcher`

The `alertmanagerconfig_matcher` block describes the strategy used by AlertmanagerConfig objects to match alerts in the routes and inhibition rules.

Depending on how this block is configured, the final Alertmanger config will have different [matchers][] in its [route][] section.

[matchers]: https://prometheus.io/docs/alerting/latest/configuration/#matcher
[route]: https://prometheus.io/docs/alerting/latest/configuration/#route

The following arguments are supported:

| Name                     | Type     | Description                                                                                                          | Default         | Required                                                              |
| ------------------------ | -------- | -------------------------------------------------------------------------------------------------------------------- | --------------- | --------------------------------------------------------------------- |
| `strategy`               | `string` | Strategy for adding matchers to AlertmanagerConfig CRDs.                                                             | `"OnNamespace"` | no                                                                    |
| `alertmanager_namespace` | `string` | Namespace to use when `alertmanagerconfig_matcher_strategy` is set to `"OnNamespaceExceptForAlertmanagerNamespace"`. |                 | only when `strategy` is `"OnNamespaceExceptForAlertmanagerNamespace"` |

The `strategy` argument should be one of the following strings:

* `"OnNamespace"`: Each AlertmanagerConfig object only matches alerts that have the `namespace` label set to the same namespace as the AlertmanagerConfig object.
* `"OnNamespaceExceptForAlertmanagerNamespace"`: The same as `"OnNamespace"`, except for AlertmanagerConfigs in the namespace given by `alertmanager_namespace`, which apply to all alerts.
* `"None"`: Every AlertmanagerConfig object applies to all alerts

`strategy` is similar to the [AlertmanagerConfigMatcherStrategy][alertmanager-config-matcher-strategy] in Prometheus Operator, but it is configured in {{< param "PRODUCT_NAME" >}} instead of in an Alertmanager CRD.
{{< param "PRODUCT_NAME" >}} doesn't require an Alertmanager CRD.

[alertmanager-config-matcher-strategy]: https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.AlertmanagerConfigMatcherStrategy

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`mimir.alerts.kubernetes` doesn't export any fields.

## Component health

`mimir.alerts.kubernetes` is reported as unhealthy if given an invalid configuration or an error occurs during reconciliation.

## Debug information

`mimir.alerts.kubernetes` doesn't expose debug information.

## Debug metrics

| Metric Name                                          | Type        | Description                                                              |
| ---------------------------------------------------- | ----------- | ------------------------------------------------------------------------ |
| `mimir_alerts_mimir_client_request_duration_seconds` | `histogram` | Duration of requests to the Mimir API.                                   |
| `mimir_alerts_config_updates_total`                  | `counter`   | Number of times the configuration has been updated.                      |
| `mimir_alerts_events_failed_total`                   | `counter`   | Number of events that failed to be processed, partitioned by event type. |
| `mimir_alerts_events_retried_total`                  | `counter`   | Number of events that were retried, partitioned by event type.           |
| `mimir_alerts_events_total`                          | `counter`   | Number of events processed, partitioned by event type.                   |

## Example

This example creates a `mimir.alerts.kubernetes` component which only loads namespace and `AlertmanagerConfig` resources if they contain an `alloy` label set to `yes`.

```alloy
remote.kubernetes.configmap "default" {
  namespace = "default"
  name      = "alertmgr-global"
}

mimir.alerts.kubernetes "default" {
  address       = "http://mimir-nginx.mimir-test.svc:80"
  global_config = remote.kubernetes.configmap.default.data["glbl"]

  template_files = {
    `default_template` = 
`{{ define "__alertmanager" }}AlertManager{{ end }}
{{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}`,
  }

  alertmanagerconfig_selector {
      match_labels = {
          alloy = "yes",
      }
  }

  alertmanagerconfig_namespace_selector {
      match_labels = {
          alloy = "yes",
      }
  }
}
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
  resources: ["alertmanagerconfigs"]
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

The following is an example of a complete Kubernetes configuration:

{{< collapse title="Example Kubernetes configuration." >}}

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: testing
  labels:
    alloy: "yes"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: grafana-alloy
  namespace: testing
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: grafana-alloy
rules:
- apiGroups: [""]
  resources: ["namespaces", "configmaps"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["monitoring.coreos.com"]
  resources: ["alertmanagerconfigs"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: grafana-alloy
subjects:
- kind: ServiceAccount
  name: grafana-alloy
  namespace: testing
roleRef:
  kind: ClusterRole
  name: grafana-alloy
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: Service
metadata:
  name: grafana-alloy
spec:
  type: NodePort
  selector:
    app: grafana-alloy
  ports:
      # By default and for convenience, the `targetPort` is set to the same value as the `port` field.
    - port: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: testing
  name: grafana-alloy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grafana-alloy
  template:
    metadata:
      labels:
        app: grafana-alloy
    spec:
      serviceAccount: grafana-alloy
      containers:
      - name: alloy
        image: grafana/alloy:latest
        imagePullPolicy: Never
        args:
        - run
        - /etc/config/config.alloy
        - --stability.level=experimental
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config-volume
          mountPath: /etc/config
      volumes:
        - name: config-volume
          configMap:
            name: alloy-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: alloy-config
  namespace: testing
data:
  config.alloy: |
      remote.kubernetes.configmap "default" {
        namespace = "testing"
        name = "alertmgr-global"
      }

      mimir.alerts.kubernetes "default" {
        address = "http://mimir-nginx.mimir-test.svc:80"
        global_config = remote.kubernetes.configmap.default.data["glbl"]
        template_files = {
          `default_template` = 
      `{{ define "__alertmanager" }}AlertManager{{ end }}
      {{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}`,
        }
        alertmanagerconfig_namespace_selector {
            match_labels = {
                alloy = "yes",
            }
        }
        alertmanagerconfig_selector {
            match_labels = {
                alloy = "yes",
            }
        }
      }
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmgr-global
  namespace: testing
data:
  glbl: |
    global:
      resolve_timeout: 5m
      http_config:
        follow_redirects: true
        enable_http2: true
      smtp_hello: localhost
      smtp_require_tls: true
    route:
      receiver: "null"
    receivers:
    - name: "null"
    - name: "alloy-namespace/global-config/myreceiver"
    templates:
    - 'default_template'
---
apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: alertmgr-config1
  namespace: testing
  labels:
    alloy: "yes"
spec:
  route:
    receiver: "null"
    routes:
    - receiver: myamc
      continue: true
  receivers:
  - name: "null"
  - name: myamc
    webhookConfigs:
    - url: http://test.url
      httpConfig:
        followRedirects: true
---
apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: alertmgr-config2
  namespace: testing
  labels:
    alloy: "yes"
spec:
  route:
    receiver: "null"
    routes:
    - receiver: 'database-pager'
      groupWait: 10s
      matchers:
      - name: service
        value: webapp
  receivers:
  - name: "null"
  - name: "database-pager"
```

{{< /collapse >}}

The Kubernetes configuration above creates the Alertmanager configuration below and sends it to Mimir:

{{< collapse title="Example merged configuration sent to Mimir." >}}

```yaml
template_files:
    default_template: |-
        {{ define "__alertmanager" }}AlertManager{{ end }}
        {{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}
alertmanager_config: |
    global:
      resolve_timeout: 5m
      http_config:
        follow_redirects: true
        enable_http2: true
      smtp_hello: localhost
      smtp_require_tls: true
    route:
      receiver: "null"
      continue: false
      routes:
      - receiver: testing/alertmgr-config1/null
        matchers:
        - namespace="testing"
        continue: true
        routes:
        - receiver: testing/alertmgr-config1/myamc
          continue: true
      - receiver: testing/alertmgr-config2/null
        matchers:
        - namespace="testing"
        continue: true
        routes:
        - receiver: testing/alertmgr-config2/database-pager
          matchers:
          - service="webapp"
          continue: false
          group_wait: 10s
    receivers:
    - name: "null"
    - name: alloy-namespace/global-config/myreceiver
    - name: testing/alertmgr-config1/null
    - name: testing/alertmgr-config1/myamc
      webhook_configs:
      - send_resolved: false
        http_config:
          follow_redirects: true
          enable_http2: true
        url: <secret>
        url_file: ""
        max_alerts: 0
        timeout: 0s
    - name: testing/alertmgr-config2/null
    - name: testing/alertmgr-config2/database-pager
    templates:
    - default_template
```

{{< /collapse >}}

You can add the `alertmanagerconfig_matcher` block to your {{< param "PRODUCT_NAME" >}} configuration to remove the namespace matchers:

```alloy
alertmanagerconfig_matcher {
  strategy = "None"
}
```

This results in the following final configuration:

{{< collapse title="Example merged configuration sent to Mimir." >}}

```yaml
template_files:
    default_template: |-
        {{ define "__alertmanager" }}AlertManager{{ end }}
        {{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}
alertmanager_config: |
    global:
      resolve_timeout: 5m
      http_config:
        follow_redirects: true
        enable_http2: true
      smtp_hello: localhost
      smtp_require_tls: true
    route:
      receiver: "null"
      continue: false
      routes:
      - receiver: testing/alertmgr-config1/null
        continue: true
        routes:
        - receiver: testing/alertmgr-config1/myamc
          continue: true
      - receiver: testing/alertmgr-config2/null
        continue: true
        routes:
        - receiver: testing/alertmgr-config2/database-pager
          matchers:
          - service="webapp"
          continue: false
          group_wait: 10s
    receivers:
    - name: "null"
    - name: alloy-namespace/global-config/myreceiver
    - name: testing/alertmgr-config1/null
    - name: testing/alertmgr-config1/myamc
      webhook_configs:
      - send_resolved: false
        http_config:
          follow_redirects: true
          enable_http2: true
        url: <secret>
        url_file: ""
        max_alerts: 0
        timeout: 0s
    - name: testing/alertmgr-config2/null
    - name: testing/alertmgr-config2/database-pager
    templates:
    - default_template
```

{{< /collapse >}}
