---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/import.string/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/import.string/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/import.string/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/import.string/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/import.string/
description: Learn about the import.string configuration block
labels:
  stage: beta
title: import.string
---

# import.string

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

The `import.string` block imports custom components from a string and exposes them to the importer.
`import.string` blocks must be given a label that determines the namespace where custom components are exposed.

## Usage

```river
import.string "NAMESPACE" {
  content = CONTENT
}
```

## Arguments

The following arguments are supported:

Name      | Type                 | Description                                                 | Default | Required
----------|----------------------|-------------------------------------------------------------|---------|---------
`content` | `secret` or `string` | The contents of the module to import as a secret or string. |         | yes

`content` is a string that contains the configuration of the module to import.
`content` is typically loaded by using the exports of another component. For example,

- `local.file.LABEL.content`
- `remote.http.LABEL.content`
- `remote.s3.LABEL.content`

## Example

This example imports a module from the content of a file stored in an S3 bucket and instantiates a custom component from the import that adds two numbers:

```river
remote.s3 "module" {
  path = "s3://test-bucket/module.river"
}

import.string "math" {
  content = remote.s3.module.content
}

math.add "default" {
  a = 15
  b = 45
}
```

{{% docs/reference %}}
[module]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/modules"
[module]:"/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/modules"
{{% /docs/reference %}}
