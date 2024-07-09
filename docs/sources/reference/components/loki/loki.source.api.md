---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.api/
aliases:
  - ../loki.source.api/ # /docs/alloy/latest/reference/components/loki.source.api/
description: Learn about loki.source.api
title: loki.source.api
---

# loki.source.api

`loki.source.api` receives log entries over HTTP and forwards them to other `loki.*` components.

The HTTP API exposed is compatible with [Loki push API][loki-push-api] and the `logproto` format.
This means that other [`loki.write`][loki.write] components can be used as a client and send requests to `loki.source.api` which enables using {{< param "PRODUCT_NAME" >}} as a proxy for logs.

[loki.write]: ../loki.write/
[loki-push-api]: https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki

## Usage

```alloy
loki.source.api "LABEL" {
    http {
        listen_address = "LISTEN_ADDRESS"
        listen_port = PORT
    }
    forward_to = RECEIVER_LIST
}
```

The component will start an HTTP server on the configured port and address with the following endpoints:

- `/loki/api/v1/push` - accepting `POST` requests compatible with [Loki push API][loki-push-api], for example, from another {{< param "PRODUCT_NAME" >}}'s [`loki.write`][loki.write] component.
- `/loki/api/v1/raw` - accepting `POST` requests with newline-delimited log lines in body.
   This can be used to send NDJSON or plain text logs.
   This is compatible with the Promtail push API endpoint.
   Refer to the [Promtail documentation][promtail-push-api] for more information.
   When this endpoint is used, the incoming timestamps can't be used and the `use_incoming_timestamp = true` setting will be ignored.
- `/ready` - accepting `GET` requests. Can be used to confirm the server is reachable and healthy.
- `/api/v1/push` - internally reroutes to `/loki/api/v1/push`.
- `/api/v1/raw` - internally reroutes to `/loki/api/v1/raw`.


[promtail-push-api]: https://grafana.com/docs/loki/latest/clients/promtail/configuration/#loki_push_api

## Arguments

`loki.source.api` supports the following arguments:

Name                     | Type                 | Description                                                | Default | Required
-------------------------|----------------------|------------------------------------------------------------|---------|---------
`forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.                  |         | yes
`use_incoming_timestamp` | `bool`               | Whether or not to use the timestamp received from request. | `false` | no
`labels`                 | `map(string)`        | The labels to associate with each received logs record.    | `{}`    | no
`relabel_rules`          | `RelabelRules`       | Relabeling rules to apply on log entries.                  | `{}`    | no

The `relabel_rules` field can make use of the `rules` export value from a [`loki.relabel`][loki.relabel] component to apply one or more relabeling rules to log entries before they're forwarded to the list of receivers in `forward_to`.

[loki.relabel]: ../loki.relabel/

## Blocks

The following blocks are supported inside the definition of `loki.source.api`:

Hierarchy | Name     | Description                                        | Required
----------|----------|----------------------------------------------------|---------
`http`    | [http][] | Configures the HTTP server that receives requests. | no

[http]: #http

### http

{{< docs/shared lookup="reference/components/loki-server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.api` doesn't export any fields.

## Component health

`loki.source.api` is only reported as unhealthy if given an invalid configuration.

## Debug metrics

The following are some of the metrics that are exposed when this component is used. Note that the metrics include labels such as `status_code` where relevant, which can be used to measure request success rates.

* `loki_source_api_request_duration_seconds` (histogram): Time (in seconds) spent serving HTTP requests.
* `loki_source_api_request_message_bytes` (histogram): Size (in bytes) of messages received in the request.
* `loki_source_api_response_message_bytes` (histogram): Size (in bytes) of messages sent in response.
* `loki_source_api_tcp_connections` (gauge): Current number of accepted TCP connections.

## Example

This example starts an HTTP server on `0.0.0.0` address and port `9999`.
The server receives log entries and forwards them to a `loki.write` component while adding a `forwarded="true"` label.
The `loki.write` component will send the logs to the specified loki instance using basic auth credentials provided.

```alloy
loki.write "local" {
    endpoint {
        url = "http://loki:3100/api/v1/push"
        basic_auth {
            username = "<your username>"
            password_file = "<your password file>"
        }
    }
}

loki.source.api "loki_push_api" {
    http {
        listen_address = "0.0.0.0"
        listen_port = 9999
    }
    forward_to = [
        loki.write.local.receiver,
    ]
    labels = {
        forwarded = "true",
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.api` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
