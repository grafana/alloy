---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.secretfilter/
description: Learn about loki.secretfilter
title: loki.secretfilter
labels:
  stage: experimental
  products:
    - oss
---

# `loki.secretfilter`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`loki.secretfilter` receives log entries and redacts detected secrets from the log lines.
The detection relies on regular expression patterns, defined in the Gitleaks configuration file embedded within the component.
`loki.secretfilter` can also use a [custom configuration file](#arguments) based on the [Gitleaks configuration file structure][gitleaks-config].

{{< admonition type="caution" >}}
Personally Identifiable Information (PII) isn't currently in scope and some secrets could remain undetected.
This component may generate false positives or redact too much.
Don't rely solely on this component to redact sensitive information.
{{< /admonition >}}

{{< admonition type="note" >}}
This component operates on log lines and doesn't scan labels or other metadata.
{{< /admonition >}}

[gitleaks-config]: https://github.com/gitleaks/gitleaks/blob/master/config/gitleaks.toml

## Usage

```alloy
loki.secretfilter "<LABEL>" {
    forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `loki.secretfilter`:

| Name                     | Type                 | Description                                                                 | Default                            | Required |
| ------------------------ | -------------------- | --------------------------------------------------------------------------- | ---------------------------------- | -------- |
| `forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.                                   |                                    | yes      |
| `allowlist`              | `list(string)`       | List of regular expressions to allowlist matching secrets.                  | `[]`                               | no       |
| `block_on_full`          | `bool`               | Block instead of dropping when the queue is full, retrying with backoff.    | `false`                            | no       |
| `enable_entropy`         | `bool`               | Enable entropy-based filtering.                                             | `false`                            | no       |
| `gitleaks_config`        | `string`             | Path to the custom `gitleaks.toml` file.                                    | Embedded Gitleaks file             | no       |
| `include_generic`        | `bool`               | Include the generic API key rule.                                           | `false`                            | no       |
| `max_forward_queue_size` | `int`                | Maximum number of log entries to buffer per destination before dropping.    | `100000`                           | no       |
| `origin_label`           | `string`             | Loki label to use for the `secrets_redacted_by_origin` metric.              | `""`                               | no       |
| `partial_mask`           | `int`                | Show the first N characters of the secret.                                  | `0`                                | no       |
| `redact_with`            | `string`             | String to use to redact secrets.                                            | `"<REDACTED-SECRET:$SECRET_NAME>"` | no       |
| `types`                  | `list(string)`       | List of secret types to look for.                                           | All types                          | no       |

When `block_on_full` is `false` (default), log entries are dropped if a destination's queue is full.
When `block_on_full` is `true`, the component retries with exponential backoff (5ms to 5s), which may slow the pipeline but prevents data loss.

The `gitleaks_config` argument is the path to the custom `gitleaks.toml` file.
If you don't provide the path to a custom configuration file, the Gitleaks configuration file [embedded in the component][embedded-config] is used.

{{< admonition type="note" >}}
This component doesn't support all the features of the Gitleaks configuration file.
It only supports regular expression-based rules, `secretGroup`, and allowlist regular expressions. `regexTarget` only supports the default value `secret`.
Other features such as `keywords`, `paths`, and `stopwords` aren't supported.
The `extend` feature isn't supported.
If you use a custom configuration file, you must include all the rules you want to use within the configuration file.
Unsupported fields and values in the configuration file are ignored.
{{< /admonition >}}

{{< admonition type="note" >}}
The embedded configuration file may change between {{< param "PRODUCT_NAME" >}} versions.
To ensure consistency, use an external configuration file.
{{< /admonition >}}

The `types` argument is a list of secret types to look for.
The values provided are used as prefixes to match rules IDs in the Gitleaks configuration.
For example, providing the type `grafana` matches the rules `grafana-api-key`, `grafana-cloud-api-token`, and `grafana-service-account-token`.
If you don't provide this argument, all rules are used.

{{< admonition type="note" >}}
Configuring this argument with the secret types you want to look for is strongly recommended.
If you don't, the component looks for all known types, which is resource-intensive.
{{< /admonition >}}

{{< admonition type="caution" >}}
Some secret types in the Gitleaks configuration file rely on regular expression patterns that don't detect the secret itself but rather the context around it.
For example, the `aws-access-token` type detects AWS key IDs, not the keys themselves.
This is because the keys don't have a unique pattern that can easily be detected with a regular expression.
As a result, with this secret type enabled, the component redacts key IDs but not actual secret keys.
This behavior is consistent with the Gitleaks redaction feature but may not be what you expect.
Currently, the secret types known to have this behavior are: `aws-access-token`.
{{< /admonition >}}

The `redact_with` argument is a string that can use variables such as `$SECRET_NAME`, replaced with the matching secret type, and `$SECRET_HASH`, replaced with the SHA1 hash of the secret.

The `include_generic` argument is a boolean that enables the generic API key rule in the Gitleaks configuration file if set to `true`.
It's disabled by default because it can generate false positives.
Consider enabling entropy-based filtering if you enable this rule.

The `allowlist` argument is a list of regular expressions to allow matching secrets.
A secret won't be redacted if it matches any of the regular expressions. The allowlist in the Gitleaks configuration file is also applied.

The `partial_mask` argument is the number of characters to show from the beginning of the secret before the redact string is added.
If set to `0`, the entire secret is redacted.
If a secret isn't at least 6 characters long, it's entirely redacted.
For short secrets, at most half of the secret is shown.

The `enable_entropy` argument enables entropy-based filtering.
When you set this to `true`, the entropy of the detected potential secret is calculated and compared against the threshold provided for the matching rule in the configuration file.
The secret is then redacted only if the entropy is above the threshold.
This can help reduce false positives.

The `origin_label` argument specifies which Loki label value to use for the `secrets_redacted_by_origin` metric.
This metric tracks how many secrets were redacted in logs from different sources or environments.

[embedded-config]: https://github.com/grafana/alloy/blob/{{< param "ALLOY_RELEASE" >}}/internal/component/loki/secretfilter/gitleaks.toml

## Blocks

The `loki.secretfilter` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                   |
| ---------- | -------------- | ------------------------------------------------------------- |
| `receiver` | `LogsReceiver` | A value that other components can use to send log entries to. |

## Component health

`loki.secretfilter` is only reported as unhealthy if given an invalid configuration.

## Debug metrics

`loki.secretfilter` exposes the following Prometheus metrics:

| Name                                                      | Type    | Description                                                                                                   |
| --------------------------------------------------------- | ------- | ------------------------------------------------------------------------------------------------------------- |
| `loki_secretfilter_processing_duration_seconds`           | Summary | Summary of the time taken to process and redact logs in seconds.                                              |
| `loki_secretfilter_secrets_allowlisted_total`             | Counter | Number of secrets that matched a rule but were in an allowlist, partitioned by source.                        |
| `loki_secretfilter_secrets_redacted_by_origin`            | Counter | Number of secrets redacted, partitioned by origin label value.                                                |
| `loki_secretfilter_secrets_redacted_by_rule_total`        | Counter | Number of secrets redacted, partitioned by rule name.                                                         |
| `loki_secretfilter_secrets_redacted_total`                | Counter | Total number of secrets that have been redacted.                                                              |
| `loki_secretfilter_secrets_skipped_entropy_by_rule_total` | Counter | Number of secrets that matched a rule but whose entropy was too low to be redacted, partitioned by rule name. |

The `origin_label` argument specifies which Loki label value to use for the `secrets_redacted_by_origin` metric.
This metric tracks how many secrets were redacted in logs from different sources or environments.

## Example

This example shows how to use `loki.secretfilter` to redact secrets from log lines before forwarding them to a Loki receiver.
It uses a custom redaction string that includes the secret type and its hash.

```alloy
local.file_match "local_logs" {
    path_targets = "<PATH_TARGETS>"
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
        url = "<LOKI_ENDPOINT>"
    }
}
```

Replace the following:

* _`<PATH_TARGETS>`_: The paths to the log files to monitor.
* _`<LOKI_ENDPOINT>`_: The URL of the Loki instance to send logs to.

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
