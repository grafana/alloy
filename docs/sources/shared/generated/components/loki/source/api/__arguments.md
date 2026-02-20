| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `forward_to` | `list(LogsReceiver)` | List of receivers to send log entries to. |  | yes |
| `graceful_shutdown_timeout` | `duration` | Timeout for server’s graceful shutdown. If configured, should be greater than zero. | `"30s"` | no |
| `labels` | `map(string)` | The labels to associate with each received logs record. | `{}` | no |
| `max_send_message_size` | `string` | Maximum size of a request to the push API.	 | `"100MiB"` | no |
| `relabel_rules` | `RelabelRules` | Relabeling rules to apply on log entries. | `{}` | no |
| `use_incoming_timestamp` | `bool` | Whether to use the timestamp received from request. | `false` | no |
