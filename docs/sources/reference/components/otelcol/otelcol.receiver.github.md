---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.github/
description: Learn about otelcol.receiver.github
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.github
---

# `otelcol.receiver.github`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.github` collects metrics about repositories, pull requests, branches, and contributors.
You can also configure it to receive webhook events from GitHub Actions and convert these into traces.

You can specify multiple `otelcol.receiver.github` components by giving them different labels.

Ensure you have the following:

- A GitHub Personal Access Token or GitHub App with appropriate permissions
- For metrics: Read access to target repositories and organizations  
- For webhooks: A publicly accessible endpoint to receive GitHub webhook events
- Network connectivity to GitHub API endpoints, for example, `api.github.com` or your GitHub Enterprise instance

{{< admonition type="note" >}}
`otelcol.receiver.github` is a wrapper over the upstream OpenTelemetry Collector [`github`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`github`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/githubreceiver
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.github "<LABEL>" {
  scraper {
    github_org = "<GITHUB_ORG>"
    auth {
      authenticator = <AUTH_HANDLER>
    }
  }

  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.github`:

| Name                  | Type       | Description                                          | Default | Required |
| --------------------- | ---------- | ---------------------------------------------------- | ------- | -------- |
| `collection_interval` | `duration` | How often to scrape metrics from GitHub.             | `"30s"` | no       |
| `initial_delay`       | `duration` | Defines how long the receiver waits before starting. | `"0s"`  | no       |

## Blocks

You can use the following blocks with `otelcol.receiver.github`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`scraper`][scraper]             | Configures the GitHub scraper.                                             | no       |
| `scraper` > [`auth`][auth]       | Configures authentication for GitHub API.                                  | yes      |
| `scraper` > [`metrics`][metrics] | Configures which metrics to collect.                                       | no       |
| [`webhook`][webhook]             | Configures webhook receiver for GitHub Actions events.                     | no       |

The > symbol indicates deeper levels of nesting.
For example, `scraper` > `auth` refers to an `auth` block defined inside a `scraper` block.

[scraper]: #scraper
[webhook]: #webhook
[output]: #output
[debug_metrics]: #debug_metrics
[auth]: #auth
[metrics]: #metrics

### `scraper`

The `scraper` block configures the GitHub metrics scraper.

You can use the following arguments:

| Name           | Type     | Description                                          | Default | Required |
| -------------- | -------- | ---------------------------------------------------- | ------- | -------- |
| `github_org`   | `string` | GitHub organization or user to scrape metrics from.  |         | yes      |
| `endpoint`     | `string` | GitHub API endpoint for GitHub Enterprise instances. |         | no       |
| `search_query` | `string` | Custom search query to filter repositories.          |         | no       |

### `auth`

The `auth` block configures authentication for the GitHub API.

You can use the following arguments:

| Name            | Type                       | Description                                      | Default | Required |
| --------------- | -------------------------- | ------------------------------------------------ | ------- | -------- |
| `authenticator` | `capsule(otelcol.Handler)` | Auth handler from an `otelcol.auth.*` component. |         | yes      |

### `metrics`

The `metrics` block allows you to enable or disable specific metrics. Each metric is configured with a nested block.

Available metrics:

| Metric Name                   | Description                                    | Default |
| ----------------------------- | ---------------------------------------------- | ------- |
| `vcs.change.count`            | Number of changes or pull requests by state.   | `true`  |
| `vcs.change.duration`         | Time a change spent in various states.         | `true`  |
| `vcs.change.time_to_approval` | Time it took for a change to get approved.     | `true`  |
| `vcs.change.time_to_merge`    | Time it took for a change to be merged.        | `true`  |
| `vcs.contributor.count`       | Number of unique contributors to a repository. | `false` |
| `vcs.ref.count`               | Number of refs or branches in a repository.    | `true`  |
| `vcs.ref.lines_delta`         | Number of lines changed in a ref.              | `true`  |
| `vcs.ref.revisions_delta`     | Number of commits or revisions in a ref.       | `true`  |
| `vcs.ref.time`                | Time since the ref was created.                | `true`  |
| `vcs.repository.count`        | Number of repositories in an organization.     | `true`  |

Each metric can be configured with:

```alloy
metrics {
  vcs.change.count {
    enabled = true
  }
  vcs.contributor.count {
    enabled = false
  }
}
```

### `webhook`

The `webhook` block configures webhook reception for GitHub Actions events.

When enabled, this block allows the receiver to convert GitHub Actions workflow and job events into traces.

{{< admonition type="note" >}}
Secure your webhook endpoint with a secret and protect it with appropriate network security measures.
{{< /admonition >}}

You can use the following arguments:

| Name                  | Type          | Description                                                | Default            | Required |
| --------------------- | ------------- | ---------------------------------------------------------- | ------------------ | -------- |
| `endpoint`            | `string`      | The `host:port` to listen for webhooks.                    | `"localhost:8080"` | no       |
| `path`                | `string`      | URL path for webhook events.                               | `"/events"`        | no       |
| `health_path`         | `string`      | URL path for health checks.                                | `"/health"`        | no       |
| `include_span_events` | `bool`        | Whether to attach raw webhook event JSON as span events.   | `false`            | no       |
| `secret`              | `string`      | Secret for validating webhook signatures.                  |                    | no       |
| `service_name`        | `string`      | Service name for traces from this receiver.                |                    | no       |
| `required_headers`    | `map(string)` | Required headers that must be present on webhook requests. |                    | no       |

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.github` doesn't export any fields.

## Component health

`otelcol.receiver.github` is reported as unhealthy if the GitHub API can't be reached or authentication fails.

## Debug information

`otelcol.receiver.github` doesn't expose additional debug info.

## Example

### Basic configuration

This example scrapes metrics from a GitHub organization:

```alloy
otelcol.auth.bearer "github" {
  token = env("GITHUB_PAT")
}

otelcol.receiver.github "default" {
  collection_interval = "5m"
  
  scraper {
    github_org = "grafana"
    search_query = "org:grafana topic:observability"
    
    auth {
      authenticator = otelcol.auth.bearer.github.handler
    }
    
    metrics {
      vcs.change.count {
        enabled = true
      }
      vcs.change.time_to_merge {
        enabled = true
      }
      vcs.repository.count {
        enabled = true
      }
      vcs.contributor.count {
        enabled = true
      }
    }
  }
  
  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.prometheus "default" {
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "https://prometheus.example.com/api/v1/write"
  }
}
```

### With webhook for GitHub Actions traces

This example adds webhook support to receive GitHub Actions workflow events as traces:

```alloy
otelcol.auth.bearer "github" {
  token = env("GITHUB_PAT")
}

otelcol.receiver.github "default" {
  scraper {
    github_org = "my-org"
    auth {
      authenticator = otelcol.auth.bearer.github.handler
    }
  }
  
  webhook {
    endpoint = "0.0.0.0:8080"
    path = "/github/webhooks"
    health_path = "/health"
    secret = env("GITHUB_WEBHOOK_SECRET")
  }
  
  output {
    metrics = [otelcol.exporter.otlphttp.default.input]
    traces  = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = "https://otlp.example.com"
  }
}
```

### GitHub Enterprise

For GitHub Enterprise instances, specify the custom endpoint:

```alloy
otelcol.auth.bearer "github_enterprise" {
  token = env("GHE_PAT")
}

otelcol.receiver.github "enterprise" {
  scraper {
    github_org = "my-enterprise-org"
    endpoint = "https://github.mycompany.com"
    
    auth {
      authenticator = otelcol.auth.bearer.github_enterprise.handler
    }
  }
  
  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.github` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->