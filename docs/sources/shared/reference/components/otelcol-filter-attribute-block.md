---
aliases:
- /docs/agent/shared/flow/reference/components/otelcol-filter-attribute-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/otelcol-filter-attribute-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/otelcol-filter-attribute-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/otelcol-filter-attribute-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/otelcol-filter-attribute-block/
description: Shared content, otelcol filter attribute block
headless: true
---

This block specifies an attribute to match against:

* More than one `attribute` block can be defined.
* Only `match_type = "strict"` is allowed if `attribute` is specified.
* All `attribute` blocks must match exactly for a match to occur.

The following arguments are supported:

Name    | Type     | Description                           | Default | Required
--------|----------|---------------------------------------|---------|---------
`key`   | `string` | The attribute key.                    |         | yes
`value` | `any`    | The attribute value to match against. |         | no

If `value` isn't set, any value will match.
The type of `value` could be a number, a string, or a boolean.
