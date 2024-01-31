---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.remote_relabel/
description: Learn about discovery.remote_relabel
title: discovery.remote_relabel
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# discovery.remote_relabel

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`discovery.remote_relabel` accepts relabeling rules from a remote control server using Pyroscope's [settings API].
Additionally it is also possible for the control server to request the targets that are received by this component in order to show the effects of the rules at the central control server.

Multiple `discovery.remote_relabel` components can be specified by giving them different labels.

## Usage

```alloy
discovery.relabel "LABEL" {
  targets = TARGET_LIST
  websocket {
    url = "ws://localhost:4040/settings.v1.SettingsService/GetCollectionRules?scope=alloy"
  }
}
```

## Arguments

The following arguments are supported:

| Name      | Type                | Description        | Default | Required |
| --------- | ------------------- | ------------------ | ------- | -------- |
| `targets` | `list(map(string))` | Targets to relabel |         | yes      |

## Blocks

The following blocks are supported inside the definition of `pyroscope.write`:

| Hierarchy              | Block          | Description                                               | Required |
| ---------------------- | -------------- | --------------------------------------------------------- | -------- |
| websocket              | [websocket][]  | Control server settingss to.                              | yes      |
| websocket > basic_auth | [basic_auth][] | Configure basic_auth for authenticating to the websocket. | no       |

[websocket]:#websocket-block
[basic_auth]:#basic_auth-block

### websocket block

The `websocket` block describes a single command and control server. Only one `websocket` block can ben provided.

The following arguments are supported:

| Name                  | Type       | Description                                                        | Default   | Required |
| --------------------- | ---------- | ------------------------------------------------------------------ | --------- | -------- |
| `url`                 | `string`   | Full URL to the websocket server. Needs to start with ws:// wss:// |           | yes      |
| `keep_alive`          | `duration` | How often to sent keep alive pings. 0 to disable.                  | `"295s"`  | no       |
| `min_backoff_period`  | `duration` | Initial backoff time between retries.                              | `"500ms"` | no       |
| `max_backoff_period`  | `duration` | Maximum backoff time between retries.                              | `"5m"`    | no       |
| `max_backoff_retries` | `int`      | Maximum number of retries. 0 to retry infinitely.                  | `10`      | no       |

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name     | Type                | Description                                   |
| -------- | ------------------- | --------------------------------------------- |
| `output` | `list(map(string))` | The set of targets after applying relabeling. |
| `rules`  | `RelabelRules`      | The currently configured relabeling rules.    |

## Component health

`discovery.remote_relabel` is only reported as unhealthy when the websocket is not connected.

## Debug information

| Name                        | Type        | Description                                              |
| --------------------------- | ----------- | -------------------------------------------------------- |
| `websocket_status`          | `string`    | Status of the websocket connection.                      |
| `websocket_connected_since` | `time.Time` | The currently configured relabeling rules.               |
| `websocket_last_error`      | `string`    | What was the last error when reconnecting the websocket. |

## Example

```alloy
discovery.remote_relabel "control_profiles" {
  targets = [
    { "__meta_foo" = "foo", "__address__" = "localhost", "instance" = "one",   "app" = "backend"  },
    { "__meta_bar" = "bar", "__address__" = "localhost", "instance" = "two",   "app" = "database" },
    { "__meta_baz" = "baz", "__address__" = "localhost", "instance" = "three", "app" = "frontend" },
  ]
  websocket {
    url = "wss://profiles-prod-001.grafana.net/settings.v1.SettingsService/GetCollectionRules?scope=alloy"
    basic_auth {
      username =  env("PROFILECLI_USERNAME")
      password =  env("PROFILECLI_PASSWORD")
    }
  }
}
```

[settings API]: https://github.com/grafana/pyroscope/blob/main/api/settings/v1/setting.proto

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.remote_relabel` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)

`discovery.remote_relabel` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
