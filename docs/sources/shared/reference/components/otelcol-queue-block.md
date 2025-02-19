---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-queue-block/
description: Shared content, otelcol queue block
headless: true
---

The following arguments are supported:

| Name            | Type      | Description                                                                | Default | Required |
| --------------- | --------- | -------------------------------------------------------------------------- | ------- | -------- |
| `enabled`       | `boolean` | Enables an in-memory buffer before sending data to the client.             | `true`  | no       |
| `num_consumers` | `number`  | Number of readers to send batches written to the queue in parallel.        | `10`    | no       |
| `queue_size`    | `number`  | Maximum number of unwritten batches allowed in the queue at the same time. | `1000`  | no       |
| `blocking`      | `boolean` | If `true`, blocks until the queue has room for a new request.              | `false` | no       |

When `enabled` is `true`, data is first written to an in-memory buffer before sending it to the configured server.
Batches sent to the component's `input` exported field are added to the buffer as long as the number of unsent batches doesn't exceed the configured `queue_size`.

`queue_size` determines how long an endpoint outage is tolerated.
Assuming 100 requests/second, the default queue size `1000` provides about 10 seconds of outage tolerance.
To calculate the correct value for `queue_size`, multiply the average number of outgoing requests per second by the time in seconds that outages are tolerated. A very high value can cause Out Of Memory (OOM) kills.

The `num_consumers` argument controls how many readers read from the buffer and send data in parallel.
Larger values of `num_consumers` allow data to be sent more quickly at the expense of increased network traffic.
