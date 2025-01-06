---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.secretfilter/
description: Learn about loki.secretfilter
title: loki.secretfilter
labels:
  stage: experimental
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# loki.secretfilter

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`loki.secretfilter` receives log entries and redacts sensitive information from them, such as secrets.
The detection is based on regular expression patterns, defined in the [Gitleaks configuration file][gitleaks] embedded within the component.
`loki.secretfilter` can also use a custom configuration file based on the Gitleaks configuration file structure.

{{< admonition type="caution" >}}
Personally Identifiable Information (PII) isn't currently in scope and some secrets could remain undetected.
This component may generate false positives or redact too much.
Don't rely solely on this component to redact sensitive information.
{{< /admonition >}}

[gitleaks]: https://github.com/gitleaks/gitleaks/blob/master/config/gitleaks.toml

## Usage

```alloy
loki.secretfilter "<LABEL>" {
    forward_to = <RECEIVER_LIST>
}
```

## Arguments

`loki.secretfilter` supports the following arguments:

Name                     | Type                 | Description                                     | Default                          | Required
-------------------------|----------------------|-------------------------------------------------|----------------------------------|---------
`forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.       |                                  | yes
`gitleaks_config`        | `string`             | Path to the custom `gitleaks.toml` file.            | Embedded Gitleaks file           | no
`types`                  | `map(string)`        | Types of secret to look for.                        | All types                        | no
`redact_with`            | `string`             | String to use to redact secrets.                    | `<REDACTED-SECRET:$SECRET_NAME>` | no
`include_generic`        | `bool`               | Include the generic API key rule.                   | `false`                          | no
`allowlist`              | `map(string)`        | List of regexes to allowlist matching secrets.      | `{}`                             | no
`partial_mask`           | `number`             | Show the first N characters (runes) of the secret.  | `0`                              | no

The `gitleaks_config` argument is the path to the custom `gitleaks.toml` file.
The Gitleaks configuration file embedded in the component is used if you don't provide the path to a custom configuration file.

{{< admonition type="note" >}}
This component does not support all the features of the Gitleaks configuration file. Currently, it only supports the regex-based rules, `secretGroup`, and allowlist regexes (`regexTarget` only supports the default value `secret`). Other features such as `keywords`, `entropy`, `paths`, and `stopwords` are not supported. The `extend` feature is also not supported, meaning that a custom configuration file must contain all the rules to use.
{{< /admonition >}}

The `types` argument is a map of secret types to look for.
The values provided are used as prefixes to match rules IDs in the Gitleaks configuration.
For example,  providing the type `grafana` will match the rules `grafana-api-key`, `grafana-cloud-api-token`, and `grafana-service-account-token`.
If you don't provide this argument, all rules are used.

{{< admonition type="note" >}}
Configuring this argument with the secret types you want to look for is strongly recommended.
If you don't, the component will look for all known types, which is resource-intensive.
{{< /admonition >}}

{{< admonition type="caution" >}}
Some secret types in the Gitleaks configuration file rely on regular expression patterns that don't detect the secret itself but rather the context around it.
For example, the `aws-access-token` type detects AWS key IDs, not the keys themselves.
This is because the keys don't have a unique pattern that can easily be detected with a regular expression.
As a result, with this secret type enabled, the component will redact key IDs but not actual secret keys.
This behavior is consistent with the Gitleaks redaction feature but may not be what you expect.
Currently, the secret types known to have this behavior are: `aws-access-token`.
{{< /admonition >}}

The `redact_with` argument is a string that can use variables such as `$SECRET_NAME` (replaced with the matching secret type) and `$SECRET_HASH`(replaced with the sha1 hash of the secret).

The `include_generic` argument is a boolean that includes the generic API key rule in the Gitleaks configuration file if set to `true`. It's disabled by default because it can generate false positives.

The `allowlist` argument is a map of regular expressions to allow matching secrets.
A secret will not be redacted if it matches any of the regular expressions. The allowlist in the Gitleaks configuration file is also applied.

The `partial_mask` argument is the number of characters (runes) to show from the beginning of the secret before the redact string is added.
If set to `0`, the entire secret is redacted.
If a secret is not at least 6 characters (runes) long, it will be entirely redacted.
For short secrets, at most half of the secret (runes) is shown.

## Blocks

The `loki.secretfilter` component doesn't support any blocks and is configured fully through arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                   |
| ---------- | -------------- | ------------------------------------------------------------- |
| `receiver` | `LogsReceiver` | A value that other components can use to send log entries to. |

## Component health

`loki.secretfilter` is only reported as unhealthy if given an invalid configuration.

## Debug metrics

`loki.secretfilter` doesn't expose any component-specific debug information.

## Example

This example shows how to use `loki.secretfilter` to redact secrets from log entries before forwarding them to a Loki receiver.
It uses a custom redaction string that will include the secret type and its hash.

```alloy
local.file_match "local_logs" {
	path_targets = <PATH_TARGETS>
}

loki.source.file "local_logs" {
	targets    = local.file_match.local_logs.targets
	forward_to = [loki.secretfilter.secret_filter.receiver]
}

loki.secretfilter "secret_filter" {
	forward_to  = [loki.write.local_loki.receiver]
	redact_with = "<ALLOY-REDACTED-SECRET:$SECRET_NAME:$SECRET_HASH>"
}

loki.write "local_loki" {
	endpoint {
		url = <LOKI_ENDPOINT>
	}
}
```
Replace the following:
  - `<PATH_TARGETS>`: The paths to the log files to monitor.
  - `<LOKI_ENDPOINT>`: The URL of the Loki instance to send logs to.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.secretfilter` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`loki.secretfilter` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
