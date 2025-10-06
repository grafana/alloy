| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `access_key` | `secret` | If set, require Data Firehose to provide a matching key. | `""` | no |
| `forward_to` | `list(LogsReceiver)` | List of receivers to send log entries to. |  | yes |
| `relabel_rules` | `RelabelRules` | Relabeling rules to apply on log entries. | `{}` | no |
| `use_incoming_timestamp` | `bool` | Whether to use the timestamp received from the request. | `false` | no |
