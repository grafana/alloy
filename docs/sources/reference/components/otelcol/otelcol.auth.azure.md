---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.auth.azure/
description: Learn about otelcol.auth.azure
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.auth.azure
---

# `otelcol.auth.azure`

`otelcol.auth.azure` exposes a `handler` that other `otelcol` components can use to authenticate requests to Azure services using Microsoft Entra ID credentials.

This component only supports client authentication.

The authorization tokens can be used by HTTP and gRPC based OpenTelemetry exporters.

{{< admonition type="note" >}}
`otelcol.auth.azure` is a wrapper over the upstream OpenTelemetry Collector [`azureauth`][] extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`azureauth`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/azureauthextension
{{< /admonition >}}

You can specify multiple `otelcol.auth.azure` components by giving them different labels.

## Usage

```alloy
otelcol.auth.azure "<LABEL>" {
    use_default = true
}
```

## Arguments

You can use the following arguments with `otelcol.auth.azure`:

| Name          | Type           | Description                                            | Default | Required |
| ------------- | -------------- | ------------------------------------------------------ | ------- | -------- |
| `scopes`      | `list(string)` | The scopes to request when fetching a token.           | `[]`    | no       |
| `use_default` | `bool`         | Authenticate using the Azure default credential chain. | `false` | no       |

You must configure exactly one authentication method.
Either set `use_default` to `true`, or provide one of the [`managed_identity`][managed_identity], [`workload_identity`][workload_identity], or [`service_principal`][service_principal] blocks.

When `use_default` is `true`, {{< param "PRODUCT_NAME" >}} uses the [`DefaultAzureCredential`][DefaultAzureCredential] chain, which looks for credentials from environment variables, workload identity, and managed identity, among other sources.

[DefaultAzureCredential]: https://learn.microsoft.com/azure/developer/go/sdk/authentication/credential-chains#defaultazurecredential-overview

## Blocks

You can use the following blocks with `otelcol.auth.azure`:

{{< docs/alloy-config >}}

| Block                                  | Description                                                                | Required |
| -------------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`debug_metrics`][debug_metrics]       | Configures the metrics that this component generates to monitor its state. | no       |
| [`managed_identity`][managed_identity] | Authenticate using an Azure managed identity.                              | no       |
| [`service_principal`][service_principal] | Authenticate using an Azure service principal.                           | no       |
| [`workload_identity`][workload_identity] | Authenticate using an Azure workload identity.                           | no       |

[debug_metrics]: #debug_metrics
[managed_identity]: #managed_identity
[service_principal]: #service_principal
[workload_identity]: #workload_identity

{{< /docs/alloy-config >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `managed_identity`

The `managed_identity` block authenticates using an [Azure managed identity][managed-identities].

| Name        | Type     | Description                                          | Default | Required |
| ----------- | -------- | ---------------------------------------------------- | ------- | -------- |
| `client_id` | `string` | The client ID of the user-assigned managed identity. | `""`    | no       |

If `client_id` is empty, the system-assigned managed identity is used.

[managed-identities]: https://learn.microsoft.com/entra/identity/managed-identities-azure-resources/overview

### `service_principal`

The `service_principal` block authenticates using an [Azure service principal][service-principals].

| Name                      | Type     | Description                                                    | Default | Required |
| ------------------------- | -------- | ------------------------------------------------------------- | ------- | -------- |
| `client_id`               | `string` | The client ID of the service principal.                       |         | yes      |
| `tenant_id`               | `string` | The tenant ID of the service principal.                       |         | yes      |
| `client_certificate_path` | `string` | The path to the certificate used to authenticate.             | `""`    | no       |
| `client_secret`           | `string` | The client secret used to authenticate.                       | `""`    | no       |

You must set exactly one of `client_secret` or `client_certificate_path`.

[service-principals]: https://learn.microsoft.com/entra/identity-platform/app-objects-and-service-principals

### `workload_identity`

The `workload_identity` block authenticates using an [Azure workload identity][workload-identities].

| Name                   | Type     | Description                                        | Default | Required |
| ---------------------- | -------- | -------------------------------------------------- | ------- | -------- |
| `client_id`            | `string` | The client ID of the application.                  |         | yes      |
| `federated_token_file` | `string` | The path to a file that contains a federated token. |        | yes      |
| `tenant_id`            | `string` | The tenant ID of the application.                  |         | yes      |

[workload-identities]: https://learn.microsoft.com/entra/workload-id/workload-identities-overview

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                     |
| --------- | -------------------------- | --------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests. |

## Component health

`otelcol.auth.azure` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.auth.azure` doesn't expose any component-specific debug information.

## Examples

### Authenticate using a managed identity

This example configures [`otelcol.exporter.otlp`][otelcol.exporter.otlp] to use `otelcol.auth.azure` with the system-assigned managed identity:

```alloy
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.azure.creds.handler
  }
}

otelcol.auth.azure "creds" {
    managed_identity {}
}
```

### Authenticate using a service principal

This example authenticates using a service principal with a client secret:

```alloy
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.azure.creds.handler
  }
}

otelcol.auth.azure "creds" {
    service_principal {
        tenant_id     = sys.env("TENANT_ID") 
        client_id     = sys.env("CLIENT_ID")
        client_secret = sys.env("CLIENT_SECRET")
    }
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/
