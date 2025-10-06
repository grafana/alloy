| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `event_hubs` | `list(string)` | Event Hubs to consume. |  | yes |
| `forward_to` | `list(LogsReceiver)` | List of receivers to send log entries to. |  | yes |
| `fully_qualified_namespace` | `string` | Event hub namespace. Must refer to a full HOST:PORT that points to your event hub, such as NAMESPACE.servicebus.windows.net:9093. |  | yes |
| `assignor` | `string` | The consumer group rebalancing strategy to use. | `"range"` | no |
| `disallow_custom_messages` | `bool` | Whether to ignore messages that don't match the schema for Azure resource logs. | `false` | no |
| `group_id` | `string` | The Kafka consumer group ID. | `"loki.source.azure_event_hubs"` | no |
| `labels` | `map(string)` | The labels to associate with each received event. | `{}` | no |
| `relabel_rules` | `RelabelRules` | Relabeling rules to apply on log entries. | `{}` | no |
| `use_incoming_timestamp` | `bool` | Whether to use the timestamp received from Azure Event Hub. | `false` | no |
