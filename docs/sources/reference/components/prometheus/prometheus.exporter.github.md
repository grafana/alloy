---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.github/
aliases:
  - ../prometheus.exporter.github/ # /docs/alloy/latest/reference/components/prometheus.exporter.github/
description: Learn about prometheus.exporter.github
title: prometheus.exporter.github
---

# prometheus.exporter.github

The `prometheus.exporter.github` component embeds
[github_exporter](https://github.com/githubexporter/github-exporter) for collecting statistics from GitHub.

## Usage

```alloy
prometheus.exporter.github "LABEL" {
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
All arguments are optional. Omitted fields take their default values.

| Name             | Type           | Description                                                      | Default                  | Required |
| ---------------- | -------------- | ---------------------------------------------------------------- | ------------------------ | -------- |
| `api_url`        | `string`       | The full URI of the GitHub API.                                  | `https://api.github.com` | no       |
| `repositories`   | `list(string)` | GitHub repositories for which to collect metrics.                |                          | no       |
| `organizations`  | `list(string)` | GitHub organizations for which to collect metrics.               |                          | no       |
| `users`          | `list(string)` | A list of GitHub users for which to collect metrics.             |                          | no       |
| `api_token`      | `secret`       | API token to use to authenticate against GitHub.                 |                          | no       |
| `api_token_file` | `string`       | File containing API token to use to authenticate against GitHub. |                          | no       |

GitHub uses an aggressive rate limit for unauthenticated requests based on IP address. To allow more API requests, it is recommended to configure either `api_token` or `api_token_file` to authenticate against GitHub.

When provided, `api_token_file` takes precedence over `api_token`.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.github` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.github` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.github` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.github`:

```alloy
prometheus.exporter.github "example" {
  api_token_file = "/etc/github-api-token"
  repositories   = ["grafana/alloy"]
}

// Configure a prometheus.scrape component to collect github metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.github.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.github` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
