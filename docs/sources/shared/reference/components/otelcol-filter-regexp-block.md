---
aliases:
- /docs/agent/shared/flow/reference/components/otelcol-filter-regexp-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/otelcol-filter-regexp-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/otelcol-filter-regexp-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/otelcol-filter-regexp-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/otelcol-filter-regexp-block/
description: Shared content, otelcol filter regexp block
headless: true
---

This block is an optional configuration for the `match_type` of `"regexp"`.
It configures a Least Recently Used (LRU) cache.

The following arguments are supported:

Name                    | Type   | Description                                                           | Default | Required
------------------------|--------|-----------------------------------------------------------------------|---------|---------
`cache_enabled`         | `bool` | Determines whether match results are LRU cached.                      | `false` | no
`cache_max_num_entries` | `int`  | The max number of entries of the LRU cache that stores match results. | `0`     | no

Enabling `cache_enabled` could make subsequent matches faster.
Cache size is unlimited unless `cache_max_num_entries` is also specified.

`cache_max_num_entries` is ignored if `cache_enabled` is false.
