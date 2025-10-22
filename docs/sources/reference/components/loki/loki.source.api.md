---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.api/
aliases:
  - ../loki.source.api/ # /docs/alloy/latest/reference/components/loki.source.api/
description: Learn about loki.source.api
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.api
---

# `loki.source.api`

`loki.source.api` receives log entries over HTTP and forwards them to other `loki.*` components.

The HTTP API exposed is compatible with [Loki push API][loki-push-api] and the `logproto` format.
This means that other [`loki.write`][loki.write] components can be used as a client and send requests to `loki.source.api` which enables using {{< param "PRODUCT_NAME" >}} as a proxy for logs.

[loki.write]: ../loki.write/
[loki-push-api]: https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki

## Usage

```alloy
loki.source.api "<LABEL>" {
    http {
        listen_address = "<LISTEN_ADDRESS>"
        listen_port = "<PORT>"
    }
    forward_to = <RECEIVER_LIST>
}
```

The component starts an HTTP server on the configured port and address with the following endpoints:

* `/loki/api/v1/push` - accepting `POST` requests compatible with [Loki push API][loki-push-api], for example, from another {{< param "PRODUCT_NAME" >}}'s [`loki.write`][loki.write] component.
* `/loki/api/v1/raw` - accepting `POST` requests with newline-delimited log lines in body.
  This can be used to send NDJSON or plain text logs.
  This is compatible with the Promtail push API endpoint.
  Refer to the [Promtail documentation][promtail-push-api] for more information.
  When this endpoint is used, the incoming timestamps can't be used and the `use_incoming_timestamp = true` setting is ignored.
* `/ready` - accepting `GET` requests. Can be used to confirm the server is reachable and healthy.
* `/api/v1/push` - internally reroutes to `/loki/api/v1/push`.
* `/api/v1/raw` - internally reroutes to `/loki/api/v1/raw`.

[promtail-push-api]: https://grafana.com/docs/loki/latest/clients/promtail/configuration/#loki_push_api

## Arguments

You can use the following arguments with `loki.source.api`:

| Name                     | Type                 | Description                                             | Default    | Required |
| ------------------------ | -------------------- | ------------------------------------------------------- | ---------- | -------- |
| `forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.               |            | yes      |
| `labels`                 | `map(string)`        | The labels to associate with each received logs record. | `{}`       | no       |
| `relabel_rules`          | `RelabelRules`       | Relabeling rules to apply on log entries.               | `{}`       | no       |
| `use_incoming_timestamp` | `bool`               | Whether to use the timestamp received from request.     | `false`    | no       |
| `max_send_message_size`  | `size`               | Maximum size of a request to the push API.              | `"100MiB"` | no       |

The `relabel_rules` field can make use of the `rules` export value from a [`loki.relabel`][loki.relabel] component to apply one or more relabeling rules to log entries before they're forwarded to the list of receivers in `forward_to`.

[loki.relabel]: ../loki.relabel/

## Blocks

You can use the following blocks with `loki.source.api`:

| Name                  | Description                                        | Required |
| --------------------- | -------------------------------------------------- | -------- |
| [`http`][http]        | Configures the HTTP server that receives requests. | no       |
| `http` > [`tls`][tls] | Configures TLS for the HTTP server.                | no       |

The > symbol indicates deeper levels of nesting.
For example, `http` > `tls` refers to a `tls` block defined inside an `http` block.

[http]: #http
[tls]: #tls

### `http`

{{< docs/shared lookup="reference/components/server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS for the HTTP server.

{{< docs/shared lookup="reference/components/server-tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.api` doesn't export any fields.

## Component health

`loki.source.api` is only reported as unhealthy if given an invalid configuration.

## Debug metrics

The following are some of the metrics that are exposed when this component is used.
The metrics include labels such as `status_code` where relevant, which can be used to measure request success rates.

* `loki_source_api_request_duration_seconds` (histogram): Time (in seconds) spent serving HTTP requests.
* `loki_source_api_request_message_bytes` (histogram): Size (in bytes) of messages received in the request.
* `loki_source_api_response_message_bytes` (histogram): Size (in bytes) of messages sent in response.
* `loki_source_api_tcp_connections` (gauge): Current number of accepted TCP connections.

## Example

This example starts an HTTP server on `0.0.0.0` address and port `9999`.
The server receives log entries and forwards them to a `loki.write` component while adding a `forwarded="true"` label.
The `loki.write` component sends the logs to the specified Loki instance using basic auth credentials provided.

```alloy
loki.write "local" {
    endpoint {
        url = "http://loki:3100/api/v1/push"
        basic_auth {
            username = "<USERNAME>"
            password_file = "<PASSWORD_FILE>"
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

Replace the following:

* _`<USERNAME>`_: Your username.
* _`<PASSWORD_FILE>`_: Your password file.

### Technical details

`loki.source.api` filters out all labels that start with `__`, for example, `__tenant_id__`.

If you need to be able to set the tenant ID, you must either make sure the `X-Scope-OrgID` header is present or use the [`loki.process`][loki.process] component.

[loki.process]: ../loki.process/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.api` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
