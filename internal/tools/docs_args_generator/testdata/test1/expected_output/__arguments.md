| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `forward_to` | `list(LogsReceiver)` | List of receivers to send enriched logs to. |  | yes |
| `labels` | `map(string)` | The labels to associate with each received logs record. | `{}` | no |
| `max_send_message_size` | `int` | Maximum size of a request to the push API.	 | `"100MiB"` | no |
| `relabel_rules` | `RelabelRules` | Relabeling rules to apply on log entries. | `{}` | no |
| `use_incoming_timestamp` | `bool` | Whether to use the timestamp received from request.	 | `false` | no |
