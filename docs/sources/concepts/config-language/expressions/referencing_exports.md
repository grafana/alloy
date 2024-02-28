---
aliases:
- ../../configuration-language/expressions/referencing-exports/ # /docs/agent/latest/flow/concepts/configuration-language/expressions/referencing-exports/
- /docs/grafana-cloud/agent/flow/concepts/config-language/expressions/referencing_exports/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/expressions/referencing_exports/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/expressions/referencing_exports/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/expressions/referencing_exports/
# Previous page aliases for backwards compatibility:
- ../../../configuration-language/expressions/referencing-exports/ # /docs/agent/latest/flow/configuration-language/expressions/referencing-exports/
- /docs/grafana-cloud/agent/flow/config-language/expressions/referencing_exports/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/expressions/referencing_exports/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/expressions/referencing_exports/
- /docs/grafana-cloud/send-data/agent/flow/config-language/expressions/referencing_exports/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/expressions/referencing_exports/
description: Learn about referencing component exports
title: Referencing component exports
weight: 200
---

# Referencing component exports

Referencing exports enables River to configure and connect components dynamically using expressions.
While components can work in isolation, they're more useful when one component's behavior and data flow are bound to the exports of another,
building a dependency relationship between the two.

Such references can only appear as part of another component's arguments or a configuration block's fields.
Components can't reference themselves.

## Using references

You build references by combining the component's name, label, and named export with dots.

For example, you can reference the contents of a file exported by the `local.file` component labeled `target` as `local.file.target.content`.
Similarly, a `prometheus.remote_write` component instance labeled `onprem` exposes its receiver for metrics on `prometheus.remote_write.onprem.receiver`.

The following example shows some references.

```river
local.file "target" {
  filename = "/etc/agent/target"
}

prometheus.scrape "default" {
  targets    = [{ "__address__" = local.file.target.content }]
  forward_to = [prometheus.remote_write.onprem.receiver]
}

prometheus.remote_write "onprem" {
  endpoint {
    url = "http://prometheus:9009/api/prom/push"
  }
}
```

In the preceding example, you wired together a very simple pipeline by writing a few River expressions.

![Flow of example pipeline](/media/docs/agent/flow_referencing_exports_diagram.svg)

After the value is resolved, it must match the [type][] of the attribute it is assigned to.
While you can only configure attributes using the basic River types,
the exports of components can take on special internal River types, such as Secrets or Capsules, which expose different functionality.

{{% docs/reference %}}
[type]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/config-language/expressions/types_and_values"
[type]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/expressions/types_and_values"
{{% /docs/reference %}}