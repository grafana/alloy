| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `additional_fields` | `list(string)` | The additional list of fields to supplement those provided via fields_type. |  | no |
| `api_token` | `secret` | The API token to authenticate with. |  | yes |
| `fields_type` | `string` | The set of fields to fetch for log entries. | `"default"` | no |
| `forward_to` | `list(LogsReceiver)` | List of receivers to send log entries to. |  | yes |
| `labels` | `map(string)` | The labels to associate with each received log entry. | `{}` | no |
| `pull_range` | `duration` | The timeframe to fetch for each pull request. | `"1m"` | no |
| `relabel_rules` | `RelabelRules` | Relabeling rules to apply on log entries. | `{}` | no |
| `use_incoming_timestamp` | `bool` | Whether to use the timestamp received from the request. | `false` | no |
| `workers` | `int` | The number of workers to use for parsing logs. | `3` | no |
| `zone_id` | `string` | The Cloudflare zone ID to use. |  | yes |
