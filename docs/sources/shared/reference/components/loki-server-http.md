---
aliases:
- /docs/agent/shared/flow/reference/components/loki-server-http/
- /docs/grafana-cloud/agent/shared/flow/reference/components/loki-server-http/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/loki-server-http/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/loki-server-http/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/loki-server-http/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/loki-server-http/
description: Shared content, loki server http
headless: true
---

The `http` block configures the HTTP server.

You can use the following arguments to configure the `http` block. Any omitted fields take their default values.

Name                   | Type       | Description                                                                                                      | Default  | Required
-----------------------|------------|------------------------------------------------------------------------------------------------------------------|----------|---------
`conn_limit`           | `int`      | Maximum number of simultaneous HTTP connections. Defaults to no limit.                                           | `0`      | no
`listen_address`       | `string`   | Network address on which the server listens for new connections. Defaults to accepting all incoming connections. | `""`     | no
`listen_port`          | `int`      | Port number on which the server listens for new connections.                                                     | `8080`   | no
`server_idle_timeout`  | `duration` | Idle timeout for HTTP server.                                                                                    | `"120s"` | no
`server_read_timeout`  | `duration` | Read timeout for HTTP server.                                                                                    | `"30s"`  | no
`server_write_timeout` | `duration` | Write timeout for HTTP server.                                                                                   | `"30s"`  | no
