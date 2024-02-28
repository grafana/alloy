---
aliases:
- /docs/agent/latest/flow/reference/components/remote.vault/
- /docs/grafana-cloud/agent/flow/reference/components/remote.vault/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/remote.vault/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/remote.vault/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/remote.vault/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/remote.vault/
description: Learn about remote.vault
title: remote.vault
---

# remote.vault

`remote.vault` connects to a [HashiCorp Vault][Vault] server to retrieve secrets.
It can retrieve a secret using the [KV v2][] secrets engine.

Multiple `remote.vault` components can be specified by giving them different
labels.

[Vault]: https://www.vaultproject.io/
[KV v2]: https://www.vaultproject.io/docs/secrets/kv/kv-v2

## Usage

```river
remote.vault "LABEL" {
  server = "VAULT_SERVER"
  path   = "VAULT_PATH"

  // Alternatively, use one of the other auth.* mechanisms.
  auth.token {
    token = "AUTH_TOKEN"
  }
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`server` | `string` | The Vault server to connect to. | | yes
`namespace` | `string` | The Vault namespace to connect to (Vault Enterprise only). | | no
`path` | `string` | The path to retrieve a secret from. | | yes
`reread_frequency` | `duration` | Rate to re-read keys. | `"0s"` | no

Tokens with a lease will be automatically renewed roughly two-thirds through
their lease duration. If the leased token isn't renewable, or renewing the
lease fails, the token will be re-read.

All tokens, regardless of whether they have a lease, are automatically reread
at a frequency specified by the `reread_frequency` argument. Setting
`reread_frequency` to `"0s"` (the default) disables this behavior.

## Blocks

The following blocks are supported inside the definition of `remote.vault`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
client_options | [client_options][] | Options for the Vault client. | no
auth.token | [auth.token][] | Authenticate to Vault with a token. | no
auth.approle | [auth.approle][] | Authenticate to Vault using AppRole. | no
auth.aws | [auth.aws][] | Authenticate to Vault using AWS. | no
auth.azure | [auth.azure][] | Authenticate to Vault using Azure. | no
auth.gcp | [auth.gcp][] | Authenticate to Vault using GCP. | no
auth.kubernetes | [auth.kubernetes][] | Authenticate to Vault using Kubernetes. | no
auth.ldap | [auth.ldap][] | Authenticate to Vault using LDAP. | no
auth.userpass | [auth.userpass][] | Authenticate to Vault using a username and password. | no
auth.custom | [auth.custom][] | Authenticate to Vault with custom authentication. | no

Exactly one `auth.*` block **must** be provided, otherwise the component will
fail to load.

[client_options]: #client_options-block
[auth.token]: #authtoken-block
[auth.approle]: #authapprole-block
[auth.aws]: #authaws-block
[auth.azure]: #authazure-block
[auth.gcp]: #authgcp-block
[auth.kubernetes]: #authkubernetes-block
[auth.ldap]: #authldap-block
[auth.userpass]: #authuserpass-block
[auth.custom]: #authcustom-block

### client_options block

The `client_options` block customizes the connection to vault.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`min_retry_wait` | `duration` | Minimum time to wait before retrying failed requests. | `"1000ms"` | no
`max_retry_wait` | `duration` | Maximum time to wait before retrying failed requests. | `"1500ms"` | no
`max_retries` | `int` | Maximum number of times to retry after a 5xx error. | `2` | no
`timeout` | `duration` | Maximum time to wait before a request times out. | `"60s"` | no

Requests which fail due to server errors (HTTP 5xx error codes) can be retried.
The `max_retries` argument specifies how many times to retry failed requests.
The `min_retry_wait` and `max_retry_wait` arguments specify how long to wait
before retrying. The wait period starts at `min_retry_wait` and exponentially
increases up to `max_retry_wait`.

Other types of failed requests, including HTTP 4xx error codes, are not
retried.

If the `max_retries` argument is set to `0`, failed requests are not retried.

### auth.token block

The `auth.token` block authenticates each request to Vault using a
token.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`token` | `secret` | Authentication token to use. | | yes

### auth.approle block

The `auth.token` block auhenticates to Vault using the [AppRole auth
method][AppRole].

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`role_id` | `string` | Role ID to authenticate as. | | yes
`secret` | `secret` | Secret to authenticate with. | | yes
`wrapping_token` | `bool` | Whether to [unwrap][] the token. | `false` | no
`mount_path` | `string` | Mount path for the login. | `"approle"` | no

[AppRole]: https://www.vaultproject.io/docs/auth/approle
[unwrap]: https://www.vaultproject.io/docs/concepts/response-wrapping

### auth.aws block

The `auth.aws` block authenticates to Vault using the [AWS auth method][AWS].

Credentials used to connect to AWS are specified by the environment variables
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION`. The
environment variable `AWS_SHARED_CREDENTIALS_FILE` may be specified to use a
credentials file instead.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`type` | `string` | Mechanism to authenticate against AWS with. | | yes
`region` | `string` | AWS region to connect to. | `"us-east-1"` | no
`role` | `string` | Overrides the inferred role name inferred. | `""` | no
`iam_server_id_header` | `string` | Configures a `X-Vault-AWS-IAM-Server-ID` header. | `""` | no
`ec2_signature_type` | `string` | Signature to use when authenticating against EC2. | `"pkcs7"` | no
`mount_path` | `string` | Mount path for the login. | `"aws"` | no

The `type` argument must be set to one of `"ec2"` or `"iam"`.

The `iam_server_id_header` argument is required used when `type` is set to
`"iam"`.

If the `region` argument is explicitly set to an empty string `""`, the region
to connect to will be inferred using an API call to the EC2 metadata service.

The `ec2_signature_type` argument configures the signature to use when
authenticating against EC2. It only applies when `type` is set to `"ec2"`.
`ec2_signature_type` must be set to either `"identity"` or `"pkcs7"`.

[AWS]: https://www.vaultproject.io/docs/auth/aws

### auth.azure block

The `auth.azure` block authenticates to Vault using the [Azure auth
method][Azure].

Credentials are retrieved for the running Azure VM using Managed Identities for
Azure Resources.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`role` | `string` | Role name to authenticate as. | | yes
`resource_url` | `string` | Resource URL to include with authentication request. | | no
`mount_path` | `string` | Mount path for the login. | `"azure"` | no

[Azure]: https://www.vaultproject.io/docs/auth/azure

### auth.gcp block

The `auth.gcp` block authenticates to Vault using the [GCP auth method][GCP].

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`role` | `string` | Role name to authenticate as. | | yes
`type` | `string` | Mechanism to authenticate against GCP with | | yes
`iam_service_account` | `string` | IAM service account name to use. | | no
`mount_path` | `string` | Mount path for the login. | `"gcp"` | no

The `type` argument must be set to `"gce"` or `"iam"`. When `type` is `"gce"`,
credentials are retrieved using the metadata service on GCE VMs. When `type` is
`"iam"`, credentials are retrieved from the file that the
`GOOGLE_APPLICATION_CREDENTIALS` environment variable points to.

When `type` is `"iam"`, the `iam_service_account` argument determines what
service account name to use.

[GCP]: https://www.vaultproject.io/docs/auth/gcp

### auth.kubernetes block

The `auth.kubernetes` block authenticates to Vault using the [Kubernetes auth
method][Kubernetes].

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`role` | `string` | Role name to authenticate as. | | yes
`service_account_file` | `string` | Override service account token file to use. | | no
`mount_path` | `string` | Mount path for the login. | `"kubernetes"` | no

When `service_account_file` is not specified, the JWT token to authenticate
with is retrieved from `/var/run/secrets/kubernetes.io/serviceaccount/token`.

[Kubernetes]: https://www.vaultproject.io/docs/auth/kubernetes

### auth.ldap block

The `auth.ldap` block authenticates to Vault using the [LDAP auth
method][LDAP].

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | LDAP username to authenticate as. | | yes
`password` | `secret` | LDAP passsword for the user. | | yes
`mount_path` | `string` | Mount path for the login. | `"ldap"` | no

[LDAP]: https://www.vaultproject.io/docs/auth/ldap

### auth.userpass block

The `auth.userpass` block authenticates to Vault using the [UserPass auth
method][UserPass].

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | Username to authenticate as. | | yes
`password` | `secret` | Passsword for the user. | | yes
`mount_path` | `string` | Mount path for the login. | `"userpass"` | no

[UserPass]: https://www.vaultproject.io/docs/auth/userpass

### auth.custom block

The `auth.custom` blocks allows authenticating against Vault using an arbitrary
authentication path like `auth/customservice/login`.

Using `auth.custom` is equivalent to calling `vault write PATH DATA` on the
command line.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`path` | `string` | Path to write to for creating an authentication token. | yes
`data` | `map(secret)` | Authentication data. | yes

All values in the `data` attribute are considered secret, even if they contain
nonsensitive information like usernames.

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`data` | `map(secret)` | Data from the secret obtained from Vault.

The `data` field contains a mapping from data field names to values. There will
be one mapping for each string-like field stored in the Vault secret.

Note that Vault permits secret engines to store arbitrary data within the
key-value pairs for a secret. The `remote.vault` component is only able to use
values which are strings or can be converted to strings. Keys with non-string
values will be ignored and omitted from the `data` field.

If an individual key stored in `data` does not hold sensitive data, it can be
converted into a string using [the `nonsensitive` function][nonsensitive]:

```river
nonsensitive(remote.vault.LABEL.data.KEY_NAME)
```

Using `nonsensitive` allows for using the exports of `remote.vault` for
attributes in components that do not support secrets.

[nonsensitive]: {{< relref "../stdlib/nonsensitive.md" >}}

## Component health

`remote.vault` will be reported as unhealthy if the latest reread or renewal of
secrets was unsuccessful.

## Debug information

`remote.vault` exposes debug information for the authentication token and
secret around:

* The latest request ID used for retrieving or renewing the token.
* The most recent time when the token was retrieved or renewed.
* The expiration time for the token (if applicable).
* Whether the token is renewable.
* Warnings from Vault from when the token was retrieved.

## Debug metrics

`remote.vault` exposes the following metrics:

* `remote_vault_auth_total` (counter): Total number of times the component
  authenticated to Vault.
* `remote_vault_secret_reads_total` (counter): Total number of times the secret
  was read from Vault.
* `remote_vault_auth_lease_renewal_total` (counter): Total number of times the
  component renewed its authentication token lease.
* `remote_vault_secret_lease_renewal_total` (counter): Total number of times
  the component renewed its secret token lease.

## Example

```river
local.file "vault_token" {
  filename  = "/var/data/vault_token"
  is_secret = true
}

remote.vault "remote_write" {
  server = "https://prod-vault.corporate.internal"
  path   = "secret/prometheus/remote_write"

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
