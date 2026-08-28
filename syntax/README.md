# Alloy configuration syntax

<p>
  <a href="https://pkg.go.dev/github.com/grafana/alloy/syntax"><img src="https://pkg.go.dev/badge/github.com/grafana/alloy.svg" alt="API"></a>
  <a href="https://grafana.com/docs/alloy/latest/concepts/configuration-syntax"><img src="https://img.shields.io/badge/Documentation-link-blue?logo=gitbook" alt="User documentation"></a>
</p>

The Alloy configuration syntax is a domain-specific language used by Grafana
Alloy to define pipelines.

The syntax was designed with the following goals:

* _Fast_: The syntax must be fast so the component controller can quickly evaluate changes.
* _Simple_: The syntax must be easy to read and write to minimize the learning curve.
* _Debuggable_: The syntax must give detailed information when there's a mistake in the configuration file.

The syntax package is importable as a Go module so other projects can use
it.

> **NOTE**: The `syntax` submodule is versioned separately from the main
> module, and does not have a stable API.

## Example

```grafana-alloy
// Discover Kubernetes pods to collect metrics from.
discovery.kubernetes "pods" {
  role = "pod"
}

// Collect metrics from Kubernetes pods.
prometheus.scrape "default" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [prometheus.remote_write.default.receiver]
}

// Get an API key from disk.
local.file "apikey" {
  filename  = "/var/data/my-api-key.txt"
  is_secret = true
}

// Send metrics to a Prometheus remote_write endpoint.
prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9009/api/prom/push"

    basic_auth {
      username = "MY_USERNAME"
      password = local.file.apikey.content
    }
  }
}
```

## Limitations

The `syntax` submodule only contains lower level concepts (attributes, blocks,
expressions). It does not contain any higher level concepts like components or
services.
