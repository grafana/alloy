---
description: Shared content, otelcol grpc balancer name
headless: true
---

The supported values for `balancer_name` are listed in the gRPC documentation on [Load balancing][]:

* `pick_first`: Tries to connect to the first address. It uses the address for all RPCs if it connects, or if it fails, it tries the next address and keeps trying until one connection is successful.
  Because of this, all the RPCs are sent to the same backend.
* `round_robin`: Connects to all the addresses it sees and sends an RPC to each backend one at a time in order.
  For example, the first RPC is sent to backend-1, the second RPC is sent to backend-2, and the third RPC is sent to backend-1.

[Load balancing]: https://github.com/grpc/grpc-go/blob/master/examples/features/load_balancing/README.md
