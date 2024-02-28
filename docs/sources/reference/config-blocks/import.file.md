---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/import.file/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/import.file/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/import.file/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/import.file/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/import.file/
description: Learn about the import.file configuration block
labels:
  stage: beta
title: import.file
---

# import.file

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

The `import.file` block imports custom components from a file and exposes them to the importer.
`import.file` blocks must be given a label that determines the namespace where custom components are exposed.

## Usage

```river
import.file "NAMESPACE" {
  filename = FILENAME
}
```

## Arguments

The following arguments are supported:

Name             | Type       | Description                                         | Default      | Required
-----------------|------------|-----------------------------------------------------|--------------|---------
`filename`       | `string`   | Path of the file on disk to watch.                  |              | yes
`detector`       | `string`   | Which file change detector to use (fsnotify, poll). | `"fsnotify"` | no
`poll_frequency` | `duration` | How often to poll for file changes.                 | `"1m"`       | no

{{< docs/shared lookup="flow/reference/components/local-file-arguments-text.md" source="agent" version="<AGENT_VERSION>" >}}

## Example

This example imports a module from a file and instantiates a custom component from the import that adds two numbers:

{{< collapse title="module.river" >}}
```river
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```
{{< /collapse >}}

{{< collapse title="importer.river" >}}
```river
import.file "math" {
  filename = "module.river"
}

math.add "default" {
  a = 15
  b = 45
}
```
{{< /collapse >}}

{{% docs/reference %}}
[module]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/modules"
[module]:"/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/modules"
{{% /docs/reference %}}
