---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/workload_identity-block/
description: Shared content, workload_identity block
headless: true
---

| Name              | Type     | Description                                                                     | Default                                                | Required |
| ----------------- | -------- | ------------------------------------------------------------------------------- | ------------------------------------------------------ | -------- |
| `client_id`       | `string` | Client ID of the Microsoft Entra application or user-assigned managed identity. |                                                        | yes      |
| `tenant_id`       | `string` | Tenant ID of the Microsoft Entra application or user-assigned managed identity. |                                                        | yes      |
| `token_file_path` | `string` | Path to the projected service account token file.                               | `"/var/run/secrets/azure/tokens/azure-identity-token"` | no       |

`client_id` and `tenant_id` must be valid [UUID][]s in one of the supported formats:

* `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
* Microsoft encoding: `{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}`
* Raw hex encoding: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

Use `workload_identity` to authenticate from an AKS Pod configured with [Microsoft Entra Workload ID][workload-id] without requiring a client secret.
Alloy reads the projected service account token from `token_file_path` and federates it for a Microsoft Entra token.

[UUID]: https://en.wikipedia.org/wiki/Universally_unique_identifier
[workload-id]: https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview
