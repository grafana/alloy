---

canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.azure_event_hubs/
aliases:
  - ../loki.source.azure_event_hubs/ # /docs/alloy/latest/reference/components/loki.source.azure_event_hubs/
description: Learn about loki.source.azure_event_hubs
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.azure_event_hubs
---

# `loki.source.azure_event_hubs`

`loki.source.azure_event_hubs` receives Azure Event Hubs messages by making use of an Apache Kafka endpoint on Event Hubs.
Refer to the [Azure Event Hubs documentation](https://learn.microsoft.com/en-us/azure/event-hubs/azure-event-hubs-kafka-overview) for more information.

To learn more about streaming Azure logs to an Azure Event Hubs, refer to
Microsoft's tutorial on how to [Stream Azure Active Directory logs to an Azure event hub](https://learn.microsoft.com/en-us/azure/active-directory/reports-monitoring/tutorial-azure-monitor-stream-logs-to-event-hub).

An Apache Kafka endpoint isn't available within the Basic pricing plan.
Refer to the [Event Hubs pricing page](https://azure.microsoft.com/en-us/pricing/details/event-hubs/) for more information.

You can specify multiple `loki.source.azure_event_hubs` components by giving them different labels.

## Usage

```alloy
loki.source.azure_event_hubs "<LABEL>" {
    fully_qualified_namespace = "<HOST:PORT>"
    event_hubs                = "<EVENT_HUB_LIST>"
    forward_to                = <RECEIVER_LIST>

    authentication {
        mechanism = "AUTHENTICATION_MECHANISM"
    }
}
```

## Arguments

You can use the following arguments with `loki.source.azure_event_hubs`:

| Name                        | Type                 | Description                                                                         | Default                          | Required |
|-----------------------------|----------------------|-------------------------------------------------------------------------------------|----------------------------------|----------|
| `event_hubs`                | `list(string)`       | Event Hubs to consume.                                                              |                                  | yes      |
| `forward_to`                | `list(LogsReceiver)` | List of receivers to send log entries to.                                           |                                  | yes      |
| `fully_qualified_namespace` | `string`             | Event hub namespace.                                                                |                                  | yes      |
| `assignor`                  | `string`             | The consumer group rebalancing strategy to use.                                     | `"range"`                        | no       |
| `disallow_custom_messages`  | `bool`               | Whether to ignore messages that don't match the [schema][] for Azure resource logs. | `false`                          | no       |
| `group_id`                  | `string`             | The Kafka consumer group ID.                                                        | `"loki.source.azure_event_hubs"` | no       |
| `labels`                    | `map(string)`        | The labels to associate with each received event.                                   | `{}`                             | no       |
| `relabel_rules`             | `RelabelRules`       | Relabeling rules to apply on log entries.                                           | `{}`                             | no       |
| `use_incoming_timestamp`    | `bool`               | Whether to use the timestamp received from Azure Event Hub.                         | `false`                          | no       |

The `fully_qualified_namespace` argument must refer to a full `HOST:PORT` that points to your event hub, such as `NAMESPACE.servicebus.windows.net:9093`.
The `assignor` argument must be set to one of `"range"`, `"roundrobin"`, or `"sticky"`.

The `relabel_rules` field can make use of the `rules` export value from a
`loki.relabel` component to apply one or more relabeling rules to log entries
before they're forwarded to the list of receivers in `forward_to`.

[schema]: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema

### Labels

The `labels` map is applied to every message that the component reads.

The following internal labels prefixed with `__` are available but are discarded if not relabeled:

- `__azure_event_hubs_category`
- `__meta_kafka_group_id`
- `__meta_kafka_member_id`
- `__meta_kafka_message_key`
- `__meta_kafka_partition`
- `__meta_kafka_topic`

## Blocks

You can use the following block with `loki.source.azure_event_hubs`:

| Name                               | Description                                        | Required |
|------------------------------------|----------------------------------------------------|----------|
| [`authentication`][authentication] | Authentication configuration with Azure Event Hub. | yes      |

[authentication]: #authentication

### `authentication`

{{< badge text="Required" >}}

The `authentication` block defines the authentication method when communicating with Azure Event Hub.

| Name                | Type           | Description                                                               | Default | Required |
|---------------------|----------------|---------------------------------------------------------------------------|---------|----------|
| `mechanism`         | `string`       | Authentication mechanism.                                                 |         | yes      |
| `connection_string` | `secret`       | Event Hubs ConnectionString for authentication on Azure Cloud.            |         | no       |
| `scopes`            | `list(string)` | Access token scopes. Default is `fully_qualified_namespace` without port. |         | no       |

`mechanism` supports the values `"connection_string"` and `"oauth"`.
If `"connection_string"` is used, you must set the `connection_string` attribute.
If `"oauth"` is used, you must configure one of the [supported credential types](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#credential-types) via environment variables or Azure CLI.

## Exported fields

`loki.source.azure_event_hubs` doesn't export any fields.

## Component health

`loki.source.azure_event_hubs` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.azure_event_hubs` doesn't expose additional debug info.

## Example

This example consumes messages from Azure Event Hub and uses OAuth 2.0 to authenticate itself.

```alloy
loki.source.azure_event_hubs "example" {
    fully_qualified_namespace = "my-ns.servicebus.windows.net:9093"
    event_hubs                = ["gw-logs"]
    forward_to                = [loki.write.example.receiver]

    authentication {
        mechanism = "oauth"
    }
}

loki.write "example" {
    endpoint {
        url = "loki:3100/api/v1/push"
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.azure_event_hubs` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
