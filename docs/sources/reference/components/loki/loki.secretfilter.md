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

{{< admonition type="caution" >}}
Detecting secrets can be resource-intensive and can increase CPU usage significantly.
Roll out this component gradually and monitor resource usage.
Place `loki.secretfilter` after components that reduce log volume so it processes fewer lines.
{{< /admonition >}}

[gitleaks-config]: https://github.com/gitleaks/gitleaks/blob/master/config/gitleaks.toml

## Example usage

This example uses `loki.secretfilter` to redact secrets from log lines before forwarding them to a Loki receiver. It uses a custom redaction template with `$SECRET_NAME` and `$SECRET_HASH`.

Optional arguments are supported, several of which are listed below:

- Omit `redact_with` to use percentage-based redaction, which defaults to 80% redacted.
- Set `redact_percent` to `100` for full redaction.
- Set `gitleaks_config` to point to a custom Gitleaks TOML configuration file.
- Set `rate` to a value below `1.0` to sample entries and reduce CPU usage; entries not selected are forwarded unchanged.

For a full list of arguments, refer to [Arguments](#arguments).

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
    // optional: gitleaks_config = "/etc/alloy/gitleaks.toml"
    // optional: redact_percent = 100  // use when redact_with is not set
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

## Arguments

You can use the following arguments with `loki.secretfilter`:

| Name                 | Type                 | Description                                                                                                                 | Default | Required |
| -------------------- | -------------------- | --------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `forward_to`         | `list(LogsReceiver)` | List of receivers to send log entries to.                                                                                   |         | yes      |
| `drop_on_timeout`    | `bool`               | When true, drop entries that exceed `processing_timeout` instead of forwarding them unredacted.                             | `false` | no       |
| `gitleaks_config`    | `string`             | Path to a custom Gitleaks TOML config file. If empty, the default Gitleaks config is used.                                  | `""`    | no       |
| `label_timed_out`    | `bool`               | When true, adds `secretfilter="timed-out"` to entries forwarded after a processing timeout.                                 | `false` | no       |
| `origin_label`       | `string`             | Loki label to use as the `origin` dimension in `secrets_redacted_by_category_total`.                                        | `""`    | no       |
| `processing_timeout` | `duration`           | Maximum time allowed to process a single log entry. `0` disables the timeout.                                               | `0`     | no       |
| `rate`               | `float`              | Entry sampling rate in `[0.0, 1.0]` where `1` processes all entries. Unsampled entries are forwarded unchanged.             | `1.0`   | no       |
| `redact_percent`     | `uint`               | When `redact_with` is not set: percent of the secret to redact (1–100), where 100 is full redaction.                        | `80`    | no       |
| `redact_with`        | `string`             | Template for the redaction placeholder. Use `$SECRET_NAME` and `$SECRET_HASH`, for example, `"<$SECRET_NAME:$SECRET_HASH>"` | `""`    | no       |

The `gitleaks_config` argument is the path to a custom [Gitleaks TOML config file][gitleaks-config].
The file supports the standard Gitleaks structure (rules, allowlists, and `[extend]` to extend the default config).
If `gitleaks_config` is empty, the component uses the default Gitleaks configuration [embedded in the component][embedded-config].

{{< admonition type="note" >}}
The default configuration may change between {{< param "PRODUCT_NAME" >}} versions.
For consistent behavior, use an external configuration file via `gitleaks_config`.
{{< /admonition >}}

If you leave `origin_label` empty, the component sets the origin label on `secrets_redacted_by_category_total` to `""`.

**Redaction behavior:**

- If `redact_with` is set, it is used as the replacement string for every detected secret.
  The supported placeholders are `$SECRET_NAME` (rule ID) and `$SECRET_HASH` (SHA1 hash of the secret).
- If `redact_with` is not set, redaction is percentage-based (Gitleaks-style).
  `redact_percent` controls how much of the secret is redacted.
  For example, `80` shows the first 20% of the secret followed by `"..."`.
  `100` replaces the entire secret with `"REDACTED"`.
  When `redact_percent` is 0 or unset, 80% redaction is used.

**Sampling:** The `rate` argument controls what fraction of log entries are processed by the secret filter.
Entries that {{< param "PRODUCT_NAME" >}} does not select based on the sampling rate pass through unchanged, with no detection or redaction applied.
Use a value below `1.0`, for example, `0.1` for 10%, to reduce CPU usage when processing high-volume logs.
Monitor `loki_secretfilter_entries_bypassed_total` to observe how many entries were skipped.

**Origin metric:** The `origin_label` argument specifies the Loki label the component uses as the origin dimension in `secrets_redacted_by_category_total`.
You can track how many secrets were redacted per source or environment.
When `origin_label` isn’t set, the `origin` label on `secrets_redacted_by_category_total` defaults to an empty string.

**Processing timeout:** The `processing_timeout` argument sets a maximum duration for processing each log entry.
When the timeout is exceeded, the `loki_secretfilter_lines_timed_out_total` metric is incremented.
By default (`drop_on_timeout = false`), the original unredacted entry is forwarded so no log lines are lost.
When `drop_on_timeout = true`, entries that exceed the timeout are dropped and the `loki_secretfilter_lines_dropped_total` metric is incremented.

Set `label_timed_out = true` to add `secretfilter="timed-out"` to any entry that {{< param "PRODUCT_NAME" >}} forwards after a timeout.
You can then query timed-out lines in Loki, for example, with `{secretfilter="timed-out"}`.
{{< param "PRODUCT_NAME" >}} applies this label only to forwarded entries.

{{< admonition type="caution" >}}
Setting `drop_on_timeout = true` means log lines can be silently dropped.
A dropped line can't be recovered, whereas an unredacted line containing a secret can still be detected and mitigated later.
Use this option only when dropping lines is preferable to forwarding potentially unredacted data.
{{< /admonition >}}

[embedded-config]: https://github.com/gitleaks/gitleaks/blob/master/config/gitleaks.toml

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

| Name                                               | Type    | Description                                                                                    |
| -------------------------------------------------- | ------- | ---------------------------------------------------------------------------------------------- |
| `loki_secretfilter_entries_bypassed_total`         | Counter | Total number of entries forwarded without processing due to sampling.                          |
| `loki_secretfilter_lines_dropped_total`            | Counter | Total number of log lines dropped due to processing timeout, when `drop_on_timeout` is `true`. |
| `loki_secretfilter_lines_timed_out_total`          | Counter | Total number of log lines that exceeded the processing timeout, whether dropped or forwarded.  |
| `loki_secretfilter_processing_duration_seconds`    | Summary | Time taken to process and redact logs, in seconds.                                             |
| `loki_secretfilter_secrets_redacted_total`         | Counter | Total number of secrets redacted.                                                              |
| `loki_secretfilter_secrets_redacted_by_category_total` | Counter | Number of secrets redacted, partitioned by rule name and origin label value. The `origin` label is empty when `origin_label` is not set or the label is absent on the entry. |

## Use a custom Gitleaks configuration

By default, the component detects secrets using the upstream [Gitleaks default ruleset][gitleaks-config], which is compiled into {{< param "PRODUCT_NAME" >}}.
To change which secrets are detected, set the `gitleaks_config` argument to the path of a custom Gitleaks TOML file.

The recommended way to write a custom configuration is to _extend_ the default rules rather than replace them.
Add an `[extend]` block with `useDefault = true` so your file starts from the complete set of built-in rules, then layer your own changes on top.

```toml
title = "Extended Gitleaks config"

[extend]
useDefault = true

# Add your own rule in addition to the built-in ones!
[[rules]]
id = "secret-internal-token"
description = "Secret internal service token"
regex = '''secret_tok_[0-9a-zA-Z]{32}'''
keywords = ["secret_tok_"]
```

Point the component at this file:

```alloy
loki.secretfilter "secret_filter" {
    forward_to      = [loki.write.local_loki.receiver]
    gitleaks_config = "/etc/alloy/gitleaks.toml"
}
```

When `useDefault = true`, redefining a `[[rules]]` block with the same `id` as a built-in rule merges your changes into that rule instead of replacing it.
This is what makes it possible to adjust a built-in rule, for example, to add allowlists to it, without losing its detection logic.

To turn off a built-in rule entirely rather than adjust it, list its `id` in `disabledRules`.
Disabling a rule stops it from detecting anything, so to keep a rule but ignore specific values, use an allowlist instead.

Adjusting a built-in rule with allowlists is the most common way to reduce false positives, as the [following section](#handle-false-positives) demonstrates.
For the full set of configuration options, such as global allowlists, the `condition` field, `stopwords`, and path-based matching, refer to the [Gitleaks configuration documentation][gitleaks-configuration].

[gitleaks-configuration]: https://github.com/gitleaks/gitleaks#configuration

{{< admonition type="note" >}}
Setting `useDefault = false`, or omitting the `[extend]` block, means only the rules you define yourself are used.
The built-in rules are not loaded, so most secrets go undetected unless you redefine them.
{{< /admonition >}}

## Handle false positives

Because detection is regular expression-based, `loki.secretfilter` sometimes redacts values that aren't secrets, such as identifiers, UUIDs, or hashes that happen to look like an API key.
When this happens, identify which rule is firing, then add an _allowlist_ to that rule so the matching values are ignored.

To find the responsible rule, inspect the `rule` label on the `loki_secretfilter_secrets_redacted_by_category_total` metric.

For example, the built-in `generic-api-key` rule is the most common source of false positives because it matches high-entropy strings broadly.
You may benefit from just raising the default entropy value!

For more precise control, keep the rule enabled and allowlist only the specific values that cause false positives.

Allowlist the false positives by extending the same configuration from the previous section.
Redefine the offending rule by `id` and add one or more `[[rules.allowlists]]` blocks.
Each allowlist uses `regexTarget` to choose what its `regexes` are tested against:

- `regexTarget = "match"` tests against the surrounding matched text, which is useful for allowlisting by field name, such as ignoring anything that looks like `token_id=...`.
- `regexTarget = "secret"` tests against the captured secret value itself, which is useful for allowlisting by value shape, such as ignoring UUIDs. This is the default when `regexTarget` is omitted.

A finding is ignored when it matches any allowlist on the rule.

```toml
title = "Extended Gitleaks config"

[extend]
useDefault = true

[[rules]]
id = "secret-internal-token"
description = "Secret internal service token"
regex = '''secret_tok_[0-9a-zA-Z]{32}'''
keywords = ["secret_tok_"]

# Reduce false positives from the built-in generic-api-key rule!
[[rules]]
id = "generic-api-key"
# Raise the generic-api-key entropy threshold above its default of 3.5.
entropy = 4.0

  # Allowlist by matched text: ignore findings around known non-secret fields.
  [[rules.allowlists]]
  description = "Ignore token identifiers"
  regexTarget = "match"
  regexes = [
    "(?i)token_?id",
  ]

  # Allowlist by secret value: ignore when the captured value is a UUID.
  [[rules.allowlists]]
  description = "Ignore UUID values"
  regexTarget = "secret"
  regexes = [
    '''^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$''',
  ]
```

With this configuration, `generic-api-key` keeps detecting real secrets, but no longer redacts log lines such as `token_id=a1B2c3D4e5F6g7H8i9J0` or values shaped like the UUID `8f14e45f-ceea-167a-9c2b-1f0a3e4d5c6b`.

## Manage performance

Secret detection runs a set of regular expressions against every log line, so its cost scales with both your log volume and the number of active rules.
The following options help you control that cost.

**Reduce the number of lines processed.**
Place `loki.secretfilter` after components that filter or drop logs, such as `loki.process`, so it only scans the lines you care about.
Fewer lines in means less work for the component.

**Sample high-volume streams with `rate`.**
Set `rate` to a value below `1.0` to process only a fraction of entries, for example, `0.1` to scan 10% of lines.
Entries that {{< param "PRODUCT_NAME" >}} doesn't select are forwarded unchanged, so this reduces CPU usage at the cost of leaving some lines unscanned.
Monitor `loki_secretfilter_entries_bypassed_total` to see how many entries were skipped.

**Bound per-line work with `processing_timeout`.**
Large lines can be expensive to scan, so set `processing_timeout` to cap how long the component spends on any single entry.
By default, a timed-out line is forwarded unredacted so no data is lost, or you can set `drop_on_timeout = true` to drop it instead.
Monitor `loki_secretfilter_lines_timed_out_total` and `loki_secretfilter_lines_dropped_total`, and use `loki_secretfilter_processing_duration_seconds` to track overall processing time.

**Detect fewer secrets.**
Every rule adds regular expression work, so removing rules you don't need, or raising a noisy rule's `entropy` threshold, also lowers CPU usage.
Refer to [Use a custom Gitleaks configuration](#use-a-custom-gitleaks-configuration) to disable or tune rules.

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
