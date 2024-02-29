---
aliases:
- /docs/agent/shared/flow/reference/components/loki-server-grpc/
- /docs/grafana-cloud/agent/shared/flow/reference/components/loki-server-grpc/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/loki-server-grpc/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/loki-server-grpc/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/loki-server-grpc/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/loki-server-grpc/
description: Shared content, loki server grpc
headless: true
---

The `grpc` block configures the gRPC server.

You can use the following arguments to configure the `grpc` block. Any omitted fields take their default values.

Name                            | Type       | Description                                                                                                         | Default      | Required
--------------------------------|------------|---------------------------------------------------------------------------------------------------------------------|--------------|---------
`conn_limit`                    | `int`      | Maximum number of simultaneous HTTP connections. Defaults to no limit.                                              | `0`          | no
`listen_address`                | `string`   | Network address on which the server listens for new connections. It defaults to accepting all incoming connections. | `""`         | no
`listen_port`                   | `int`      | Port number on which the server listens for new connections. Defaults to a random free port.                        | `0`          | no
`max_connection_age_grace`      | `duration` | An additive period after `max_connection_age` after which the connection is forcibly closed.                        | `"infinity"` | no
`max_connection_age`            | `duration` | The duration for the maximum time a connection may exist before it is closed.                                       | `"infinity"` | no
`max_connection_idle`           | `duration` | The duration after which an idle connection is closed.                                                              | `"infinity"` | no
`server_max_concurrent_streams` | `int`      | Limit on the number of concurrent streams for gRPC calls (0 = unlimited).                                           | `100`        | no
`server_max_recv_msg_size`      | `int`      | Limit on the size of a gRPC message this server can receive (bytes).                                                | `4MB`        | no
`server_max_send_msg_size`      | `int`      | Limit on the size of a gRPC message this server can send (bytes).                                                   | `4MB`        | no
