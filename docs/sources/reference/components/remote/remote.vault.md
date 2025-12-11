---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote/remote.vault/
aliases:
  - ../remote.vault/ # /docs/alloy/latest/reference/components/remote.vault/
description: Learn about remote.vault
labels:
  stage: general-availability
  products:
    - oss
title: remote.vault
---

# `remote.vault`

`remote.vault` connects to a [HashiCorp Vault][Vault] server to retrieve secrets.
It can retrieve a secret using the [KV v2][] secrets engine.

You can specify multiple `remote.vault` components by giving them different labels.

[Vault]: https://www.vaultproject.io/
[KV v2]: https://www.vaultproject.io/docs/secrets/kv/kv-v2

## Usage

```alloy
remote.vault "<LABEL>" {
  server = "<VAULT_SERVER>"
  path   = "<VAULT_PATH>"
  key    = "<VAULT_KEY>"

  // Alternatively, use one of the other auth.* mechanisms.
  auth.token {
    token = "<AUTH_TOKEN>"
  }
}
```

## Arguments

You can use the following arguments with `remote.vault`:

| Name               | Type       | Description                                                | Default | Required |
| ------------------ | ---------- | ---------------------------------------------------------- | ------- | -------- |
| `path`             | `string`   | The path to retrieve a secret from.                        |         | yes      |
| `server`           | `string`   | The Vault server to connect to.                            |         | yes      |
| `namespace`        | `string`   | The Vault namespace to connect to (Vault Enterprise only). |         | no       |
| `key`              | `string`   | The key to retrieve a secret from.                         |         | no       |
| `reread_frequency` | `duration` | Rate to re-read keys.                                      | `"0s"`  | no       |

Tokens with a lease are automatically renewed roughly two-thirds through their lease duration.
If the leased token isn't renewable, or renewing the lease fails, the token is re-read.

All tokens, regardless of whether they have a lease, are automatically reread at a frequency specified by the `reread_frequency` argument.
Setting `reread_frequency` to `"0s"` (the default) disables this behavior.

## Blocks

You can use the following blocks with `remote.vault`:

| Block                                | Description                                          | Required |
| ------------------------------------ | ---------------------------------------------------- | -------- |
| [`auth.approle`][auth.approle]       | Authenticate to Vault using AppRole.                 | no       |
| [`auth.aws`][auth.aws]               | Authenticate to Vault using AWS.                     | no       |
| [`auth.azure`][auth.azure]           | Authenticate to Vault using Azure.                   | no       |
| [`auth.custom`][auth.custom]         | Authenticate to Vault with custom authentication.    | no       |
| [`auth.gcp`][auth.gcp]               | Authenticate to Vault using GCP.                     | no       |
| [`auth.kubernetes`][auth.kubernetes] | Authenticate to Vault using Kubernetes.              | no       |
| [`auth.ldap`][auth.ldap]             | Authenticate to Vault using LDAP.                    | no       |
| [`auth.token`][auth.token]           | Authenticate to Vault with a token.                  | no       |
| [`auth.userpass`][auth.userpass]     | Authenticate to Vault using a username and password. | no       |
| [`client_options`][client_options]   | Options for the Vault client.                        | no       |

Exactly one `auth.*` block **must** be provided, otherwise the component will fail to load.

[auth.approle]: #authapprole
[auth.aws]: #authaws
[auth.azure]: #authazure
[auth.custom]: #authcustom
[auth.gcp]: #authgcp
[auth.kubernetes]: #authkubernetes
[auth.ldap]: #authldap
[auth.token]: #authtoken
[auth.userpass]: #authuserpass
[client_options]: #client_options

### `auth.approle`

The `auth.approle` block authenticates to Vault using the [AppRole auth method][AppRole].

| Name             | Type     | Description                      | Default     | Required |
| ---------------- | -------- | -------------------------------- | ----------- | -------- |
| `role_id`        | `string` | Role ID to authenticate as.      |             | yes      |
| `secret`         | `secret` | Secret to authenticate with.     |             | yes      |
| `mount_path`     | `string` | Mount path for the login.        | `"approle"` | no       |
| `wrapping_token` | `bool`   | Whether to [unwrap][] the token. | `false`     | no       |

[AppRole]: https://www.vaultproject.io/docs/auth/approle
[unwrap]: https://www.vaultproject.io/docs/concepts/response-wrapping

### `auth.aws`

The `auth.aws` block authenticates to Vault using the [AWS auth method][AWS].

Credentials used to connect to AWS are specified by the environment variables `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION`.
The environment variable `AWS_SHARED_CREDENTIALS_FILE` may be specified to use a credentials file instead.

| Name                   | Type     | Description                                       | Default       | Required |
| ---------------------- | -------- | ------------------------------------------------- | ------------- | -------- |
| `type`                 | `string` | Mechanism to authenticate against AWS with.       |               | yes      |
| `ec2_signature_type`   | `string` | Signature to use when authenticating against EC2. | `"pkcs7"`     | no       |
| `iam_server_id_header` | `string` | Configures a `X-Vault-AWS-IAM-Server-ID` header.  | `""`          | no       |
| `mount_path`           | `string` | Mount path for the login.                         | `"aws"`       | no       |
| `region`               | `string` | AWS region to connect to.                         | `"us-east-1"` | no       |
| `role`                 | `string` | Overrides the inferred role name inferred.        | `""`          | no       |

The `type` argument must be set to one of `"ec2"` or `"iam"`.

The `iam_server_id_header` argument is required used when `type` is set to `"iam"`.

If the `region` argument is explicitly set to an empty string `""`, the region to connect to will be inferred using an API call to the EC2 metadata service.

The `ec2_signature_type` argument configures the signature to use when authenticating against EC2.
It only applies when `type` is set to `"ec2"`.
`ec2_signature_type` must be set to either `"identity"` or `"pkcs7"`.

[AWS]: https://www.vaultproject.io/docs/auth/aws

### `auth.azure`

The `auth.azure` block authenticates to Vault using the [Azure auth method][Azure].

Credentials are retrieved for the running Azure VM using Managed Identities for Azure Resources.

| Name           | Type     | Description                                          | Default   | Required |
| -------------- | -------- | ---------------------------------------------------- | --------- | -------- |
| `role`         | `string` | Role name to authenticate as.                        |           | yes      |
| `resource_url` | `string` | Resource URL to include with authentication request. |           | no       |
| `mount_path`   | `string` | Mount path for the login.                            | `"azure"` | no       |

[Azure]: https://www.vaultproject.io/docs/auth/azure

### `auth.custom`

The `auth.custom` blocks allows authenticating against Vault using an arbitrary authentication path like `auth/customservice/login`.

Using `auth.custom` is equivalent to calling `vault write PATH DATA` on the command line.


| Name         | Type            | Description                                            | Default | Required |
|--------------|-----------------|--------------------------------------------------------| ------- |----------|
| `path`       | `string`        | Path to write to for creating an authentication token. |         | yes      |
| `data`       | `map(secret)`   | Authentication data.                                   |         | yes      |
| `namespace`  | `string`        | The namespace to authenticate to.                      |         | no       |

All values in the `data` attribute are considered secret, even if they contain nonsensitive information like usernames.

With Vault Enterprise, you can authenticate against a parent namespace while storing secrets in a child namespace.
By specifying the namespace argument in `auth.custom`, you can authenticate to a namespace different from the one used to retrieve the secrets.

You can also define Vault environment variables, which the clients used by {{< param "PRODUCT_NAME" >}} will automatically load.
This approach allows you to use certificate-based authentication by setting the `VAULT_CACERT` and `VAULT_CAPATH` environment variables.
Refer to the [Vault Environment variables](https://developer.hashicorp.com/vault/docs/commands#configure-environment-variables) documentation for more information.
### `auth.gcp`

The `auth.gcp` block authenticates to Vault using the [GCP auth method][GCP].

| Name                  | Type     | Description                                | Default | Required |
| --------------------- | -------- | ------------------------------------------ | ------- | -------- |
| `role`                | `string` | Role name to authenticate as.              |         | yes      |
| `type`                | `string` | Mechanism to authenticate against GCP with |         | yes      |
| `iam_service_account` | `string` | IAM service account name to use.           |         | no       |
| `mount_path`          | `string` | Mount path for the login.                  | `"gcp"` | no       |

The `type` argument must be set to `"gce"` or `"iam"`. When `type` is `"gce"`, credentials are retrieved using the metadata service on GCE VMs.
When `type` is `"iam"`, credentials are retrieved from the file that the `GOOGLE_APPLICATION_CREDENTIALS` environment variable points to.

When `type` is `"iam"`, the `iam_service_account` argument determines what service account name to use.

[GCP]: https://www.vaultproject.io/docs/auth/gcp

### `auth.kubernetes`

The `auth.kubernetes` block authenticates to Vault using the [Kubernetes auth method][Kubernetes].

| Name                   | Type     | Description                                 | Default        | Required |
| ---------------------- | -------- | ------------------------------------------- | -------------- | -------- |
| `role`                 | `string` | Role name to authenticate as.               |                | yes      |
| `service_account_file` | `string` | Override service account token file to use. |                | no       |
| `mount_path`           | `string` | Mount path for the login.                   | `"kubernetes"` | no       |

When `service_account_file` is not specified, the JWT token to authenticate with is retrieved from `/var/run/secrets/kubernetes.io/serviceaccount/token`.

[Kubernetes]: https://www.vaultproject.io/docs/auth/kubernetes

### `auth.ldap`

The `auth.ldap` block authenticates to Vault using the [LDAP auth method][LDAP].

| Name         | Type     | Description                       | Default  | Required |
| ------------ | -------- | --------------------------------- | -------- | -------- |
| `username`   | `string` | LDAP username to authenticate as. |          | yes      |
| `password`   | `secret` | LDAP password for the user.       |          | yes      |
| `mount_path` | `string` | Mount path for the login.         | `"ldap"` | no       |

[LDAP]: https://www.vaultproject.io/docs/auth/ldap

### `auth.token`

The `auth.token` block authenticates each request to Vault using a token.

| Name    | Type     | Description                  | Default | Required |
| ------- | -------- | ---------------------------- | ------- | -------- |
| `token` | `secret` | Authentication token to use. |         | yes      |

### `auth.userpass`

The `auth.userpass` block authenticates to Vault using the [UserPass auth method][UserPass].

| Name         | Type     | Description                  | Default      | Required |
| ------------ | -------- | ---------------------------- | ------------ | -------- |
| `username`   | `string` | Username to authenticate as. |              | yes      |
| `password`   | `secret` | Password for the user.       |              | yes      |
| `mount_path` | `string` | Mount path for the login.    | `"userpass"` | no       |

[UserPass]: https://www.vaultproject.io/docs/auth/userpass

### `client_options`

The `client_options` block customizes the connection to vault.

| Name             | Type       | Description                                           | Default    | Required |
| ---------------- | ---------- | ----------------------------------------------------- | ---------- | -------- |
| `min_retry_wait` | `duration` | Minimum time to wait before retrying failed requests. | `"1000ms"` | no       |
| `max_retry_wait` | `duration` | Maximum time to wait before retrying failed requests. | `"1500ms"` | no       |
| `max_retries`    | `int`      | Maximum number of times to retry after a 5xx error.   | `2`        | no       |
| `timeout`        | `duration` | Maximum time to wait before a request times out.      | `"60s"`    | no       |

Requests which fail due to server errors (HTTP 5xx error codes) can be retried.
The `max_retries` argument specifies how many times to retry failed requests.
The `min_retry_wait` and `max_retry_wait` arguments specify how long to wait before retrying.
The wait period starts at `min_retry_wait` and exponentially increases up to `max_retry_wait`.

Other types of failed requests, including HTTP 4xx error codes, aren't retried.

If the `max_retries` argument is set to `0`, failed requests aren't retried.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name   | Type          | Description                               |
| ------ | ------------- | ----------------------------------------- |
| `data` | `map(secret)` | Data from the secret obtained from Vault. |

The `data` field contains a mapping from data field names to values.
There is one mapping for each string-like field stored in the Vault secret.

Vault permits secret engines to store arbitrary data within the key-value pairs for a secret.
The `remote.vault` component is only able to use values which are strings or can be converted to strings.
Keys with non-string values are ignored and omitted from the `data` field.

If an individual key stored in `data` doesn't hold sensitive data, it can be converted into a string using [the `nonsensitive` function][convert.nonsensitive]:

```alloy
convert.nonsensitive(remote.vault.LABEL.data.KEY_NAME)
```

Using `convert.nonsensitive` allows for using the exports of `remote.vault` for attributes in components that don't support secrets.

[convert.nonsensitive]: ../../../stdlib/convert/

## Component health

`remote.vault` is reported as unhealthy if the latest reread or renewal of secrets was unsuccessful.

## Debug information

`remote.vault` exposes debug information for the authentication token and secret around:

* The latest request ID used for retrieving or renewing the token.
* The most recent time when the token was retrieved or renewed.
* The expiration time for the token (if applicable).
* Whether the token is renewable.
* Warnings from Vault from when the token was retrieved.

## Debug metrics

`remote.vault` exposes the following metrics:

* `remote_vault_auth_total` (counter): Total number of times the component authenticated to Vault.
* `remote_vault_secret_reads_total` (counter): Total number of times the secret was read from Vault.
* `remote_vault_auth_lease_renewal_total` (counter): Total number of times the component renewed its authentication token lease.
* `remote_vault_secret_lease_renewal_total` (counter): Total number of times the component renewed its secret token lease.

## Example

```alloy
local.file "vault_token" {
  filename  = "/var/data/vault_token"
  is_secret = true
}

remote.vault "remote_write" {
  server = "https://prod-vault.corporate.internal"
  path   = "secret"
  key    = "prometheus/remote_write"

  auth.token {
    token = local.file.vault_token.content
  }
}

metrics.remote_write "prod" {
  remote_write {
    url = "https://onprem-mimir:9009/api/v1/push"
    basic_auth {
      username = remote.vault.remote_write.data.username
      password = remote.vault.remote_write.data.password
    }
  }
}
```
