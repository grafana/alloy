---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/managed_identity-block/
description: Shared content, managed_identity block
headless: true
---

| Name        | Type     | Description                                             | Default | Required |
| ----------- | -------- | ------------------------------------------------------- | ------- | -------- |
| `client_id` | `string` | Client ID of the managed identity used to authenticate. |         | yes      |

`client_id` should be a valid [UUID][] in one of the supported formats:

* `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* Microsoft encoding: `{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}`
* Raw hex encoding: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

[UUID]: https://en.wikipedia.org/wiki/Universally_unique_identifier
