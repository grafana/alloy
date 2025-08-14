---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/azuread-oauth-block/
description: Shared content, oauth block
headless: true
---

| Name            | Type     | Description                                                                                    | Default | Required |
| --------------- | -------- | ---------------------------------------------------------------------------------------------- | ------- | -------- |
| `client_id`     | `string` | The client ID of the Azure Active Directory application that's being used to authenticate.     |         | yes      |
| `client_secret` | `secret` | The client secret of the Azure Active Directory application that's being used to authenticate. |         | yes      |
| `tenant_id`     | `string` | The tenant ID of the Azure Active Directory application that's being used to authenticate.     |         | yes      |
