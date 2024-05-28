---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote.aws.secrets_manager/
description: Learn about remote.aws.secrets_manager
title: remote.aws.secrets_manager
---

# remote.aws.secret_manager

`remote.aws.secrets_manager` securely exposes the secrets located in [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) to other components.
By default, the secret is fetched once only at startup. If configured, the secret is polled for changes so that the most recent value is always available. 

{{< admonition type="note" >}}
The polling for changes could incur costs due to frequent API calls.
{{< /admonition >}}

You can specify multiple `remote.aws.secrets_manager` components by giving them different labels.
By default, [AWS environment variables][] are used to authenticate against AWS.
For custom authentication, you can use the `key` and `secret` arguments inside `client` blocks.

 [AWS environment variables]: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html

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
`poll_frequency` | `duration` | How often to poll the API for changes.                                   |         | no

## Blocks

Hierarchy | Name       | Description                                        | Required
----------|------------|----------------------------------------------------|---------
client    | [client][] | Additional AWS client configuration options. | no

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

Instances of `remote.aws.secrets_manager` report as healthy if the most recent fetch of stored secrets was successful.

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
