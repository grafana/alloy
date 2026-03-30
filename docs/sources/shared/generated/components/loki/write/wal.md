| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `drain_timeout` | `duration` | Maximum time the WAL drain procedure can take, before being forcefully stopped. | `"15s"` | no |
| `enabled` | `bool` | Whether to enable the WAL. | `false` | no |
| `max_read_frequency` | `duration` | Maximum backoff time in the backup read mechanism. | `"1s"` | no |
| `max_segment_age` | `duration` | Maximum time a WAL segment should be allowed to live. Segments older than this setting are eventually deleted. | `"1h"` | no |
| `min_read_frequency` | `duration` | Minimum backoff time in the backup read mechanism. | `"250ms"` | no |
