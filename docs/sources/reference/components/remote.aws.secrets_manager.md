---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote.aws.secrets_manager/
description: Learn about remote.aws.secrets_manager
title: remote.aws.secrets_manager
---

# remote.aws.secret_manager

`remote.aws.secrets_manager` securely exposes value of secrets located in [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) to other components.
The secret would be fetched one time only at startup. Restart Alloy if you have updated to new value and would like 
the component to fetch the latest version.

Multiple `remote.aws.secrets_manager` components can be specified using different name
labels. By default, [AWS environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html) are used to authenticate against AWS. The `key` and `secret` arguments inside `client` blocks can be used to provide custom authentication.

## Usage

```alloy
remote.aws.secrets_manager "LABEL" {
  id = AWS_SECRETS_MANAGER_SECRET_ID
}
```

## Arguments

The following arguments are supported:

Name             | Type       | Description                                                              | Default | Required
-----------------|------------|--------------------------------------------------------------------------|---------|---------
`id`             | `string`   | Secret ID.                                                               |         | yes

## Blocks

Hierarchy | Name       | Description                                        | Required
----------|------------|----------------------------------------------------|---------
client    | [client][] | Additional options for configuring the AWS client. | no

[client]: #client-block

### client block

The `client` block customizes options to connect to the AWS server.

Name             | Type     | Description                                                                             | Default | Required
-----------------|----------|-----------------------------------------------------------------------------------------|---------|---------
`key`            | `string` | Used to override default access key.                                                    |         | no
`secret`         | `secret` | Used to override default secret value.                                                  |         | no
`disable_ssl`    | `bool`   | Used to disable SSL, generally used for testing.                                        |         | no
`region`         | `string` | Used to override default region.                                                        |         | no
`signing_region` | `string` | Used to override the signing region when using a custom endpoint.                       |         | no


## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                 | Description                
----------|----------------------|---------------------------------------------------------
`data`    | `map(secret)`        |  Data from the secret obtained from AWS Secrets Manager.

The `data` field contains a mapping from data field names to values.

## Component health

Instances of `remote.aws.secrets_manager` report as healthy if most recent fetch of stored secrets was successful.

## Debug information

`remote.aws.secrets_manager` doesn't expose any component-specific debug information.

## Debug metrics

`remote.aws.secrets_manager` doesn't expose any component-specific debug metrics.

## Example

```alloy
remote.aws.secrets_manager "data" {
  id = "foo"
}

metrics.remote_write "prod" {
  remote_write {
    url = "https://onprem-mimir:9009/api/v1/push"
    basic_auth {
      username = remote.vault.remote_write.data.username
      password = remote.vault.remote_write.data.password
    }
  }
```
