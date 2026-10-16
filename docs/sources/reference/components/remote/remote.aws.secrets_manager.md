---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote/remote.aws.secrets_manager/
description: Learn about remote.aws.secrets_manager
labels:
  stage: experimental
  products:
    - oss
review_date: 2026-10-01
title: remote.aws.secrets_manager
---

# `remote.aws.secrets_manager`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`remote.aws.secrets_manager` reads a secret from [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) and exposes it to other components.
`remote.aws.secrets_manager` reads the secret again at the interval that `poll_frequency` sets.

If the secret is a JSON object, each top-level field is available in the `data` export.
The raw secret string is always available in the `content` export.

You can specify multiple `remote.aws.secrets_manager` components by giving them different labels.

By default, `remote.aws.secrets_manager` uses the [AWS SDK default credential chain](https://docs.aws.amazon.com/sdkref/latest/guide/standardized-credentials.html).
This includes environment variables, shared configuration files, AWS Identity and Access Management (IAM) Roles for Service Accounts (IRSA), and Amazon Elastic Compute Cloud (Amazon EC2) or Amazon Elastic Container Service (Amazon ECS) instance roles.
Use the `client` block to set static credentials, a region, a profile, a custom endpoint, or a role to assume.

The identity that {{< param "PRODUCT_NAME" >}} uses needs these IAM permissions:

- `secretsmanager:GetSecretValue` on the secret.
- `kms:Decrypt` on the key, if the secret uses a customer managed AWS Key Management Service (AWS KMS) key.
- `sts:AssumeRole` on the role, if you use the `assume_role` block.

## Usage

```alloy
remote.aws.secrets_manager "<LABEL>" {
  secret_id = "<SECRET_NAME_OR_ARN>"
}
```

## Arguments

You can use the following arguments with `remote.aws.secrets_manager`:

| Name             | Type       | Description                                          | Default | Required |
| ---------------- | ---------- | ---------------------------------------------------- | ------- | -------- |
| `secret_id`      | `string`   | The name or ARN of the secret.                       |         | yes      |
| `poll_frequency` | `duration` | How often to poll the secret for changes.            | `"1h"`  | no       |
| `version_id`     | `string`   | The unique identifier of the secret version to read. |         | no       |
| `version_stage`  | `string`   | The staging label of the secret version to read.     |         | no       |

Set the `poll_frequency` argument to `"0s"` to read the secret only when `remote.aws.secrets_manager` starts and when its arguments change.
A reload that doesn't change the arguments of `remote.aws.secrets_manager` doesn't read the secret again.
Otherwise, `poll_frequency` must be at least `"1m"`.

With the default `poll_frequency`, a rotated secret can take up to one hour to reach the components that use it.
If you rotate the secret, use the alternating-users rotation strategy, so that the old credentials work until the next poll.
You can also set a shorter `poll_frequency`.
Refer to [Rotation strategies](https://docs.aws.amazon.com/secretsmanager/latest/userguide/rotation-strategy.html) for more information.

You can set only one of the `version_id` and `version_stage` arguments.
If you set neither, `remote.aws.secrets_manager` reads the version with the `AWSCURRENT` staging label.

{{< admonition type="note" >}}
AWS charges for each Secrets Manager API call.
Each `remote.aws.secrets_manager` component makes one call at each poll.
It also reads the secret at startup and when its arguments change.
The AWS SDK makes up to 3 attempts for each call.
If you use the `assume_role` block, the component also calls AWS Security Token Service (AWS STS) when the role credentials expire.
Refer to [AWS Secrets Manager pricing](https://aws.amazon.com/secrets-manager/pricing/) for more information.
{{< /admonition >}}

## Blocks

You can use the following blocks with `remote.aws.secrets_manager`:

{{< docs/alloy-config >}}

| Block                                   | Description                                   | Required |
| --------------------------------------- | --------------------------------------------- | -------- |
| [`client`][client]                      | Options for the connection to AWS.            | no       |
| `client` > [`assume_role`][assume_role] | Assume an IAM role before reading the secret. | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `client` > `assume_role` refers to an `assume_role` block defined inside a `client` block.

[client]: #client
[assume_role]: #assume_role

{{< /docs/alloy-config >}}

### `client`

The `client` block configures the connection to AWS.

| Name       | Type               | Description                                                                                | Default | Required |
| ---------- | ------------------ | ------------------------------------------------------------------------------------------ | ------- | -------- |
| `endpoint` | `string`           | A custom URL for the Secrets Manager API, for example a VPC endpoint.                      |         | no       |
| `key`      | `string`           | An AWS access key ID.                                                                      |         | no       |
| `profile`  | `string`           | The shared configuration profile to use. Overrides the `AWS_PROFILE` environment variable. |         | no       |
| `region`   | `string`           | The AWS region. Overrides the environment and instance metadata.                           |         | no       |
| `secret`   | [`secret`][secret] | An AWS secret access key.                                                                  |         | no       |

You must set both the `key` and `secret` arguments, or neither.
If you set them, they replace the default credential chain.

`remote.aws.secrets_manager` finds the region in this order:

1. The `region` argument.
1. The `AWS_REGION` or `AWS_DEFAULT_REGION` environment variable.
1. The region in the shared configuration profile. The profile comes from the `profile` argument or, if you don't set it, from the `AWS_PROFILE` environment variable.
1. The Amazon EC2 instance metadata service.

If none of these gives a region, the component returns an error.

Each read, including all SDK attempts, must finish within 30 seconds.
The AWS SDK makes up to 3 attempts for each call by default.
The SDK reads these environment variables:

- `AWS_MAX_ATTEMPTS` and `AWS_RETRY_MODE` set the retry behavior.
- `HTTPS_PROXY` and `NO_PROXY` set the proxy.
- `AWS_CA_BUNDLE` sets a custom certificate authority.
- `AWS_PROFILE` sets the shared configuration profile. The `profile` argument overrides it.

Refer to the [AWS SDK settings reference](https://docs.aws.amazon.com/sdkref/latest/guide/settings-reference.html) for more information.

### `assume_role`

The `assume_role` block makes `remote.aws.secrets_manager` call AWS STS `AssumeRole` with its base credentials.
`remote.aws.secrets_manager` then reads the secret with the credentials of the assumed role.
The AWS STS call uses the default regional AWS STS endpoint.
The `endpoint` argument in the `client` block doesn't change the AWS STS endpoint.

| Name           | Type               | Description                                          | Default              | Required |
| -------------- | ------------------ | ---------------------------------------------------- | -------------------- | -------- |
| `role_arn`     | `string`           | The ARN of the IAM role to assume.                   |                      | yes      |
| `external_id`  | [`secret`][secret] | The external ID that the role trust policy requires. |                      | no       |
| `session_name` | `string`           | The name of the role session.                        | `"alloy-<HOSTNAME>"` | no       |

The default `session_name` is `alloy-` followed by the host name.
Characters that AWS STS doesn't allow are replaced by `-`, and the name has at most 64 characters.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type               | Description                                             |
| --------- | ------------------ | ------------------------------------------------------- |
| `content` | [`secret`][secret] | The raw secret string.                                  |
| `data`    | `map(secret)`      | The top-level fields of a secret that is a JSON object. |

`remote.aws.secrets_manager` fills `data` with these rules:

- If the secret isn't a JSON object, `data` is empty.
- A string field value is exported as the plain string.
- A number, boolean, `null`, object, or array field value is exported as its JSON text.
- If the JSON object has duplicate keys, the last value wins.

`remote.aws.secrets_manager` doesn't support binary secrets.

## Component health

`remote.aws.secrets_manager` is reported as healthy if the most recent read of the secret was successful.
If a read fails, `remote.aws.secrets_manager` keeps the last values it read and is reported as unhealthy.
The health message then ends with the time of the last successful read, which shows how old the exported values are.

If the first read fails, the component doesn't start and doesn't retry by itself.
Fix the cause, then reload the configuration.

After a failed poll, the component retries after the `poll_frequency` interval or after 1 minute, whichever is shorter.
After a successful read, the component returns to the `poll_frequency` schedule.

If a read with new arguments fails after a configuration change, the component keeps the previous arguments and the last values it read, and it keeps polling with the previous arguments.
{{< param "PRODUCT_NAME" >}} reports the component as unhealthy, with the configuration error, until you load a configuration that works.
This includes restoring the previous configuration.
If the arguments come from the exports of another component that change often, each failed change runs the read again and restarts the poll schedule.

## Debug information

`remote.aws.secrets_manager` exposes debug information about the secret and its reads:

- The ARN of the secret.
- The version ID and the staging labels of the version that it read last.
- The creation date of that version.
- The most recent time that `remote.aws.secrets_manager` read the secret successfully.
- The error of the most recent read, if it failed.
- The time of the next scheduled poll, if there is one.

This information contains no secret values.

## Debug metrics

The following Prometheus metrics are exposed:

| Name                                                              | Type      | Description                                                                    |
| ----------------------------------------------------------------- | --------- | ------------------------------------------------------------------------------ |
| `remote_aws_secrets_manager_fetches_total`                        | `counter` | Total number of secret fetches, with a `result` label of `success` or `error`. |
| `remote_aws_secrets_manager_timestamp_last_accessed_unix_seconds` | `gauge`   | The last successful access in Unix seconds.                                    |

## Examples

This example reads a JSON secret with the fields `username` and `password` and uses them for basic authentication.
The `username` argument takes a string, so the example uses [convert.nonsensitive][] to convert the secret value:

```alloy
remote.aws.secrets_manager "mimir" {
  secret_id = "prod/mimir"

  client {
    region = "us-east-1"
  }
}

prometheus.remote_write "default" {
  endpoint {
    url = "https://mimir.example.com/api/v1/push"

    basic_auth {
      username = convert.nonsensitive(remote.aws.secrets_manager.mimir.data.username)
      password = remote.aws.secrets_manager.mimir.data.password
    }
  }
}
```

[convert.nonsensitive]: ../../../stdlib/convert/
[secret]: ../../../../get-started/expressions/types_and_values/#secrets
