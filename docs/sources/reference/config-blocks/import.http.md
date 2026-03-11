---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/import.http/
description: Learn about the import.http configuration block
labels:
  stage: general-availability
  products:
    - oss
title: import.http
---

# `import.http`

`import.http` retrieves a module from an HTTP server.

Use `import.http` to load {{< param "PRODUCT_NAME" >}} configuration from a remote HTTP server.
The remote file must define configuration inside a `declare` block.
{{< param "PRODUCT_NAME" >}} evaluates imported modules as reusable components, so the remote file must not include top-level global configuration blocks such as `logging`, `remotecfg`, or CLI settings.
Global configuration belongs in the local configuration file that imports the module.
{{< param "PRODUCT_NAME" >}} periodically polls the URL to detect and apply configuration changes.

Refer to [Load configuration from remote sources][load-remote] for more information.

## Usage

```alloy
import.http "<LABEL>" {
  url = <URL>
}
```

## Arguments

You can use the following arguments with `import.http`:

| Name             | Type          | Description                     | Default | Required |
| ---------------- | ------------- | ------------------------------- | ------- | -------- |
| `url`            | `string`      | URL to poll.                    |         | yes      |
| `headers`        | `map(string)` | Custom headers for the request. | `{}`    | no       |
| `method`         | `string`      | HTTP method for the request.    | `"GET"` | no       |
| `poll_frequency` | `duration`    | Frequency to poll the URL.      | `"1m"`  | no       |
| `poll_timeout`   | `duration`    | Timeout when polling the URL.   | `"10s"` | no       |

## Blocks

You can use the following blocks with `import.http`:

| Block                                            | Description                                                | Required |
| ------------------------------------------------ | ---------------------------------------------------------- | -------- |
| [`client`][client]                               | HTTP client settings when connecting to the endpoint.      | no       |
| `client` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| `client` > [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| `client` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `client` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| `client` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `basic_auth` refers to a `basic_auth` block defined inside a `client` block.

### `client`

The `client` block configures settings for connecting to the HTTP server.

{{< docs/shared lookup="reference/components/http-client-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authorization`

The `authorization` block configures custom authorization for polling the configured URL.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

The `basic_auth` block configures basic authentication for polling the configured URL.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

The `oauth2` block configures OAuth 2.0 authorization for polling the configured URL.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS settings for connecting to HTTPS servers.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Behavior when the remote server is unavailable

If {{< param "PRODUCT_NAME" >}} can't reach the configured URL, it continues running with the previously loaded configuration.

{{< param "PRODUCT_NAME" >}} retries fetching the remote module at the next `poll_frequency` interval.

{{< param "PRODUCT_NAME" >}} writes errors retrieving the configuration to the logs, but these errors don't stop the current configuration from operating.

## Monitor configuration fetch failures

To detect configuration update problems early, monitor for repeated fetch failures:

- Check collector logs for repeated HTTP errors when retrieving remote modules.
- Investigate persistent `4xx` errors, which indicate authentication or URL configuration issues.
- Investigate persistent `5xx` errors, which indicate remote server problems.
- Verify network connectivity and proxy configuration if requests time out.

If configuration updates are critical for your deployment, consider adding alerting based on log monitoring or collector health checks.

## Example

This example imports custom components from an HTTP response and instantiates a custom component for adding two numbers.

Create a module file and host it on your HTTP server:

```alloy
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

In your local configuration file, import the remote module and use the declared component:

```alloy
import.http "math" {
  url = <SERVER_URL>
}

math.add "default" {
  a = 15
  b = 45
}
```

### Load configuration from a remote HTTP server

You can use `import.http` to load an {{< param "PRODUCT_NAME" >}} configuration containing standard components from a remote HTTP server.

The following example shows how to load a Prometheus scrape configuration from a remote server.

Create a module file and host it on your HTTP server:

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

In your local configuration file, import the remote module and use the declared component:

```alloy
import.http "remote" {
  url            = "http://config-server.example.com/prometheus_scrape.alloy"
  poll_frequency = "5m"
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}

remote.scrape "app" {
  targets    = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.remote_write.default.receiver]
}
```

[load-remote]: ../../configure/load-remote-configuration/
[client]: #client
[basic_auth]: #basic_auth
[authorization]: #authorization
[oauth2]: #oauth2
[tls_config]: #tls_config
