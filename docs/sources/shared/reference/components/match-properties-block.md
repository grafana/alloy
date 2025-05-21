---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/match-properties-block/
description: Shared content, match properties block
headless: true
---

The following arguments are supported:

| Name                 | Type           | Description                                                                    | Default | Required |
| -------------------- | -------------- | ------------------------------------------------------------------------------ | ------- | -------- |
| `match_type`         | `string`       | Controls how items to match against are interpreted.                           |         | yes      |
| `log_bodies`         | `list(string)` | A list of strings that the LogRecord's body field must match against.          | `[]`    | no       |
| `log_severity_texts` | `list(string)` | A list of strings that the LogRecord's severity text field must match against. | `[]`    | no       |
| `metric_names`       | `list(string)` | A list of strings to match the metric name against.                            | `[]`    | no       |
| `services`           | `list(string)` | A list of items to match the service name against.                             | `[]`    | no       |
| `span_kinds`         | `list(string)` | A list of items to match the span kind against.                                | `[]`    | no       |
| `span_names`         | `list(string)` | A list of items to match the span name against.                                | `[]`    | no       |

`match_type` is required and you must set it to either `"regexp"` or `"strict"`.

A match occurs if at least one item in the lists matches.
