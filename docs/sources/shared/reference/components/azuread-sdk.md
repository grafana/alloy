---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/azuread-sdk-block/
description: Shared content, sdk block
headless: true
---

This block configures [Azure SDK authentication][sdk-auth].

| Name        | Type     | Description                                                                                | Default | Required |
| ----------- | -------- | ------------------------------------------------------------------------------------------ | ------- | -------- |
| `tenant_id` | `string` | The tenant ID of the Azure Active Directory application that's being used to authenticate. |         | yes      |

[sdk-auth]: https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication