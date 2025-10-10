| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `forward_to` | `[]loki.LogsReceiver` | List of receivers to send enriched logs to. |  | yes |
| `labels_to_copy` | `array` | List of labels to copy from discovered targets to logs. If empty, all labels will be copied. |  | no |
| `logs_match_label` | `string` | The label from incoming logs to match against discovered targets. |  | no |
| `target_match_label` | `string` | The label from discovered targets to match against, for example, "__inventory_consul_service". |  | yes |
| `targets` | `[]discovery.Target` | List of targets from a discovery component. |  | yes |
