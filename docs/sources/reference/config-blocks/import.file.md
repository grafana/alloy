---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/import.file/
description: Learn about the import.file configuration block
title: import.file
---

<span class="badge docs-labels__stage docs-labels__item">Public preview</span>

# import.file

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `import.file` block imports custom components from a file or a directory and exposes them to the importer.
`import.file` blocks must be given a label that determines the namespace where custom components are exposed.

Imported directories are treated as single modules to support composability.
That means that you can define a custom component in one file and use it in another custom component in another file
in the same directory.

You can use the keyword `module_path` in combination with the `stdlib` function [file.path_join][] to import a module relative to the current module's path.
The `module_path` keyword works for modules that are imported via `import.file`, `import.git` and `import.string`.

## Usage

```alloy
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

{{< collapse title="main.alloy" >}}

```alloy
import.file "math" {
  filename = "module.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

{{< /collapse >}}

{{< collapse title="module.alloy" >}}

```alloy
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

{{< /collapse >}}


This example imports a module from a file inside of a module that is imported via [import.git][]:

{{< collapse title="main.alloy" >}}

```alloy
import.git "math" {
  repository = "https://github.com/wildum/module.git"
  path       = "relative_math.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

{{< /collapse >}}

{{< collapse title="relative_math.alloy" >}}

```alloy
import.file "lib" {
  filename = file.path_join(module_path, "lib.alloy")
}

declare "add" {
  argument "a" {}
  argument "b" {}

  lib.plus "default" {
    a = argument.a.value
    b = argument.b.value
  }

  export "output" {
    value = lib.plus.default.sum
  }
}
```

{{< /collapse >}}

{{< collapse title="lib.alloy" >}}

```alloy
declare "plus" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

{{< /collapse >}}

This example imports a module from a file inside of a module that is imported via another `import.file`:

{{< collapse title="main.alloy" >}}

```alloy
import.file "math" {
  filename = "path/to/module/relative_math.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

{{< /collapse >}}

{{< collapse title="relative_math.alloy" >}}

```alloy
import.file "lib" {
  filename = file.path_join(module_path, "lib.alloy")
}

declare "add" {
  argument "a" {}
  argument "b" {}

  lib.plus "default" {
    a = argument.a.value
    b = argument.b.value
  }

  export "output" {
    value = lib.plus.default.sum
  }
}
```

{{< /collapse >}}

{{< collapse title="lib.alloy" >}}

```alloy
declare "plus" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

{{< /collapse >}}



[file.path_join]: ../../stdlib/file/
[import.git]: ../import.git/