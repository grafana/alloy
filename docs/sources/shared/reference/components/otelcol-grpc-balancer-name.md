---
aliases:
- /docs/agent/shared/flow/reference/components/otelcol-grpc-balancer-name/
- /docs/grafana-cloud/agent/shared/flow/reference/components/otelcol-grpc-balancer-name/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/otelcol-grpc-balancer-name/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/otelcol-grpc-balancer-name/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/otelcol-grpc-balancer-name/
description: Shared content, otelcol grpc balancer name
headless: true
---

The supported values for `balancer_name` are listed in the gRPC documentation on [Load balancing][]:
* `pick_first`: Tries to connect to the first address, uses it for all RPCs if it connects, or tries the next address if it fails (and keeps doing that until one connection is successful).
  Because of this, all the RPCs will be sent to the same backend.
* `round_robin`: Connects to all the addresses it sees and sends an RPC to each backend one at a time in order.
  For example, the first RPC is sent to backend-1, the second RPC is sent to backend-2, and the third RPC is sent to backend-1.

[Load balancing]: https://github.com/grpc/grpc-go/blob/master/examples/features/load_balancing/README.md#pick_first
