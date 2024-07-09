---

canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.azure_event_hubs/
aliases:
  - ../loki.source.azure_event_hubs/ # /docs/alloy/latest/reference/components/loki.source.azure_event_hubs/
description: Learn about loki.source.azure_event_hubs
title: loki.source.azure_event_hubs
---

# loki.source.azure_event_hubs

`loki.source.azure_event_hubs` receives Azure Event Hubs messages by making use of an Apache Kafka
endpoint on Event Hubs. For more information, see
the [Azure Event Hubs documentation](https://learn.microsoft.com/en-us/azure/event-hubs/azure-event-hubs-kafka-overview).

To learn more about streaming Azure logs to an Azure Event Hubs, refer to 
Microsoft's tutorial on how to [Stream Azure Active Directory logs to an Azure event hub](https://learn.microsoft.com/en-us/azure/active-directory/reports-monitoring/tutorial-azure-monitor-stream-logs-to-event-hub).

Note that an Apache Kafka endpoint is not available within the Basic pricing plan. For more information, see
the [Event Hubs pricing page](https://azure.microsoft.com/en-us/pricing/details/event-hubs/).

Multiple `loki.source.azure_event_hubs` components can be specified by giving them
different labels.

## Usage

```alloy
loki.source.azure_event_hubs "LABEL" {
	fully_qualified_namespace = "HOST:PORT"
	event_hubs                = EVENT_HUB_LIST
	forward_to                = RECEIVER_LIST

	authentication {
		mechanism = "AUTHENTICATION_MECHANISM"
	}
}
```

## Arguments

`loki.source.azure_event_hubs` supports the following arguments:

Name                        | Type                 | Description                                                        | Default                          | Required
----------------------------|----------------------|--------------------------------------------------------------------|----------------------------------|---------
`fully_qualified_namespace` | `string`             | Event hub namespace.                                               |                                  | yes
`event_hubs`                | `list(string)`       | Event Hubs to consume.                                             |                                  | yes
`group_id`                  | `string`             | The Kafka consumer group id.                                       | `"loki.source.azure_event_hubs"` | no
`assignor`                  | `string`             | The consumer group rebalancing strategy to use.                    | `"range"`                        | no
`use_incoming_timestamp`    | `bool`               | Whether or not to use the timestamp received from Azure Event Hub. | `false`                          | no
`labels`                    | `map(string)`        | The labels to associate with each received event.                  | `{}`                             | no
`forward_to`                | `list(LogsReceiver)` | List of receivers to send log entries to.                          |                                  | yes
`relabel_rules`             | `RelabelRules`       | Relabeling rules to apply on log entries.                          | `{}`                             | no
`disallow_custom_messages`  | `bool`               | Whether to ignore messages that don't match the [schema](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema) for Azure resource logs. | `false` | no
`relabel_rules`             | `RelabelRules`       | Relabeling rules to apply on log entries.                          | `{}`                             | no

The `fully_qualified_namespace` argument must refer to a full `HOST:PORT` that points to your event hub, such as `NAMESPACE.servicebus.windows.net:9093`.
The `assignor` argument must be set to one of `"range"`, `"roundrobin"`, or `"sticky"`.

The `relabel_rules` field can make use of the `rules` export value from a
`loki.relabel` component to apply one or more relabeling rules to log entries
before they're forwarded to the list of receivers in `forward_to`.

### Labels

The `labels` map is applied to every message that the component reads.

The following internal labels prefixed with `__` are available but are discarded if not relabeled:

- `__meta_kafka_message_key`
- `__meta_kafka_topic`
- `__meta_kafka_partition`
- `__meta_kafka_member_id`
- `__meta_kafka_group_id`
- `__azure_event_hubs_category`

## Blocks

The following blocks are supported inside the definition of `loki.source.azure_event_hubs`:

Hierarchy      | Name             | Description                                        | Required
---------------|------------------|----------------------------------------------------|---------
authentication | [authentication] | Authentication configuration with Azure Event Hub. | yes

[authentication]: #authentication-block

### authentication block

The `authentication` block defines the authentication method when communicating with Azure Event Hub.

Name                | Type           | Description                                                               | Default | Required
--------------------|----------------|---------------------------------------------------------------------------|---------|---------
`mechanism`         | `string`       | Authentication mechanism.                                                 |         | yes
`connection_string` | `string`       | Event Hubs ConnectionString for authentication on Azure Cloud.            |         | no
`scopes`            | `list(string)` | Access token scopes. Default is `fully_qualified_namespace` without port. |         | no

`mechanism` supports the values `"connection_string"` and `"oauth"`. If `"connection_string"` is used,
you must set the `connection_string` attribute. If `"oauth"` is used, you must configure one of the supported credential
types as documented
here: https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#credential-types via environment
variables or Azure CLI.

## Exported fields

`loki.source.azure_event_hubs` does not export any fields.

## Component health

`loki.source.azure_event_hubs` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`loki.source.azure_event_hubs` does not expose additional debug info.

## Example

This example consumes messages from Azure Event Hub and uses OAuth to authenticate itself.

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
