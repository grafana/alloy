---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/declare/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/declare/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/declare/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/declare/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/declare/
description: Learn about the declare configuration block
menuTitle: declare
title: declare block
---

# declare block

`declare` is an optional configuration block used to define a new [custom component][].
`declare` blocks must be given a label that determines the name of the custom component.

## Example

```river
declare "COMPONENT_NAME" {
    COMPONENT_DEFINITION
}
```

## Arguments

The `declare` block has no predefined schema for its arguments.
The body of the `declare` block is used as the component definition.
The body can contain the following:

* [argument][] blocks
* [export][] blocks
* [declare][] blocks
* [import][] blocks
* Component definitions (either built-in or custom components)

The `declare` block may not contain any configuration blocks that aren't listed above.

## Exported fields

The `declare` block has no predefined schema for its exports.
The fields exported by the `declare` block are determined by the [export blocks][export] found in its definition.

## Example

This example creates and uses a custom component that self-collects process metrics and forwards them to an argument specified by the user of the custom component:

```river
declare "self_collect" {
  argument "metrics_output" {
    optional = false
    comment  = "Where to send collected metrics."
  }

  prometheus.scrape "selfmonitor" {
    targets = [{
      __address__ = "127.0.0.1:12345",
    }]

    forward_to = [argument.metrics_output.value]
  }
}

self_collect "example" {
  metrics_output = prometheus.remote_write.example.receiver
}

prometheus.remote_write "example" {
  endpoint {
    url = REMOTE_WRITE_URL
  }
}
```

{{% docs/reference %}}
[argument]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/config-blocks/argument"
[argument]:"/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/argument"
[export]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/config-blocks/export"
[export]:"/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/export"
[declare]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/config-blocks/declare"
[declare]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/declare"
[import]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/modules#importing-modules"
[import]:"/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/modules#importing-modules"
{{% /docs/reference %}}
