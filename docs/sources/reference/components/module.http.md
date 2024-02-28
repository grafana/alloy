---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/module.http/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/module.http/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/module.http/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/module.http/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/module.http/
description: Learn about module.http
labels:
  stage: beta
title: module.http
---

# module.http

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

`module.http` is a [module loader][] component.

`module.http` embeds a [remote.http][] component to retrieve the module from a remote
HTTP server. This allows you to use a single module loader, rather than a `remote.http`
component paired with a [module.string][] component.

[module]: {{< relref "../../concepts/modules.md" >}}
[remote.http]: {{< relref "./remote.http.md" >}}
[module.string]: {{< relref "./module.string.md" >}}
[module loader]: {{< relref "../../concepts/modules.md#module-loaders" >}}

## Usage

```river
module.http "LABEL" {
  url = URL

  arguments {
    MODULE_ARGUMENT_1 = VALUE_1
    ...
  }
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`url` | `string` | URL to poll. | | yes
`method` | `string` | Define HTTP method for the request | `"GET"` | no
`headers` | `map(string)` | Custom headers for the request. | `{}` | no
`poll_frequency` | `duration` | Frequency to poll the URL. | `"1m"` | no
`poll_timeout` | `duration` | Timeout when polling the URL. | `"10s"` | no
`is_secret` | `bool` | Whether the response body should be treated as a secret. | false | no

[secret]: {{< relref "../../concepts/config-language/expressions/types_and_values.md#secrets" >}}

## Blocks

The following blocks are supported inside the definition of `module.http`:

Hierarchy        | Block      | Description | Required
---------------- | ---------- | ----------- | --------
arguments | [arguments][] | Arguments to pass to the module. | no

[arguments]: #arguments-block

### arguments block

The `arguments` block specifies the list of values to pass to the loaded
module.

The attributes provided in the `arguments` block are validated based on the
[argument blocks][] defined in the module source:

* If a module source marks one of its arguments as required, it must be
  provided as an attribute in the `arguments` block of the module loader.

* Attributes in the `argument` block of the module loader are rejected if
  they are not defined in the module source.

[argument blocks]: {{< relref "../config-blocks/argument.md" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`exports` | `map(any)` | The exports of the Module loader.

`exports` exposes the `export` config block inside a module. It can be accessed
from the parent config via `module.http.LABEL.exports.EXPORT_LABEL`.

Values in `exports` correspond to [export blocks][] defined in the module
source.

[export blocks]: {{< relref "../config-blocks/export.md" >}}

## Component health

`module.http` is reported as healthy if the most recent load of the module was
successful.

Before the first load of the module, the health is reported as `Unknown`.

If the module is not loaded successfully, the current health displays as
unhealthy, and the health includes the error from loading the module.

## Debug information

`module.http` does not expose any component-specific debug information.

## Debug metrics

`module.http` does not expose any component-specific debug metrics.

## Example

In this example, the `module.http` component loads a module from a locally running
HTTP server, polling for changes once every minute.

The module sets up a Redis exporter and exports the list of targets to the parent config to scrape
and remote write.


Parent:

```river
module.http "remote_module" {
  url              = "http://localhost:8080/redis_module.yaml"
  poll_frequency   = "1m"
}

prometheus.exporter.unix "default" { }

prometheus.scrape "local_agent" {
  targets         = concat(prometheus.exporter.unix.default.targets, module.http.remote_module.exports.targets)
  forward_to      = [module.http.metrics.exports.prometheus_remote_write.receiver]
  scrape_interval = "10s"
}

prometheus.remote_write "default" {
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

Module:

```river
prometheus.exporter.redis "local_redis" {
  redis_addr          = REDIS_ADDR
  redis_password_file = REDIS_PASSWORD_FILE
}

export "redis_targets" {
  value = prometheus.exporter.redis.local_redis.targets
}
```
Replace the following:
  - `REDIS_ADDR`: The address of your Redis instance.
  - `REDIS_PASSWORD_FILE`: The path to a file containing the password for your Redis instance.
