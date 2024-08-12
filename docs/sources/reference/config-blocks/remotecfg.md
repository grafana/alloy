---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/remotecfg/
description: Learn about the remotecfg configuration block
menuTitle: remotecfg
title: remotecfg block
---

<span class="badge docs-labels__stage docs-labels__item">Public preview</span>

# remotecfg block

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`remotecfg` is an optional configuration block that enables {{< param "PRODUCT_NAME" >}} to fetch and load the configuration from a remote endpoint.
`remotecfg` is specified without a label and can only be provided once per configuration file.

The [API definition][] for managing and fetching configuration that the `remotecfg` block uses is available under the Apache 2.0 license.

## Example

```alloy
remotecfg {
    url = "SERVICE_URL"
    basic_auth {
        username      = "USERNAME"
        password_file = "PASSWORD_FILE"
    }

    id             = constants.hostname
    attributes     = {"cluster" = "dev", "namespace" = "otlp-dev"}
    poll_frequency = "5m"
}
```

## Arguments

The following arguments are supported:

Name             | Type                 | Description                                       | Default     | Required
-----------------|----------------------|---------------------------------------------------|-------------|---------
`url`            | `string`             | The address of the API to poll for configuration. | `""`        | no
`id`             | `string`             | A self-reported ID.                               | `see below` | no
`attributes`     | `map(string)`        | A set of self-reported attributes.                | `{}`        | no
`poll_frequency` | `duration`           | How often to poll the API for new configuration.  | `"1m"`      | no
`name`           | `string`             | A human-readable name for the collector.          | `""`        | no

If the `url` is not set, then the service block is a no-op.

If not set, the self-reported `id` that {{< param "PRODUCT_NAME" >}} uses is a randomly generated, anonymous unique ID (UUID) that is stored as an `alloy_seed.json` file in the {{< param "PRODUCT_NAME" >}} storage path so that it can persist across restarts.
You can use the `name` field to set another human-friendly identifier for the specific {{< param "PRODUCT_NAME" >}} instance.

The `id` and `attributes` fields are used in the periodic request sent to the
remote endpoint so that the API can decide what configuration to serve.

The `attribute` map keys can include any custom value except the reserved prefix `collector.`.
The reserved label prefix is for automatic system attributes.
You can't override this prefix.

* `collector.os`: The operating system where {{< param "PRODUCT_NAME" >}} is running.
* `collector.version`: The version of {{< param "PRODUCT_NAME" >}}.

The `poll_frequency` must be set to at least `"10s"`.

## Blocks

The following blocks are supported inside the definition of `remotecfg`:

Hierarchy           | Block             | Description                                              | Required
--------------------|-------------------|----------------------------------------------------------|---------
basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting.
For example, `oauth2 > tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

[API definition]: https://github.com/grafana/alloy-remote-config
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block
