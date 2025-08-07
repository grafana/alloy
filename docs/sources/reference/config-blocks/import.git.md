---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/import.git/
description: Learn about the import.git configuration block
labels:
  stage: general-availability
  products:
    - oss
title: import.git
---

# `import.git`

The `import.git` block imports custom components from a Git repository and exposes them to the importer.
`import.git` blocks must be given a label that determines the namespace where custom components are exposed.

The entire repository is cloned, and the module path is accessible via the `module_path` keyword.
This enables, for example, your module to import other modules within the repository by setting relative paths in the [import.file][] blocks.

## Usage

```alloy
import.git "<NAMESPACE>" {
  repository = "<GIT_REPOSTORY>"
  path       = "<PATH_TO_MODULE>"
}
```

## Arguments

You can use the following arguments with `import.git`:

| Name             | Type       | Description                                             | Default  | Required |
| ---------------- | ---------- | ------------------------------------------------------- | -------- | -------- |
| `path`           | `string`   | The path in the repository where the module is stored.  |          | yes      |
| `repository`     | `string`   | The Git repository address to retrieve the module from. |          | yes      |
| `pull_frequency` | `duration` | The frequency to pull the repository for updates.       | `"60s"`  | no       |
| `revision`       | `string`   | The Git revision to retrieve the module from.           | `"HEAD"` | no       |

You must set the `repository` attribute to a repository address that Git would recognize with a `git clone <REPOSITORY_ADDRESS>` command, such as `https://github.com/grafana/alloy.git`.

When provided, the `revision` attribute must be set to a valid branch, tag, or commit SHA within the repository.

You must set the `path` attribute to a path accessible from the repository's root.
It can either be an {{< param "PRODUCT_NAME" >}} configuration file such as `<FILE_NAME>.alloy` or `<DIR_NAME>/<FILE_NAME>.alloy` or
a directory containing {{< param "PRODUCT_NAME" >}} configuration files such as `<DIR_NAME>` or `.` if the {{< param "PRODUCT_NAME" >}} configuration files are stored at the root of the repository.

If `pull_frequency` isn't `"0s"`, the Git repository is pulled for updates at the frequency specified.
If it's set to `"0s"`, the Git repository is pulled once on init.

{{< admonition type="warning" >}}
Pulling hosted Git repositories too often can result in throttling.
{{< /admonition >}}

## Blocks

You can use the following blocks with `import.git`:

| Block                      | Description                                                  | Required |
| -------------------------- | ------------------------------------------------------------ | -------- |
| [`basic_auth`][basic_auth] | Configure `basic_auth` for authenticating to the repository. | no       |
| [`ssh_key`][ssh_key]       | Configure an SSH Key for authenticating to the repository.   | no       |

### `basic_auth`

| Name       | Type     | Description          | Default | Required |
| ---------- | -------- | -------------------- | ------- | -------- |
| `password` | `secret` | Basic auth password. |         | no       |
| `username` | `string` | Basic auth username. |         | no       |

### `ssh_key`

| Name         | Type     | Description                       | Default | Required |
| ------------ | -------- | --------------------------------- | ------- | -------- |
| `username`   | `string` | SSH username.                     | `""`    | yes      |
| `key_file`   | `string` | SSH private key path.             | `""`    | no       |
| `key`        | `secret` | SSH private key.                  |         | no       |
| `passphrase` | `secret` | Passphrase for SSH key if needed. |         | no       |

## Examples

This example imports custom components from a Git repository and uses a custom component to add two numbers:

```alloy
import.git "math" {
  repository = "https://github.com/wildum/module.git"
  revision   = "master"
  path       = "math.alloy"
}

math.add "default" {
  a = 15
  b = 45
}
```

This example imports custom components from a directory in a Git repository and uses a custom component to add two numbers:

```alloy
import.git "math" {
  repository = "https://github.com/wildum/module.git"
  revision   = "master"
  path       = "modules"
}

math.add "default" {
  a = 15
  b = 45
}
```

[import.file]: ../import.file/
[basic_auth]: #basic_auth
[ssh_key]: #ssh_key
