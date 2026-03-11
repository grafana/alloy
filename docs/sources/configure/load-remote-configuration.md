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

Use `import.http` to load configuration from an HTTP server.
This is the recommended approach when you have a static configuration file hosted on a web server.
{{< param "PRODUCT_NAME" >}} periodically polls the URL to detect and apply configuration changes.

### Create the remote configuration file

Create a configuration file on your HTTP server.
The file must contain valid {{< param "PRODUCT_NAME" >}} configuration wrapped in a `declare` block.

The following example creates a reusable Prometheus scrape configuration.

prometheus_scrape.alloy (hosted on your HTTP server)

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

Use `import.http` in your local configuration to import the remote file.

config.alloy (local configuration)

```alloy
import.http "prometheus" {
  url            = "http://<CONFIG_SERVER_ADDRESS>/prometheus_scrape.alloy"
  poll_frequency = "5m"
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://<MIMIR_ADDRESS>/api/v1/push"
  }
}

prometheus.scrape "app" {
  targets    = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.remote_write.default.receiver]
}
```

Replace the following:

* _`<CONFIG_SERVER_ADDRESS>`_: The address of your HTTP server hosting the configuration file.
* _`<MIMIR_ADDRESS>`_: The address of your Prometheus-compatible remote write endpoint.

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

* _`<ORGANIZATION>`_: Your GitHub organization or username.
* _`<REPOSITORY>`_: The name of your Git repository.
* _`<MIMIR_ADDRESS>`_: The address of your Prometheus-compatible remote write endpoint.

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
