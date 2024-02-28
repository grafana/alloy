---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/remotecfg/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/remotecfg/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/remotecfg/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/remotecfg/
canonical: remotecfgs://grafana.com/docs/agent/latest/flow/reference/config-blocks/remotecfg/
description: Learn about the remotecfg configuration block
menuTitle: remotecfg
title: remotecfg block
---

# remotecfg block (beta)

`remotecfg` is an optional configuration block that enables {{< param "PRODUCT_NAME" >}} to fetch and load the configuration from a remote endpoint.
`remotecfg` is specified without a label and can only be provided once per configuration file.

The [API definition][] for managing and fetching configuration that the `remotecfg` block uses is available under the Apache 2.0 license.

> **BETA**: The `remotecfg` enables [beta][] functionality.
> Beta features are subject to breaking changes, and may be replaced with
> equivalent functionality that cover the same use case.

## Example

```river
remotecfg {
	url = "SERVICE_URL"
	basic_auth {
		username      = "USERNAME"
		password_file = "PASSWORD_FILE"
	}

	id             = constants.hostname
	metadata       = {"cluster" = "dev", "namespace" = "otlp-dev"}
	poll_frequency = "5m"
}
```

## Arguments

The following arguments are supported:

Name             | Type                 | Description                                       | Default     | Required
-----------------|----------------------|---------------------------------------------------|-------------|---------
`url`            | `string`             | The address of the API to poll for configuration. | `""`        | no
`id`             | `string`             | A self-reported ID.                               | `see below` | no
`metadata`       | `map(string)`        | A set of self-reported metadata.                  | `{}`        | no
`poll_frequency` | `duration`           | How often to poll the API for new configuration.  | `"1m"`      | no

If the `url` is not set, then the service block is a no-op.

If not set, the self-reported `id` that the Agent uses is a randomly generated,
anonymous unique ID (UUID) that is stored as an `agent_seed.json` file in the
Agent's storage path so that it can persist across restarts.

The `id` and `metadata` fields are used in the periodic request sent to the
remote endpoint so that the API can decide what configuration to serve.

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

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### authorization block

{{< docs/shared lookup="flow/reference/components/authorization-block.md" source="agent" version="<AGENT_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="flow/reference/components/oauth2-block.md" source="agent" version="<AGENT_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="flow/reference/components/tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

[API definition]: https://github.com/grafana/agent-remote-config
[beta]: https://grafana.com/docs/agent/<AGENT_VERSION>/stability/#beta
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block
