---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/azuread-block/
description: Shared content, azuread block
headless: true
---

| Name    | Type     | Description                                              | Default         | Required |
|---------|----------|----------------------------------------------------------|-----------------|----------|
| `cloud` | `string` | The Azure Cloud.                                         | `"AzurePublic"` | no       |
| `scope` | `string` | Custom OAuth 2.0 scope to request when acquiring tokens. |                 | no       |

The supported values for `cloud` are:

* `"AzurePublic"`
* `"AzureChina"`
* `"AzureGovernment"`
