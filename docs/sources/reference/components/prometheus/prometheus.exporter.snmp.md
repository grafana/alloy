---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.snmp/
aliases:
  - ../prometheus.exporter.snmp/ # /docs/alloy/latest/reference/components/prometheus.exporter.snmp/
description: Learn about prometheus.exporter.snmp
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-19
title: prometheus.exporter.snmp
---

# `prometheus.exporter.snmp`

The `prometheus.exporter.snmp` component embeds the [`snmp_exporter`][snmp-exporter].
The `snmp_exporter` lets you collect SNMP data and expose them as Prometheus metrics.

{{< admonition type="note" >}}
`prometheus.exporter.snmp` uses the configuration format introduced in version {{< param "SNMP_VERSION" >}} of the Prometheus `snmp_exporter`.
{{< /admonition >}}

You can specify multiple `prometheus.exporter.snmp` components by giving them different labels.

## Usage

```alloy
prometheus.exporter.snmp "<LABEL>" {
  config_files = ["<SNMP_CONFIG_FILE_PATH>", ...]

  target "<TARGET_NAME>" {
    address = "<TARGET_ADDRESS>"
  }
}
```

or

```alloy
prometheus.exporter.snmp "<LABEL>" {
  config_files = ["<SNMP_CONFIG_FILE_PATH>", ...]
  targets     = <TARGET_LIST>
}

## Arguments

You can use the following arguments with `prometheus.exporter.snmp`:

| Name                    | Type                 | Description                                                                                                              | Default     | Required |
| ----------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------ | ----------- | -------- |
| `concurrency`           | `int`                | Number of modules to scrape in parallel for each target.                                                                 | `1`         | no       |
| `config`                | `string` or `secret` | SNMP configuration as inline string.                                                                                     |             | no       |
| `config_file`           | `string`             | Deprecated. Use `config_files` instead.                                                                                  |             | no       |
| `config_files`          | `list(string)`       | Paths to YAML files that define SNMP modules and authentications.                                                        |             | no       |
| `config_merge_strategy` | `string`             | Defines how `config` or `config_files` contents merge with the embedded SNMP configuration. Can be `replace` or `merge`. | `"replace"` | no       |
| `targets`               | `list(map(string))`  | List of SNMP targets, where each target is a map of labels.                                                              |             | no       |

The `config_files` argument points to YAML files that define which `snmp_exporter` modules to use.
Refer to the [`snmp_exporter` configuration documentation][snmp-exporter-config] for details on how to generate a configuration file.

The `config_file` and `config_files` arguments are mutually exclusive.
If you set both, the component fails with an invalid configuration error.

The `config` argument must be a YAML document as string defining which SNMP modules and authorizations to use.
`config` is typically loaded by using the exports of another component.
For example,

- `local.file.LABEL.content`
- `remote.http.LABEL.content`
- `remote.s3.LABEL.content`

Set either `config` or a configuration file argument, `config_files` or the deprecated `config_file`, not both.
If you set `config` together with `config_files` or `config_file`, the component fails with an invalid configuration error.

Set `config_merge_strategy` to `merge` to add additional configuration to the embedded SNMP configuration.
For example, if you need to add a few custom `auth` settings without regenerating the whole configuration.

The `targets` argument is an alternative to the [`target`][target] block, and is useful when another component supplies the SNMP targets.
Use either `target` blocks or the `targets` argument, not both.
If you use both, the component fails with an invalid configuration error.

You can set the following labels on a target:

- `name`: The name of the target. Required.
- `address` or `__address__`: The address of the SNMP device. Required.
- `module`: SNMP modules to use for polling, separated by comma.
- `auth`: The SNMP authentication profile to use.
- `walk_params`: The `walk_param` profile to use for this target.
- `snmp_context`: Override the `context_name` parameter in the SNMP configuration file.

The component adds any other labels you define to the scraped metrics.

The component sets the `instance` label on its exported targets to the component ID, for example, `prometheus.exporter.snmp.example`.
The component also appends the target name to the `job` label of each exported target, for example, `integrations/snmp/network_switch_1`.

## Blocks

You can use the following blocks with `prometheus.exporter.snmp`:

{{< docs/alloy-config >}}

| Block                      | Description                                                                | Required |
| -------------------------- | -------------------------------------------------------------------------- | -------- |
| [`target`][target]         | Configures an SNMP target.                                                 | no       |
| [`walk_param`][walk_param] | Configures an SNMP connection profile that overrides the default settings. | no       |

[target]: #target
[walk_param]: #walk_param

{{< /docs/alloy-config >}}

### `target`

The `target` block defines an individual SNMP target.
You can define multiple `target` blocks to configure multiple targets.
Each `target` block requires a label, which the component uses in the `job` label of the target.

You can use the following arguments with the `target` block:

| Name           | Type          | Description                                                           | Default | Required |
| -------------- | ------------- | --------------------------------------------------------------------- | ------- | -------- |
| `address`      | `string`      | The address of the SNMP device.                                       |         | yes      |
| `auth`         | `string`      | The SNMP authentication profile to use.                               | `""`    | no       |
| `labels`       | `map(string)` | Map of labels to apply to all metrics captured from the target.       | `{}`    | no       |
| `module`       | `string`      | SNMP modules to use for polling, separated by comma.                  | `""`    | no       |
| `snmp_context` | `string`      | Override the `context_name` parameter in the SNMP configuration file. | `""`    | no       |
| `walk_params`  | `string`      | The `walk_param` profile to use for this target.                      | `""`    | no       |

### `walk_param`

The `walk_param` block defines an individual SNMP connection profile that overrides the default SNMP settings.
You can define multiple `walk_param` blocks to configure multiple connection profiles.
Each `walk_param` block requires a label, which is the name that the `walk_params` argument of a target refers to.

You can use the following arguments with the `walk_param` block:

| Name              | Type       | Description                                   | Default | Required |
| ----------------- | ---------- | --------------------------------------------- | ------- | -------- |
| `max_repetitions` | `int`      | How many objects to request with GET/GETBULK. | `25`    | no       |
| `retries`         | `int`      | How many times to retry a failed request.     | `0`     | no       |
| `timeout`         | `duration` | Timeout for each individual SNMP request.     | `5s`    | no       |

A `walk_param` block always overrides the `retries` value from the module configuration.
If you omit `retries`, targets that use the block don't retry failed requests.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.snmp` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.snmp` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.snmp` doesn't expose any component-specific debug metrics.

## Example

The following example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.snmp`:

```alloy
prometheus.exporter.snmp "example" {
  config_files = ["snmp_modules.yml"]

  target "network_switch_1" {
    address     = "192.168.1.2"
    module      = "system,if_mib"
    walk_params = "public"
    labels = {
      "env" = "dev",
    }
  }

  target "network_router_2" {
    address     = "192.168.1.3"
    module      = "system,if_mib,mikrotik"
    walk_params = "private"
  }

  walk_param "private" {
    retries = 2
  }

  walk_param "public" {
    retries = 2
  }
}

// Configure a prometheus.scrape component to collect SNMP metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.snmp.example.targets
  forward_to = [ /* ... */ ]
}
```

The following example uses an embedded configuration with secrets:

```alloy
local.file "snmp_config" {
  filename  = "snmp_modules.yml"
  is_secret = true
}

prometheus.exporter.snmp "example" {
  config = local.file.snmp_config.content

  target "network_switch_1" {
    address     = "192.168.1.2"
    module      = "system,if_mib"
    walk_params = "public"
  }

  target "network_router_2" {
    address     = "192.168.1.3"
    module      = "system,if_mib,mikrotik"
    walk_params = "private"
  }

  walk_param "private" {
    retries = 2
  }

  walk_param "public" {
    retries = 2
  }
}

// Configure a prometheus.scrape component to collect SNMP metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.snmp.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

The following example uses the alternative way to pass targets:

```alloy
prometheus.exporter.snmp "example" {
  config_files = ["snmp_modules.yml"]

  targets = [
    {
      "name"        = "network_switch_1",
      "address"     = "192.168.1.2",
      "module"      = "system,if_mib",
      "walk_params" = "public",
      "env"         = "dev",
    },
    {
      "name"        = "network_router_2",
      "address"     = "192.168.1.3",
      "module"      = "system,if_mib,mikrotik",
      "walk_params" = "private",
    },
  ]

  walk_param "private" {
    retries = 2
  }

  walk_param "public" {
    retries = 2
  }
}

// Configure a prometheus.scrape component to collect SNMP metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.snmp.example.targets
  forward_to = [ /* ... */ ]
}
```

The following example uses the [`local.file` component][file] to read targets from a YAML file and send them to the `prometheus.exporter.snmp` component:

```alloy
local.file "targets" {
  filename = "targets.yml"
}

prometheus.exporter.snmp "example" {
  config_files = ["snmp_modules.yml"]

  targets = encoding.from_yaml(local.file.targets.content)

  walk_param "private" {
    retries = 2
  }

  walk_param "public" {
    retries = 2
  }
}

// Configure a prometheus.scrape component to collect SNMP metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.snmp.example.targets
  forward_to = [ /* ... */ ]
}
```

The YAML file in this example looks like this:

```yaml
- name: t1
  address: localhost:161
  module: default
  auth: public_v2
- name: t2
  address: localhost:161
  module: default
  auth: public_v2
```

The following example uses the [`discovery.file` component][disc] to send targets to the `prometheus.exporter.snmp` component:

```alloy
discovery.file "example" {
  files = ["targets.yml"]
}

prometheus.exporter.snmp "example" {
  config_files = ["snmp_modules.yml"]
  targets = discovery.file.example.targets
}

// Configure a prometheus.scrape component to collect SNMP metrics.
prometheus.scrape "demo" {
    targets    = prometheus.exporter.snmp.example.targets
    forward_to = [ /* ... */ ]
}
```

The YAML file in this example looks like this:

```yaml
- targets:
  - localhost:161
  labels:
    name: t1
    module: default
    auth: public_v2
- targets:
  - localhost:161
  labels:
    name: t2
    module: default
    auth: public_v2
```

[scrape]: ../prometheus.scrape/
[file]: ../../local/local.file/
[disc]: ../../discovery/discovery.file/
[target]: #target
[snmp-exporter]: https://github.com/prometheus/snmp_exporter/tree/{{< param "SNMP_VERSION" >}}
[snmp-exporter-config]: https://github.com/prometheus/snmp_exporter/tree/{{< param "SNMP_VERSION" >}}?tab=readme-ov-file#configuration

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.snmp` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
