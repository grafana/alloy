---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/rule-block-database-observability/
description: Shared content, rule block for the database observability components
headless: true
---

The `rule` block defines a relabeling rule applied to the metrics and the logs of a single monitored database.
If you define more than one `rule` block, the rules are applied in top-down order, after the labels the component sets itself, so a rule can read those labels with `source_labels`.

It takes the same arguments as the [`rule` block][prometheus.relabel] used elsewhere in {{< param "PRODUCT_NAME" >}}, in a restricted form: because one rule set applies to two signal paths, it supports the labelling actions only, and it can't overwrite most of the labels the component sets.
{{< param "PRODUCT_NAME" >}} refuses to load a configuration containing a rule that breaks either restriction, so such a rule is never silently ignored.

Two further constraints are specific to a `database_instance` block:

- **`target_label` must be a literal label name**: {{< param "PRODUCT_NAME" >}} checks it against the reserved labels when it loads the configuration, so it can't contain a capture group reference.
- **`hashmod` requires `source_labels`, and can't target `instance`**: hashing the empty value gives every database the same result, and hashing into `instance` buckets several databases onto one value even when the source is present.

{{< admonition type="caution" >}}
A relabeling rule that computes an empty value *removes* its target label instead of setting it, and the default `replacement` of `"$1"` computes an empty value when a rule has no `source_labels`.
So `rule { target_label = "environment" }` removes `environment` rather than setting it, and {{< param "PRODUCT_NAME" >}} rejects it.
Always set `replacement` alongside a `target_label`.

{{< param "PRODUCT_NAME" >}} can only reject such a rule where the outcome is certain.
A rule that reads `source_labels` depends on the labels present at runtime, so a missing or misspelled source label still removes the target label, except for the reserved labels and `instance`, which keep their previous value.
Only removal is prevented: a rule that computes a *non-empty* value still overwrites those labels, which is why `hashmod` can't target `instance` at all.
{{< /admonition >}}

You can use the `hashmod`, `labeldrop`, `lowercase`, `replace`, and `uppercase` actions.
The rest aren't supported:

- `drop`, `dropequal`, `keep`, and `keepequal` filter rather than label, and filtering means two different things on the two signal paths. Dropping a metrics target stops the database exporting metrics entirely, whereas dropping a log entry discards one line. To filter one database's logs, use a [`loki.relabel`][loki.relabel] component on the receiver named in `forward_to`, with a `drop` rule that matches that database's `instance` or `server_id`. Every database the component monitors forwards to the same receivers, so the rule has to identify the one it applies to. To filter one database's metrics, use a [`discovery.relabel`][discovery.relabel] component on the component's exported targets.
- `labelmap` builds label names from `replacement` at runtime, so {{< param "PRODUCT_NAME" >}} can't check them against the reserved labels when it loads the configuration.
- `labelkeep` removes every label its `regex` doesn't match, so a rule that preserves the reserved labels would have to match all of them. Use `labeldrop` to remove specific labels instead.

Because the rules run after the component's own labels, and those are protected, a `labeldrop` rule can only remove a label that an earlier rule in the same block created.

#### Reserved labels

A rule can't write to the following labels, and a `labeldrop` regular expression can't match them:

- **Both metrics and logs**: `job` and `server_id`.
- **Logs only**: `engine` and `op`.
- **Metrics only**: `provider_name`, `provider_region`, `provider_account`, `__address__`, `__scheme__`, and `__metrics_path__`.

This applies to `rule` blocks only.
A downstream `discovery.relabel` or `loki.relabel` component can still overwrite any of them, but it applies to every database the component monitors rather than one.

The `instance` label is a special case: a rule can rewrite its value, which is a common reason to use a `rule` block, because a friendly name such as `orders_db` then applies to that database's metrics and logs together.
A rule can't remove it, because metrics and logs are correlated on it, and because an empty `instance` on a metrics target makes Prometheus fill it in from `__address__`, which every database in one component shares, collapsing them all onto one series.

For the same reason, two `database_instance` blocks can't rewrite `instance` to the same value.
{{< param "PRODUCT_NAME" >}} rejects that when both values are fixed in the configuration, but it can't when the value comes from a label the databases share: `source_labels = ["job"]` gives every database the same `instance`, since they all carry the same `job`.
Derive `instance` from something that differs per database, or set a literal value in each block.

#### Labels set on only one signal path

Some of the labels above exist on one signal path and not the other, and a rule that reads one with `source_labels` behaves differently on each path: where the label is absent the rule computes an empty value, which *removes* its `target_label` rather than setting it to an empty string.

A rule reading `provider_region`, which a database with a [`cloud_provider`](#cloud_provider) block carries on its metrics targets but not on its log entries, sets `region` on that database's metrics and removes `region` from its logs.
Reading a logs-only label such as `op` does the same in reverse.
Only `job`, `server_id`, and `instance` are set on both paths, so prefer those as `source_labels`, or a label an earlier rule in the same block created.

[prometheus.relabel]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.relabel/#rule
[loki.relabel]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.relabel/
[discovery.relabel]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/discovery/discovery.relabel/
