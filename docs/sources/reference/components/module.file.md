---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/module.file/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/module.file/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/module.file/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/module.file/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/module.file/
description: Learn about module.file
labels:
  stage: beta
title: module.file
---

# module.file

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

`module.file` is a *module loader* component. A module loader is a {{< param "PRODUCT_NAME" >}}
component which retrieves a [module][] and runs the components defined inside of it.

`module.file` simplifies the configurations for modules loaded from a file by embedding
a [local.file][] component. This allows a single module loader to do the equivalence of
using the more generic [module.string][] paired with a [local.file][] component.

[module]: {{< relref "../../concepts/modules.md" >}}
[local.file]: {{< relref "./local.file.md" >}}
[module.string]: {{< relref "./module.string.md" >}}

## Usage

```river
module.file "LABEL" {
  filename = FILENAME

  arguments {
    MODULE_ARGUMENT_1 = VALUE_1
    MODULE_ARGUMENT_2 = VALUE_2
    ...
  }
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`filename`       | `string`   | Path of the file on disk to watch | | yes
`detector`       | `string`   | Which file change detector to use (fsnotify, poll) | `"fsnotify"` | no
`poll_frequency` | `duration` | How often to poll for file changes | `"1m"` | no
`is_secret`      | `bool`     | Marks the file as containing a [secret][] | `false` | no

[secret]: {{< relref "../../concepts/config-language/expressions/types_and_values.md#secrets" >}}

{{< docs/shared lookup="flow/reference/components/local-file-arguments-text.md" source="agent" version="<AGENT_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of `module.file`:

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

* Attributes in the `argument` block of the module loader will be rejected if
  they are not defined in the module source.

[argument blocks]: {{< relref "../config-blocks/argument.md" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`exports` | `map(any)` | The exports of the Module loader.

`exports` exposes the `export` config block inside a module. It can be accessed
from the parent config via `module.file.LABEL.exports.EXPORT_LABEL`.

Values in `exports` correspond to [export blocks][] defined in the module
source.

[export blocks]: {{< relref "../config-blocks/export.md" >}}

## Component health

`module.file` is reported as healthy if the most recent load of the module was
successful.

If the module is not loaded successfully, the current health displays as
unhealthy and the health includes the error from loading the module.

## Debug information

`module.file` does not expose any component-specific debug information.

## Debug metrics

`module.file` does not expose any component-specific debug metrics.

## Example

In this example, we pass credentials from a parent config to a module which loads
a `prometheus.remote_write` component. The exports of the
`prometheus.remote_write` component are exposed to parent config, allowing
the parent config to pass metrics to it.

Parent:

```river
module.file "metrics" {
  filename = "/path/to/prometheus_remote_write_module.river"

  arguments {
    username = env("PROMETHEUS_USERNAME")
    password = env("PROMETHEUS_PASSWORD")
  }
}

prometheus.exporter.unix "default" { }

prometheus.scrape "local_agent" {
  targets         = prometheus.exporter.unix.default.targets
  forward_to      = [module.file.metrics.exports.prometheus_remote_write.receiver]
  scrape_interval = "10s"
}
```

Module:

```river
argument "username" { }

argument "password" { }

export "prometheus_remote_write" {
  value = prometheus.remote_write.grafana_cloud
}

prometheus.remote_write "grafana_cloud" {
  endpoint {
    url = "https://prometheus-us-central1.grafana.net/api/prom/push"

    basic_auth {
      username = argument.username.value
      password = argument.password.value
    }
  }
}
```
