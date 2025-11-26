---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/build-pipelines/
description: Learn how to build data pipelines with components
title: Build data pipelines
weight: 60
---

# Build data pipelines

{{< param "PRODUCT_NAME" >}} components work together to create _pipelines_ that collect, transform, and send telemetry data.
By connecting components through their exports and arguments, you can build powerful data processing workflows that automatically respond to changes and handle complex data transformations.

Before you begin, make sure you understand:

- [Basic component configuration](../configure-components/)
- [Component arguments and exports](../configure-components/#arguments-and-exports)
- [Component references](../configure-components/#component-references)

## What are pipelines?

A pipeline forms when components reference each other's exports.
Most arguments in components are constant values, but when you use expressions that reference component exports, you create dependencies between components.

```alloy
// Simple constant value
log_level = "debug"

// Expression that creates a dependency
api_key = local.file.secret.content
```

When a component's argument references another component's export, {{< param "PRODUCT_NAME" >}} re-evaluates the dependent component whenever the referenced component updates its exports.

## Your first pipeline

This simple pipeline reads a password from a file and uses it to authenticate with a remote system:

```alloy
local.file "api_key" {
    filename = "/etc/secrets/api.key"
}

prometheus.remote_write "production" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"

        basic_auth {
            username = "admin"
            password = local.file.api_key.content
        }
    }
}
```

This pipeline has two components:

1. `local.file` reads a file and exports its content
2. `prometheus.remote_write` uses that content as a password

When the file changes, {{< param "PRODUCT_NAME" >}} automatically updates the password used by the remote write component.

{{< figure src="/media/docs/alloy/diagram-example-basic-alloy.png" width="600" alt="Example pipeline with local.file and prometheus.remote_write components" >}}

## Multi-stage pipelines

You can chain multiple components together to create more complex pipelines.
This example shows a complete metrics collection pipeline:

```alloy
// Discover Kubernetes pods to scrape
discovery.kubernetes "pods" {
  role = "pod"
}

// Scrape metrics from the discovered pods
prometheus.scrape "app_metrics" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [prometheus.remote_write.production.receiver]
}

// Send metrics to remote storage
prometheus.remote_write "production" {
  endpoint {
    url = "https://prometheus.example.com/api/v1/write"

    basic_auth {
      username = "metrics"
      password = local.file.api_key.content
    }
  }
}

// Read API key from file
local.file "api_key" {
  filename = "/etc/secrets/api-key"
  is_secret = true
}
```

{{< figure src="/media/docs/alloy/diagram-concepts-example-pipeline.png" width="600" alt="Example of a complete metrics pipeline" >}}

This pipeline demonstrates several key concepts:

1. **Service discovery**: `discovery.kubernetes` finds targets to monitor
2. **Data collection**: `prometheus.scrape` collects metrics from those targets
3. **Data forwarding**: The scraper forwards metrics to the remote write component
4. **Authentication**: The remote write component uses credentials from a file

## Log processing pipeline

Here's a more complex example that processes log data through multiple transformation stages:

```alloy
// Find log files to read
local.file_match "app_logs" {
    path_targets = [{"__path__" = "/var/log/app/*.log"}]
}

// Read the log files
loki.source.file "local_files" {
    targets    = local.file_match.app_logs.targets
    forward_to = [loki.process.add_labels.receiver]
}

// Extract data from log messages and add labels
loki.process "add_labels" {
    stage.logfmt {
        mapping = {
            "extracted_level" = "level",
            "extracted_service" = "service",
        }
    }

    stage.labels {
        values = {
            "level" = "extracted_level",
            "service" = "extracted_service",
        }
    }

    forward_to = [loki.write.grafana_cloud.receiver]
}

// Send processed logs to Loki
loki.write "grafana_cloud" {
    endpoint {
        url = "https://logs-prod.grafana.net/loki/api/v1/push"

        basic_auth {
            username = "12345"
            password = local.file.api_key.content
        }
    }
}

// Read API credentials
local.file "api_key" {
    filename = "/etc/secrets/loki-key"
    is_secret = true
}
```

This pipeline shows how data flows through multiple processing stages:

1. **Discovery**: Find log files to monitor
2. **Collection**: Read log entries from files
3. **Transformation**: Parse log messages and extract metadata
4. **Enrichment**: Add structured labels to log entries
5. **Output**: Send processed logs to remote storage

## Pipeline patterns

### Fan-out pattern

Send data from one component to multiple destinations:

```alloy
prometheus.scrape "app_metrics" {
  targets = [{"__address__" = "app:8080"}]
  forward_to = [
    prometheus.remote_write.production.receiver,
    prometheus.remote_write.staging.receiver,
  ]
}

prometheus.remote_write "production" {
  endpoint {
    url = "https://prod-prometheus.example.com/api/v1/write"
  }
}

prometheus.remote_write "staging" {
  endpoint {
    url = "https://staging-prometheus.example.com/api/v1/write"
  }
}
```

### Processing chain

Transform data through multiple stages:

```alloy
loki.source.file "raw_logs" {
  targets = [{"__path__" = "/var/log/app.log"}]
  forward_to = [loki.process.parse.receiver]
}

loki.process "parse" {
  stage.json {
    expressions = {
      level = "level",
      message = "msg",
    }
  }
  forward_to = [loki.process.filter.receiver]
}

loki.process "filter" {
  stage.match {
    selector = "{level=\"error\"}"
    action   = "keep"
  }
  forward_to = [loki.write.alerts.receiver]
}

loki.write "alerts" {
  endpoint {
    url = "https://loki.example.com/loki/api/v1/push"
  }
}
```

## Best practices

### Keep pipelines simple

Break complex pipelines into logical stages.
Each component should have a clear, single responsibility.

### Use descriptive labels

Choose component labels that describe their purpose:

```alloy
// Good: Descriptive labels
prometheus.scrape "api_metrics" { }
prometheus.scrape "database_metrics" { }

// Avoid: Generic labels
prometheus.scrape "scraper1" { }
prometheus.scrape "scraper2" { }
```

### Handle secrets securely

Mark sensitive components appropriately:

```alloy
local.file "database_password" {
  filename = "/etc/secrets/db-password"
  is_secret = true  // Prevents value from appearing in UI
}
```

### Test incrementally

Build pipelines step by step.
Start with basic data collection, then add processing and forwarding components.

## Debugging pipelines

When pipelines don't work as expected:

1. **Check component health** in the {{< param "PRODUCT_NAME" >}} UI
2. **Verify component exports** contain expected data
3. **Review component dependencies** to ensure proper data flow
4. **Check for cycles** - components can't reference themselves directly or indirectly

## Next steps

Now that you understand how to build pipelines:

- [Component controller][] - Learn how {{< param "PRODUCT_NAME" >}} manages component execution
- [Expressions][] - Create dynamic configurations using functions and references
- [Tutorials][] - Follow step-by-step guides to build complete monitoring solutions

[Component controller]: ./component-controller/
[Expressions]: ../expressions/
[Tutorials]: ../../tutorials/
