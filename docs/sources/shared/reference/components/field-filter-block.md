---
aliases:
- /docs/agent/shared/flow/reference/components/filter-field-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/filter-field-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/field-filter-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/filter-field-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/filter-field-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/field-filter-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/filter-field-block/
description: Shared content, filter field block
headless: true
---

The following attributes are supported:

Name    | Type     | Description                                                   | Default  | Required
--------|----------|---------------------------------------------------------------|----------|---------
`key`   | `string` | The key or name of the field or labels that a filter can use. |          | yes
`value` | `string` | The value associated with the key that a filter can use.      |          | yes
`op`    | `string` | The filter operation to apply on the given key: value pair.   | `equals` | no

For `op`, the following values are allowed:
* `equals`: The field value must equal the provided value.
* `not-equals`: The field value must not be equal to the provided value.
* `exists`: The field value must exist. Only applicable to `annotation` fields.
* `does-not-exist`: The field value must not exist. Only applicable to `annotation` fields.
