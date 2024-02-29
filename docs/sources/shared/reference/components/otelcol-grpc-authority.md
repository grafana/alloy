---
aliases:
- /docs/agent/shared/flow/reference/components/otelcol-grpc-authority/
- /docs/grafana-cloud/agent/shared/flow/reference/components/otelcol-grpc-authority/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/otelcol-grpc-authority/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/otelcol-grpc-authority/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/otelcol-grpc-authority/
description: Shared content, otelcol grpc authority
headless: true
---

The `:authority` header in gRPC specifies the host to which the request is being sent.
It's similar to the `Host` [header][HTTP host header] in HTTP requests.
By default, the value for `:authority` is derived from the endpoint URL used for the gRPC call.
Overriding `:authority` could be useful when routing traffic using a proxy like Envoy, which [makes routing decisions][Envoy route matching] based on the value of the `:authority` header.

[HTTP host header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host
[Envoy route matching]: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/route_matching
