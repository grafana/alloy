| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `connection_string` | `secret` | Event Hubs ConnectionString for authentication on Azure Cloud. |  | no |
| `mechanism` | `string` | Authentication mechanism. |  | yes |
| `scopes` | `list(string)` | Access token scopes. Default is fully_qualified_namespace without port. |  | no |
