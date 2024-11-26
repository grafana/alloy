---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/oauth-block/
description: Shared content, oauth block
headless: true
---

Name        | Type     | Description                                             | Default | Required
------------|----------|---------------------------------------------------------|---------|---------
`client_id` | `string` | Client ID of the Microsoft authenication application used to authenticate. |         | yes
`client_secret` | `string` | Client secret of the Microsoft authenication application used to authenticate. |         | yes
`tenant_id` | `string` | Tenant ID of the Microsoft authenication application used to authenticate. |         | yes

`client_id` should be a valid [UUID][] in one of the supported formats:
* `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* Microsoft encoding: `{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}`
* Raw hex encoding: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

`tenant_id` should be a valid [UUID][] in one of the supported formats:
* `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* Microsoft encoding: `{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}`
* Raw hex encoding: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

[UUID]: https://en.wikipedia.org/wiki/Universally_unique_identifier
