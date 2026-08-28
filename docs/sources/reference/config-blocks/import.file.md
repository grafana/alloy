---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/import.file/
description: Learn about the import.file configuration block
labels:
  stage: general-availability
  products:
    - oss
title: import.file
---

# `import.file`

The `import.file` block imports custom components from a file or a directory and exposes them to the importer.
`import.file` blocks must be given a label that determines the namespace where custom components are exposed.

Imported directories are treated as single modules to support composability.
That means that you can define a custom component in one file and use it in another custom component in another file in the same directory.

You can use the keyword `module_path` in combination with the `stdlib` function [`file.path_join`][file.path_join] to import a module relative to the current module's path.
The `module_path` keyword works for modules that are imported via `import.file`, `import.git`, and `import.string`.

## Usage

```alloy
import.file "<NAMESPACE>" {
  filename = <PATH_NAME>
}
```

## Arguments

You can use the following arguments with `import.file`:

| Name             | Type       | Description                                              | Default      | Required |
| ---------------- | ---------- | -------------------------------------------------------- | ------------ | -------- |
| `filename`       | `string`   | Path of the file or directory on disk to watch.          |              | yes      |
| `detector`       | `string`   | Which file change detector to use, `fsnotify` or `poll`. | `"fsnotify"` | no       |
| `poll_frequency` | `duration` | How often to poll for file changes.                      | `"1m"`       | no       |

{{< docs/shared lookup="reference/components/local-file-arguments-text.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Examples

### Import a module from a local file

This example imports a module from a file and instantiates a custom component from the import that adds two numbers:

**`main.alloy`**

```alloy
import.file "math" {
  filename = "module.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

**`module.alloy`**

```alloy
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

### Import a module in a module imported via import.git

This example imports a module from a file inside of a module that's imported via [`import.git`][import.git]:

**`main.alloy`**

```alloy
import.git "math" {
  repository = "https://github.com/wildum/module.git"
  path       = "relative_math.alloy"
  revision   = "master"
}

math.add "default" {
  a = 15
  b = 45
}
```

**`relative_math.alloy`**

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

**`lib.alloy`**

```alloy
declare "plus" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

### Import a module in a module imported via import.file

This example imports a module from a file inside of a module that's imported via another `import.file`:

**`main.alloy`**

```alloy
import.file "math" {
  filename = "path/to/module/relative_math.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

**`relative_math.alloy`**

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

**`lib.alloy`**

```alloy
declare "plus" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

[file.path_join]: ../../stdlib/file/
[import.git]: ../import.git/
