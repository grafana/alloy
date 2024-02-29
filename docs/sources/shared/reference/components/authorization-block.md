---
aliases:
- /docs/agent/shared/flow/reference/components/authorization-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/authorization-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/authorization-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/authorization-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/authorization-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/authorization-block/
description: Shared content, authorization block
headless: true
---

Name               | Type     | Description                                | Default | Required
-------------------|----------|--------------------------------------------|---------|---------
`credentials_file` | `string` | File containing the secret value.          |         | no
`credentials`      | `secret` | Secret value.                              |         | no
`type`             | `string` | Authorization type, for example, "Bearer". |         | no

`credential` and `credentials_file` are mutually exclusive, and only one can be provided inside an `authorization` block.
