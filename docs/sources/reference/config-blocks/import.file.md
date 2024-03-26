---
aliases:
- ./reference/config-blocks/import.file/
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/import.file/
description: Learn about the import.file configuration block
labels:
  stage: beta
title: import.file
---

# import.file

{{< docs/shared lookup="stability/beta.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `import.file` block imports custom components from a file or a directory and exposes them to the importer.
`import.file` blocks must be given a label that determines the namespace where custom components are exposed.

Imported directories are treated as single modules to support composability.
That means that you can define a custom component in one file and use it in another custom component in another file
in the same directory.

## Usage

```river
import.file "NAMESPACE" {
  filename = PATH_NAME
}
```

## Arguments

The following arguments are supported:

| Name             | Type       | Description                                         | Default      | Required |
| ---------------- | ---------- | --------------------------------------------------- | ------------ | -------- |
| `filename`       | `string`   | Path of the file or directory on disk to watch.     |              | yes      |
| `detector`       | `string`   | Which file change detector to use (fsnotify, poll). | `"fsnotify"` | no       |
| `poll_frequency` | `duration` | How often to poll for file changes.                 | `"1m"`       | no       |

{{< docs/shared lookup="reference/components/local-file-arguments-text.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Example

This example imports a module from a file and instantiates a custom component from the import that adds two numbers:

{{< collapse title="module.alloy" >}}

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

{{< collapse title="importer.alloy" >}}

```river
import.file "math" {
  filename = "module.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

{{< /collapse >}}
