---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/module.git/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/module.git/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/module.git/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/module.git/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/module.git/
description: Learn about module.git
labels:
  stage: beta
title: module.git
---

# module.git

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

`module.git` is a *module loader* component. A module loader is a {{< param "PRODUCT_NAME" >}}
component which retrieves a [module][] and runs the components defined inside of it.

`module.git` retrieves a module source from a file in a Git repository.

[module]: {{< relref "../../concepts/modules.md" >}}

## Usage

```river
module.git "LABEL" {
  repository = "GIT_REPOSTORY"
  path       = "PATH_TO_MODULE"

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
`repository` | `string` | The Git repository address to retrieve the module from. | | yes
`revision` | `string` | The Git revision to retrieve the module from. | `"HEAD"` | no
`path` | `string` | The path in the repository where the module is stored. | | yes
`pull_frequency` | `duration` | The frequency to pull the repository for updates. | `"60s"` | no

The `repository` attribute must be set to a repository address that would be
recognized by Git with a `git clone REPOSITORY_ADDRESS` command, such as
`htts://github.com/grafana/agent.git`.

The `revision` attribute, when provided, must be set to a valid branch, tag, or
commit SHA within the repository.

The `path` attribute must be set to a path which is accessible from the root of
the repository, such as `FILE_NAME.river` or `FOLDER_NAME/FILE_NAME.river`.

If `pull_frequency` is not `"0s"`, the Git repository will be pulled for
updates at the frequency specified, causing the loaded module to update with
the retrieved changes.

## Blocks

The following blocks are supported inside the definition of `module.git`:

Hierarchy        | Block      | Description | Required
---------------- | ---------- | ----------- | --------
basic_auth | [basic_auth][] | Configure basic_auth for authenticating to the repo. | no
ssh_key | [ssh_key][] | Configure a SSH Key for authenticating to the repo. | no
arguments | [arguments][] | Arguments to pass to the module. | no

[basic_auth]: #basic_auth-block
[ssh_key]: #ssh_key-block
[arguments]: #arguments-block

### basic_auth block

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### ssh_key block

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username`  | `string` | SSH username. | | yes
`key`       | `secret` | SSH private key | | no
`key_file`  | `string` | SSH private key path. | | no
`passphrase` | `secret` | Passphrase for SSH key if needed. | | no

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
from the parent config via `module.git.COMPONENT_LABEL.exports.EXPORT_LABEL`.

Values in `exports` correspond to [export blocks][] defined in the module
source.

[export blocks]: {{< relref "../config-blocks/export.md" >}}

## Component health

`module.git` is reported as healthy if the repository was cloned successfully
and most recent load of the module was successful.

## Debug information

`module.git` includes debug information for:

* The full SHA of the currently checked out revision.
* The most recent error when trying to fetch the repository, if any.

## Debug metrics

`module.git` does not expose any component-specific debug metrics.

## Examples

This example uses a module loaded from a Git repository which adds two numbers:

```river
module.git "add" {
  repository = "https://github.com/rfratto/agent-modules.git"
  revision   = "main"
  path       = "add/module.river"

  arguments {
    a = 15
    b = 45
  }
}
```

The same example as above using basic auth:
```river
module.git "add" {
  repository = "https://github.com/rfratto/agent-modules.git"
  revision   = "main"
  path       = "add/module.river"

  basic_auth {
    username = "USERNAME"
    password = "PASSWORD"
  }

  arguments {
    a = 15
    b = 45
  }
}
```

Using SSH Key from another component:
```river
local.file "ssh_key" {
  filename = "PATH/TO/SSH.KEY"
  is_secret = true
}

module.git "add" {
  repository = "github.com:rfratto/agent-modules.git"
  revision   = "main"
  path       = "add/module.river"

  ssh_key {
    username = "git"
    key = local.file.ssh_key.content
  }

  arguments {
    a = 15
    b = 45
  }
}
```

The same example as above using SSH Key auth:
```river
module.git "add" {
  repository = "github.com:rfratto/agent-modules.git"
  revision   = "main"
  path       = "add/module.river"

  ssh_key {
    username = "git"
    key_file = "PATH/TO/SSH.KEY"
  }

  arguments {
    a = 15
    b = 45
  }
}
```
