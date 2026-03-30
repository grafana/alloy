| Name  | Type  | Description  | Default  | Required |
| ----- | ----- | ------------ | -------- | -------- |
| `block_on_overflow` | `bool` | If true, block until there is space in the queue; if false, drop entries when the queue is full. | `true` | no |
| `capacity` | `string` | Controls the size of the underlying send queue buffer. This setting should be considered a worst-case scenario of memory consumption, in which all enqueued batches are full. | `"10MiB"` | no |
| `drain_timeout` | `duration` | Configures the maximum time the client can take to drain the send queue upon shutdown. During that time, it enqueues pending batches and drains the send queue sending each. | `"15s"` | no |
| `min_shards` | `int` | Minimum number of concurrent shards sending samples to the endpoint. | `1` | no |
