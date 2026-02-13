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

| Name             | Type                 | Description                                                                                                                                 | Default                            | Required |
| ---------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------- | -------- |
| `forward_to`     | `list(LogsReceiver)` | List of receivers to send log entries to.                                                                                                   |                                    | yes      |
| `config_path`    | `string`             | Path to a custom Gitleaks TOML config file. If empty, the default Gitleaks config is used.                                                 | `""`                               | no       |
| `origin_label`   | `string`             | Loki label to use for the `secrets_redacted_by_origin` metric. If empty, that metric is not registered.                                     | `""`                               | no       |
| `redact_with`    | `string`             | Template for the redaction placeholder. Use `$SECRET_NAME` and `$SECRET_HASH`. When set, percentage-based redaction is not used.            | `"<REDACTED-SECRET:$SECRET_NAME>"`      | no       |
| `redact_percent` | `uint`               | When `redact_with` is not set: percent of the secret to redact (1–100). Shows leading (100-N)% + `"..."`; 100 = full `"REDACTED"`. 0 or unset defaults to 80. | `80`                               | no       |

The `config_path` argument is the path to a custom [Gitleaks TOML config file][gitleaks-config]. The file supports the standard Gitleaks structure (rules, allowlists, and `[extend]` to extend the default config). If `config_path` is empty, the component uses the default Gitleaks configuration [embedded in the component][embedded-config].

{{< admonition type="note" >}}
The default configuration may change between {{< param "PRODUCT_NAME" >}} versions. For consistent behavior, use an external configuration file via `config_path`.
{{< /admonition >}}

**Redaction behavior:**

- If `redact_with` is set, it is used as the replacement string for every detected secret. Supported placeholders: `$SECRET_NAME` (rule ID) and `$SECRET_HASH` (SHA1 hash of the secret).
- If `redact_with` is not set, redaction is percentage-based (Gitleaks-style): `redact_percent` controls how much of the secret is redacted. For example, `80` shows the first 20% of the secret followed by `"..."`; `100` replaces the entire secret with `"REDACTED"`. When `redact_percent` is 0 or unset, 80% redaction is used.

**Origin metric:** The `origin_label` argument specifies which Loki label to use for the `secrets_redacted_by_origin` metric, so you can track how many secrets were redacted per source or environment.

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

| Name                                           | Type    | Description                                                       |
| ---------------------------------------------- | ------- | ----------------------------------------------------------------- |
| `loki_secretfilter_processing_duration_seconds` | Summary | Time taken to process and redact logs, in seconds.                 |
| `loki_secretfilter_secrets_redacted_total`      | Counter | Total number of secrets redacted.                                  |
| `loki_secretfilter_secrets_redacted_by_rule_total` | Counter | Number of secrets redacted, partitioned by rule name.          |
| `loki_secretfilter_secrets_redacted_by_origin`  | Counter | Number of secrets redacted, partitioned by origin label (when `origin_label` is set). |

## Example

This example shows how to use `loki.secretfilter` to redact secrets from log lines before forwarding them to a Loki receiver. It uses a custom redaction template with `$SECRET_NAME` and `$SECRET_HASH`. You can instead omit `redact_with` to use percentage-based redaction (default 80% redacted), set `redact_percent` (e.g. `100` for full redaction), or set `config_path` to point to a custom Gitleaks TOML file.

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
    // optional: config_path = "/etc/alloy/gitleaks.toml"
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
