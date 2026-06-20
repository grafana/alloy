---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/azuread-block/
description: Shared content, azuread block
headless: true
---

| Name    | Type     | Description                                                         | Default         | Required |
| ------- | -------- | ------------------------------------------------------------------- | --------------- | -------- |
| `cloud` | `string` | The Azure Cloud.                                                    | `"AzurePublic"` | no       |
| `scope` | `string` | Custom OAuth 2.0 scope (audience) to request when acquiring tokens. | `""`            | no       |

The supported values for `cloud` are:

* `"AzurePublic"`
* `"AzureChina"`
* `"AzureGovernment"`

When `scope` is left empty, {{< param "PRODUCT_NAME" >}} uses the per-cloud Azure Monitor ingestion audience, for example, `https://monitor.azure.com//.default` for `"AzurePublic"`.
Set `scope` to request a token for a different audience, such as a custom Microsoft Entra app registration.
