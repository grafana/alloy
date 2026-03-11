---
canonical: https://grafana.com/docs/alloy/latest/configure/load-remote-configuration/
description: Learn how to load Grafana Alloy configuration from remote sources
menuTitle: Load remote configuration
title: Load configuration from remote sources
weight: 700
---

# Load configuration from remote sources

{{< param "PRODUCT_NAME" >}} provides several methods to load configuration from remote sources.
The method you choose depends on your use case and infrastructure.

## Before you begin

You should have a basic understanding of [{{< param "PRODUCT_NAME" >}} configuration syntax][syntax] and [modules][modules].

[syntax]: ../../get-started/syntax/
[modules]: ../../get-started/modules/

## Choose a method

Use the following table to choose the appropriate method for your use case:

| Method                       | Use case                                                                              |
| ---------------------------- | ------------------------------------------------------------------------------------- |
| [`import.http`][import.http] | Load configuration from an HTTP server hosting static files.                          |
| [`import.git`][import.git]   | Load configuration from a Git repository with version control.                        |
| [`import.file`][import.file] | Load configuration from a local file or directory.                                    |
| [`remotecfg`][remotecfg]     | Dynamically manage configuration through a remote configuration management API server.|

[import.http]: ../../reference/config-blocks/import.http/
[import.git]: ../../reference/config-blocks/import.git/
[import.file]: ../../reference/config-blocks/import.file/
[remotecfg]: ../../reference/config-blocks/remotecfg/

## Load from an HTTP server

Use `import.http` to load configuration modules from an HTTP server.
This is the recommended approach when you have configuration modules hosted on a web server.

{{< param "PRODUCT_NAME" >}} treats remote files loaded with `import.http` as modules.
Modules define reusable components using `declare` blocks, which the local configuration then instantiates.
After you import a module, you must instantiate its declared components in the local configuration for them to run.

{{< admonition type="note" >}}
You can't point {{< param "PRODUCT_NAME" >}} directly at a remote URL on startup.
You must have a local configuration file that uses `import.http` to import modules from the remote server.
The remote file must define reusable components using `declare` blocks.
{{< /admonition >}}

{{< param "PRODUCT_NAME" >}} periodically polls the URL to detect and apply configuration changes.

### Module requirements

Modules must define reusable components using `declare` blocks and can't contain top-level configuration blocks such as `logging` or `remotecfg`.
CLI flags (command-line settings) are configured when starting {{< param "PRODUCT_NAME" >}}, not within modules.

The following module is invalid because it contains a `logging` block, which isn't allowed in modules:

```alloy
logging {
  level = "debug"
}

declare "pipeline" {
  prometheus.scrape "default" {
    targets = [{"__address__" = "localhost:9090"}]
  }
}
```

The following module is valid because it only contains `declare` blocks:

```alloy
declare "pipeline" {
  prometheus.scrape "default" {
    targets = [{"__address__" = "localhost:9090"}]
  }
}
```

Global configuration such as `logging` must remain in the local configuration file that imports the module.

### Create the remote configuration file

Create a configuration file on your HTTP server.
The file must contain valid {{< param "PRODUCT_NAME" >}} configuration wrapped in a `declare` block.

The following example creates a reusable Prometheus scrape configuration:

```alloy
declare "scrape" {
  argument "targets" {}
  argument "forward_to" {}

  prometheus.scrape "default" {
    targets    = argument.targets.value
    forward_to = argument.forward_to.value
  }
}
```

### Import the remote configuration

In your local configuration, import the remote file and use the declared component:

```alloy
import.http "remote" {
  url            = "http://<CONFIG_SERVER_ADDRESS>/prometheus_scrape.alloy"
  poll_frequency = "5m"
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://<MIMIR_ADDRESS>/api/v1/push"
  }
}

remote.scrape "app" {
  targets    = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.remote_write.default.receiver]
}
```

Replace the following:

- _`<CONFIG_SERVER_ADDRESS>`_: The address of your HTTP server hosting the configuration file.
- _`<MIMIR_ADDRESS>`_: The address of your Prometheus-compatible remote write endpoint.

You can load multiple remote modules by defining multiple `import.http` blocks in your local configuration file.
Each block can point to a different module file on the remote server.

For example:

```alloy
import.http "metrics" {
  url = "http://config-server.example.com/metrics.alloy"
}

import.http "logs" {
  url = "http://config-server.example.com/logs.alloy"
}
```

Each imported module can define its own `declare` blocks and components.

If you need to manage many configuration files or directories, consider using [`import.git`][import.git] to load modules from a version-controlled repository.

Refer to [`import.http`][import.http] for more information.

## Load from a Git repository

Use `import.git` to load configuration from a Git repository.
This approach provides version control and supports authentication for private repositories.
{{< param "PRODUCT_NAME" >}} periodically pulls the repository to detect and apply configuration changes.

```alloy
import.git "modules" {
  repository     = "https://github.com/<ORGANIZATION>/<REPOSITORY>.git"
  revision       = "main"
  path           = "modules/prometheus.alloy"
  pull_frequency = "5m"
}

modules.scrape "app" {
  targets    = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://<MIMIR_ADDRESS>/api/v1/push"
  }
}
```

Replace the following:

- _`<ORGANIZATION>`_: Your GitHub organization or username.
- _`<REPOSITORY>`_: The name of your Git repository.
- _`<MIMIR_ADDRESS>`_: The address of your Prometheus-compatible remote write endpoint.

Refer to [`import.git`][import.git] for more information.

## Load from a local file

Use `import.file` to load configuration from a local file or directory.
This is useful when you want to organize your configuration into separate files on the local filesystem.
{{< param "PRODUCT_NAME" >}} watches the file for changes and automatically applies updates.

```alloy
import.file "modules" {
  filename = "/etc/alloy/modules/prometheus.alloy"
}

modules.scrape "app" {
  targets    = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://<MIMIR_ADDRESS>/api/v1/push"
  }
}
```

Replace _`<MIMIR_ADDRESS>`_ with the address of your Prometheus-compatible remote write endpoint.

Refer to [`import.file`][import.file] for more information.

## Use dynamic remote configuration management

Use `remotecfg` when you need a configuration management server that can serve different configurations based on collector identity and attributes.
This approach requires implementing or using a server that supports the [alloy-remote-config API][api-definition].

{{< admonition type="note" >}}
The `remotecfg` block isn't for loading static configuration files from an HTTP server.
If you want to load a static configuration file from an HTTP server, use `import.http` instead.
{{< /admonition >}}

The `remotecfg` block sends the collector's `id` and `attributes` to the server, allowing the server to dynamically decide which configuration to serve.

```alloy
remotecfg {
  url            = "http://<REMOTECFG_SERVER_ADDRESS>"
  id             = constants.hostname
  attributes     = {"cluster" = "production", "environment" = "us-east-1"}
  poll_frequency = "5m"
}

logging {
  level  = "info"
  format = "logfmt"
}
```

Replace _`<REMOTECFG_SERVER_ADDRESS>`_ with the address of your remote configuration management server.

Refer to [`remotecfg`][remotecfg] for more information.

[api-definition]: https://github.com/grafana/alloy-remote-config
