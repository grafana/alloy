---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/import.git/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/import.git/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/import.git/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/import.git/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/import.git/
description: Learn about the import.git configuration block
labels:
  stage: beta
title: import.git
---

# import.git

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

The `import.git` block imports custom components from a Git repository and exposes them to the importer.
`import.git` blocks must be given a label that determines the namespace where custom components are exposed.

## Usage

```river
import.git "NAMESPACE" {
  repository = "GIT_REPOSTORY"
  path       = "PATH_TO_MODULE"
}
```

## Arguments

The following arguments are supported:

Name             | Type       | Description                                             | Default  | Required
-----------------|------------|---------------------------------------------------------|----------|---------
`repository`     | `string`   | The Git repository address to retrieve the module from. |          | yes
`revision`       | `string`   | The Git revision to retrieve the module from.           | `"HEAD"` | no
`path`           | `string`   | The path in the repository where the module is stored.  |          | yes
`pull_frequency` | `duration` | The frequency to pull the repository for updates.       | `"60s"`  | no

The `repository` attribute must be set to a repository address that would be
recognized by Git with a `git clone REPOSITORY_ADDRESS` command, such as
`https://github.com/grafana/agent.git`.

You must set the `repository` attribute to a repository address that Git would recognize
with a `git clone REPOSITORY_ADDRESS` command, such as `https://github.com/grafana/agent.git`.

When provided, the `revision` attribute must be set to a valid branch, tag, or
commit SHA within the repository.

You must set the `path` attribute to a path accessible from the repository's root,
such as `FILE_NAME.river` or `FOLDER_NAME/FILE_NAME.river`.

If `pull_frequency` isn't `"0s"`, the Git repository is pulled for updates at the frequency specified.
If it's set to `"0s"`, the Git repository is pulled once on init.

{{< admonition type="warning" >}}
Pulling hosted Git repositories too often can result in throttling.
{{< /admonition >}}

## Blocks

The following blocks are supported inside the definition of `import.git`:

Hierarchy  | Block          | Description                                                | Required
-----------|----------------|------------------------------------------------------------|---------
basic_auth | [basic_auth][] | Configure basic_auth for authenticating to the repository. | no
ssh_key    | [ssh_key][]    | Configure an SSH Key for authenticating to the repository. | no

### basic_auth block

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### ssh_key block

Name         | Type     | Description                       | Default | Required
-------------|----------|-----------------------------------|---------|---------
`username`   | `string` | SSH username.                     |         | yes
`key`        | `secret` | SSH private key.                  |         | no
`key_file`   | `string` | SSH private key path.             |         | no
`passphrase` | `secret` | Passphrase for SSH key if needed. |         | no

## Examples

This example imports custom components from a Git repository and uses a custom component to add two numbers:

```river
import.git "math" {
  repository = "https://github.com/wildum/module.git"
  revision   = "master"
  path       = "math.river"
}

math.add "default" {
  a = 15
  b = 45
}
```

[basic_auth]: #basic_auth-block
[ssh_key]: #ssh_key-block

{{% docs/reference %}}
[module]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/modules"
[module]:"/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/modules"
{{% /docs/reference %}}
