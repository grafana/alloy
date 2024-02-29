---
aliases:
- /docs/agent/shared/flow/reference/components/sigv4-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/sigv4-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/sigv4-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/sigv4-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/sigv4-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/sigv4-block/
description: Shared content, sigv4 block
headless: true
---

Name         | Type     | Description                                         | Default | Required
-------------|----------|-----------------------------------------------------|---------|---------
`access_key` | `string` | AWS API access key.                                 |         | no
`profile`    | `string` | Named AWS profile used to authenticate.             |         | no
`region`     | `string` | AWS region.                                         |         | no
`role_arn`   | `string` | AWS Role ARN, an alternative to using AWS API keys. |         | no
`secret_key` | `secret` | AWS API secret key.                                 |         | no

If `region` is left blank, the region from the default credentials chain is used.

If `access_key` is left blank, the environment variable `AWS_ACCESS_KEY_ID` is used.

If `secret_key` is left blank, the environment variable `AWS_SECRET_ACCESS_KEY` is used.
